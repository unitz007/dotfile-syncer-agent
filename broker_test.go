package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrokerNotifier_NextPolicyRunCommand(t *testing.T) {
	// Mock Broker Server returning the new spec-based payload.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agent/policy-run" {
			t.Errorf("Expected path /agent/policy-run, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Agent test-token" {
			t.Errorf("Expected Authorization header 'Agent test-token', got %s", r.Header.Get("Authorization"))
		}

		response := struct {
			HasRun bool            `json:"has_run"`
			RunID  string          `json:"run_id"`
			Spec   *SyncPolicySpec `json:"spec"`
		}{
			HasRun: true,
			RunID:  "run-123",
			Spec: &SyncPolicySpec{
				APIVersion: "dotsync/v1",
				Kind:       "SyncPolicy",
				Metadata:   SpecMetadata{Name: "test"},
				Spec: SyncPolicySpecBody{
					Repository: "https://github.com/test/repo",
					Strategy:   "symlink",
					Files: []SpecFileMapping{
						{Source: ".bashrc", Target: "~/.bashrc"},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	notifier := &BrokerNotifier{
		brokerUrl:  server.URL,
		agentToken: "test-token",
		machine:    "agent-1",
	}

	cmd, err := notifier.NextPolicyRunCommand()
	if err != nil {
		t.Fatalf("NextPolicyRunCommand failed: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected command, got nil")
	}

	if cmd.RunID != "run-123" {
		t.Errorf("Expected RunID 'run-123', got %s", cmd.RunID)
	}

	if cmd.Spec == nil {
		t.Fatal("Expected Spec, got nil")
	}

	if cmd.Spec.Metadata.Name != "test" {
		t.Errorf("Unexpected spec name: %s", cmd.Spec.Metadata.Name)
	}
}

func TestBrokerNotifier_NextPolicyRunCommand_NoRun(t *testing.T) {
	// Mock Broker Server returning has_run=false.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := struct {
			HasRun bool `json:"has_run"`
		}{HasRun: false}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	notifier := &BrokerNotifier{
		brokerUrl:  server.URL,
		agentToken: "test-token",
	}

	cmd, err := notifier.NextPolicyRunCommand()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cmd != nil {
		t.Error("Expected nil command when has_run=false")
	}
}

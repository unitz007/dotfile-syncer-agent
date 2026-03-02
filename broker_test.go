package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrokerNotifier_NextPolicyRunCommand(t *testing.T) {
	// Mock Broker Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agent/policy-run" {
			t.Errorf("Expected path /agent/policy-run, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Agent test-token" {
			t.Errorf("Expected Authorization header 'Agent test-token', got %s", r.Header.Get("Authorization"))
		}

		// Response payload
		response := struct {
			HasRun        bool           `json:"has_run"`
			RunID         string         `json:"run_id"`
			ExecutionPlan *ExecutionPlan `json:"execution_plan"`
		}{
			HasRun: true,
			RunID:  "run-123",
			ExecutionPlan: &ExecutionPlan{
				Install:      []string{"echo hello"},
				DotfilesRepo: "https://github.com/test/repo",
				Files: []FileMapping{
					{Source: ".bashrc", Target: "~/.bashrc"},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// BrokerNotifier
	notifier := &BrokerNotifier{
		brokerUrl:  server.URL,
		agentToken: "test-token",
		machine:    "agent-1",
	}

	// Call
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

	if cmd.ExecutionPlan == nil {
		t.Fatal("Expected ExecutionPlan, got nil")
	}

	if len(cmd.ExecutionPlan.Install) != 1 || cmd.ExecutionPlan.Install[0] != "echo hello" {
		t.Errorf("Unexpected Install commands: %v", cmd.ExecutionPlan.Install)
	}
}

func TestBrokerNotifier_NextPolicyRunCommand_InvalidPlan(t *testing.T) {
	// Mock Broker Server returning invalid plan
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := struct {
			HasRun        bool           `json:"has_run"`
			RunID         string         `json:"run_id"`
			ExecutionPlan *ExecutionPlan `json:"execution_plan"`
		}{
			HasRun: true,
			RunID:  "run-bad",
			ExecutionPlan: &ExecutionPlan{
				Files: []FileMapping{
					{Source: ".bashrc", Target: ""}, // Invalid: empty target
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	notifier := &BrokerNotifier{
		brokerUrl:  server.URL,
		agentToken: "test-token",
	}

	cmd, err := notifier.NextPolicyRunCommand()
	if err == nil {
		t.Error("Expected validation error, got nil")
	}
	if cmd != nil {
		t.Error("Expected nil command on error, got struct")
	}
}

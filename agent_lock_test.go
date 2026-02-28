package main

import (
	"os"
	"path"
	"testing"
)

func TestAcquireAgentLockAllowsSingleInstance(t *testing.T) {
	dir, err := os.MkdirTemp("", "agent-lock-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	config := &Configurations{
		ConfigPath: dir,
	}

	release, err := acquireAgentLock(config)
	if err != nil {
		t.Fatalf("expected first lock acquisition to succeed, got error: %v", err)
	}

	lockFile := path.Join(dir, "agent.lock")
	if _, statErr := os.Stat(lockFile); statErr != nil {
		t.Fatalf("expected lock file to exist, got error: %v", statErr)
	}

	err = release()
	if err != nil {
		t.Fatalf("expected lock release to succeed, got error: %v", err)
	}

	if _, statErr := os.Stat(lockFile); !os.IsNotExist(statErr) {
		t.Fatalf("expected lock file to be removed after release, got: %v", statErr)
	}
}

func TestAcquireAgentLockPreventsSecondInstance(t *testing.T) {
	dir, err := os.MkdirTemp("", "agent-lock-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	config := &Configurations{
		ConfigPath: dir,
	}

	release, err := acquireAgentLock(config)
	if err != nil {
		t.Fatalf("expected first lock acquisition to succeed, got error: %v", err)
	}
	defer func() {
		_ = release()
	}()

	_, err = acquireAgentLock(config)
	if err == nil {
		t.Fatalf("expected second lock acquisition to fail, but it succeeded")
	}
}


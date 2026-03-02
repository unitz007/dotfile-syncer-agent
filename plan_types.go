package main

import "fmt"

// ExecutionPlan represents the set of instructions for the Agent to execute.
// It mirrors the structure returned by the Broker's Policy Engine.
type ExecutionPlan struct {
	// Install contains a sequential list of shell commands to install required software.
	Install []string `json:"install"`

	// DotfilesRepo is the URL of the Git repository to sync.
	DotfilesRepo string `json:"dotfiles_repo"`

	// SyncStrategy defines how dotfiles should be applied (e.g., "symlink", "copy").
	SyncStrategy string `json:"sync_strategy"`

	// WebhookEnabled indicates if the agent should listen for webhook events.
	WebhookEnabled bool `json:"webhook_enabled"`

	// Files maps source files from the repo to target paths on the agent.
	Files []FileMapping `json:"files"`
}

// FileMapping defines a single file synchronization rule.
type FileMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// PolicyRunResponse represents the response from the Broker's policy run endpoint.
type PolicyRunResponse struct {
	HasRun        bool          `json:"has_run"`
	RunID         string        `json:"run_id,omitempty"`
	ExecutionPlan ExecutionPlan `json:"execution_plan,omitempty"`
}

// Validate checks if the execution plan is valid and safe to execute.
func (p *ExecutionPlan) Validate() error {
	if p == nil {
		return nil // Empty plan is technically valid (no-op)
	}

	// Validate DotfilesRepo if files are present
	if len(p.Files) > 0 && p.DotfilesRepo == "" {
		return fmt.Errorf("execution plan contains file mappings but no dotfiles repository URL")
	}

	for _, f := range p.Files {
		if f.Source == "" {
			return fmt.Errorf("file mapping missing source")
		}
		if f.Target == "" {
			return fmt.Errorf("file mapping missing target for source: %s", f.Source)
		}
	}

	return nil
}

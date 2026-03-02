package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type PlanExecutor struct {
	config         *Configurations
	brokerNotifier *BrokerNotifier
	git            *Git
}

func NewPlanExecutor(config *Configurations, brokerNotifier *BrokerNotifier, git *Git) *PlanExecutor {
	return &PlanExecutor{
		config:         config,
		brokerNotifier: brokerNotifier,
		git:            git,
	}
}

// Execute runs the provided execution plan.
// It reports status updates to the broker.
func (e *PlanExecutor) Execute(runID string, plan *ExecutionPlan) error {
	e.reportStatus(runID, "running", "Execution started")

	// 1. Install Packages / Run Commands
	if len(plan.Install) > 0 {
		if err := e.executeInstall(runID, plan.Install); err != nil {
			e.reportStatus(runID, "failed", fmt.Sprintf("Install failed: %v", err))
			return err
		}
	}

	// 2. Sync Files
	if len(plan.Files) > 0 {
		if err := e.executeFileSync(runID, plan); err != nil {
			e.reportStatus(runID, "failed", fmt.Sprintf("File sync failed: %v", err))
			return err
		}
	}

	e.reportStatus(runID, "succeeded", "Execution completed successfully")
	return nil
}

func (e *PlanExecutor) executeInstall(runID string, commands []string) error {
	for _, cmdStr := range commands {
		e.reportStatus(runID, "running", fmt.Sprintf("Executing: %s", cmdStr))
		
		// Simple command execution (sh -c)
		cmd := exec.Command("sh", "-c", cmdStr)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command '%s' failed: %w", cmdStr, err)
		}
	}
	return nil
}

func (e *PlanExecutor) executeFileSync(runID string, plan *ExecutionPlan) error {
	// 1. Ensure Repo is up to date
	repoURL := plan.DotfilesRepo
	if repoURL == "" {
		// Fallback to configured repo if not in plan (though validation checks this)
		repoURL = e.config.GitUrl
	}

	// We assume the repo is cloned at e.config.DotfilePath
	// If the plan specifies a different repo, we might need to clone it elsewhere?
	// For now, let's assume we use the main dotfile path.
	// TODO: Handle different repos or multiple repos? 
	// The current architecture assumes one main dotfile repo.
	
	e.reportStatus(runID, "running", "Syncing repository...")
	// We can reuse e.git to pull/clone.
	// But e.git is initialized with e.config which has the path.
	// If plan.DotfilesRepo is different, we might have a mismatch.
	// We'll update e.config.GitUrl if needed, but the path remains.
	
	if repoURL != "" && repoURL != e.config.GitUrl {
		// Update config for this run? Or just warn?
		// e.config.GitUrl = repoURL // This might be risky if we persist it.
	}

	if err := e.git.CloneOrPullRepository(); err != nil {
		return fmt.Errorf("failed to sync repository: %w", err)
	}

	// 2. Process Files
	for _, mapping := range plan.Files {
		src := filepath.Join(e.config.DotfilePath, mapping.Source)
		target, err := expandPath(mapping.Target)
		if err != nil {
			return fmt.Errorf("invalid target path '%s': %w", mapping.Target, err)
		}

		e.reportStatus(runID, "running", fmt.Sprintf("Syncing %s -> %s", mapping.Source, target))

		if err := installFile(src, target, plan.SyncStrategy); err != nil {
			return fmt.Errorf("failed to sync %s: %w", mapping.Source, err)
		}
	}

	return nil
}

func (e *PlanExecutor) reportStatus(runID, status string, message interface{}) {
	if e.brokerNotifier != nil {
		// We use a struct or map for the result
		result := map[string]interface{}{
			"message": message,
		}
		_ = e.brokerNotifier.ReportPolicyRunResult(runID, status, result)
	}
}

// Helper functions

func expandPath(pathStr string) (string, error) {
	if strings.HasPrefix(pathStr, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, pathStr[2:]), nil
	}
	return pathStr, nil
}

func installFile(src, target, strategy string) error {
	// Ensure target directory exists
	targetDir := filepath.Dir(target)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Check if source exists
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source file not found: %w", err)
	}

	// Remove existing target if it exists
	if _, err := os.Lstat(target); err == nil {
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("failed to remove existing target: %w", err)
		}
	}

	if strategy == "symlink" {
		if err := os.Symlink(src, target); err != nil {
			return fmt.Errorf("symlink failed: %w", err)
		}
	} else {
		// Default to copy
		if err := copyFile(src, target); err != nil {
			return fmt.Errorf("copy failed: %w", err)
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

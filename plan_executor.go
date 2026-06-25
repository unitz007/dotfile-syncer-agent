package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// allowedInstallPattern restricts broker-supplied install commands to the form
// "<package-manager> install <package-name>" with no shell metacharacters.
var allowedInstallPattern = regexp.MustCompile(`^(brew|apt|apt-get|dnf|yum|pacman|apk)\s+install\s+[A-Za-z0-9@._+=/-]+$`)

type PlanExecutor struct {
	config         *Configurations
	brokerNotifier *BrokerNotifier
	git            *Git
	NoPull         bool // when true, skip git pull and apply from local repo state
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
		trimmed := strings.TrimSpace(cmdStr)
		if !allowedInstallPattern.MatchString(trimmed) {
			return fmt.Errorf("install command rejected (not in allowlist): %q", trimmed)
		}
		e.reportStatus(runID, "running", fmt.Sprintf("Executing: %s", trimmed))

		parts := strings.Fields(trimmed)
		cmd := exec.Command(parts[0], parts[1:]...) // #nosec G204 — validated above
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command '%s' failed: %w", trimmed, err)
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
	
	if !e.NoPull {
		e.reportStatus(runID, "running", "Syncing repository...")
		if err := e.git.CloneOrPullRepository(); err != nil {
			return fmt.Errorf("failed to sync repository: %w", err)
		}
	}

	// 2. Process Files
	dotfileBase := filepath.Clean(e.config.DotfilePath)
	for _, mapping := range plan.Files {
		src := filepath.Clean(filepath.Join(dotfileBase, mapping.Source))
		if !strings.HasPrefix(src, dotfileBase+string(filepath.Separator)) && src != dotfileBase {
			return fmt.Errorf("source path escapes dotfile directory: %s", mapping.Source)
		}

		target, err := expandPath(mapping.Target)
		if err != nil {
			return fmt.Errorf("invalid target path '%s': %w", mapping.Target, err)
		}
		// Reject absolute target paths that aren't home-relative (~/...)
		if filepath.IsAbs(mapping.Target) {
			return fmt.Errorf("absolute target paths are not allowed: %s", mapping.Target)
		}

		if _, err := os.Stat(src); os.IsNotExist(err) {
			Warnln("source not found, skipping:", mapping.Source)
			continue
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

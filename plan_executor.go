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

// ApplyResult collects what happened during an Execute call.
type ApplyResult struct {
	PackagesInstalled []string
	FilesApplied      []string
	FilesSkipped      []string
}

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

// Execute runs the provided execution plan and returns a result summary.
func (e *PlanExecutor) Execute(runID string, plan *ExecutionPlan) (*ApplyResult, error) {
	result := &ApplyResult{}
	e.reportStatus(runID, "running", "Execution started")

	// Pull repo once up front when there is file work to do.
	hasFiles := len(plan.SoftwareUnits) > 0 || len(plan.Files) > 0
	if !e.NoPull && hasFiles {
		e.reportStatus(runID, "running", "Syncing repository...")
		if err := e.git.CloneOrPullRepository(); err != nil {
			e.reportStatus(runID, "failed", fmt.Sprintf("repo sync failed: %v", err))
			return result, fmt.Errorf("failed to sync repository: %w", err)
		}
	}

	if len(plan.SoftwareUnits) > 0 {
		// Software-grouped execution: each unit's packages are installed before its configs.
		for _, unit := range plan.SoftwareUnits {
			e.executeSoftwareUnit(runID, unit, plan.SyncStrategy, result)
		}
	} else {
		// Legacy flat execution path.
		if len(plan.Install) > 0 {
			installed, err := e.executeInstall(runID, plan.Install)
			result.PackagesInstalled = append(result.PackagesInstalled, installed...)
			if err != nil {
				e.reportStatus(runID, "failed", fmt.Sprintf("Install failed: %v", err))
				return result, err
			}
		}
		if len(plan.Files) > 0 {
			if err := e.executeFileSyncFlat(runID, plan, result); err != nil {
				e.reportStatus(runID, "failed", fmt.Sprintf("File sync failed: %v", err))
				return result, err
			}
		}
	}

	e.reportStatus(runID, "succeeded", "Execution completed successfully")
	return result, nil
}

// executeSoftwareUnit installs packages for one software unit, then applies its
// config file mappings.  A package install failure causes configs to be skipped
// (warned, not hard-failed) so other units continue.
func (e *PlanExecutor) executeSoftwareUnit(runID string, unit SoftwareUnit, strategy string, result *ApplyResult) {
	Infoln(fmt.Sprintf("Applying software unit: %s", unit.Name))

	if len(unit.Install) > 0 {
		installed, err := e.executeInstall(runID, unit.Install)
		result.PackagesInstalled = append(result.PackagesInstalled, installed...)
		if err != nil {
			Warnln(fmt.Sprintf("[%s] package install failed — skipping configs: %v", unit.Name, err))
			for _, f := range unit.Files {
				result.FilesSkipped = append(result.FilesSkipped, f.Source+" (package install failed)")
			}
			return
		}
	}

	dotfileBase := filepath.Clean(e.config.DotfilePath)
	for _, mapping := range unit.Files {
		src := filepath.Clean(filepath.Join(dotfileBase, mapping.Source))
		if !strings.HasPrefix(src, dotfileBase+string(filepath.Separator)) && src != dotfileBase {
			Warnln(fmt.Sprintf("[%s] source path escapes dotfile dir, skipping: %s", unit.Name, mapping.Source))
			result.FilesSkipped = append(result.FilesSkipped, mapping.Source+" (path traversal rejected)")
			continue
		}
		target, err := expandPath(mapping.Target)
		if err != nil || filepath.IsAbs(mapping.Target) {
			Warnln(fmt.Sprintf("[%s] invalid target, skipping: %s", unit.Name, mapping.Target))
			result.FilesSkipped = append(result.FilesSkipped, mapping.Source+" (invalid target)")
			continue
		}
		if _, err := os.Stat(src); os.IsNotExist(err) {
			Warnln(fmt.Sprintf("[%s] source not found, skipping: %s", unit.Name, mapping.Source))
			result.FilesSkipped = append(result.FilesSkipped, mapping.Source)
			continue
		}
		if err := installFile(src, target, strategy); err != nil {
			Warnln(fmt.Sprintf("[%s] failed to sync %s: %v", unit.Name, mapping.Source, err))
			result.FilesSkipped = append(result.FilesSkipped, mapping.Source+" (sync error)")
			continue
		}
		result.FilesApplied = append(result.FilesApplied, fmt.Sprintf("%s → %s", mapping.Source, target))
	}
}

func (e *PlanExecutor) executeInstall(runID string, commands []string) ([]string, error) {
	var installed []string
	for _, cmdStr := range commands {
		trimmed := strings.TrimSpace(cmdStr)
		if !allowedInstallPattern.MatchString(trimmed) {
			return installed, fmt.Errorf("install command rejected (not in allowlist): %q", trimmed)
		}

		// parts: [manager, "install", pkg]
		parts := strings.Fields(trimmed)
		manager, pkg := parts[0], parts[2]

		if isPackageInstalled(manager, pkg) {
			Infoln(fmt.Sprintf("already installed, skipping: %s", pkg))
			continue
		}

		e.reportStatus(runID, "running", fmt.Sprintf("Executing: %s", trimmed))

		cmd := exec.Command(parts[0], parts[1:]...) // #nosec G204 — validated above
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return installed, fmt.Errorf("command '%s' failed: %w", trimmed, err)
		}
		installed = append(installed, trimmed)
	}
	return installed, nil
}

// isPackageInstalled queries the package manager to check whether a package is
// already present on the system.  Returns true if the package is installed so
// the caller can skip the install step.
func isPackageInstalled(manager, pkg string) bool {
	var cmd *exec.Cmd
	switch manager {
	case "brew":
		cmd = exec.Command("brew", "list", pkg)
	case "apt", "apt-get":
		cmd = exec.Command("dpkg", "-s", pkg)
	case "dnf", "yum":
		cmd = exec.Command("rpm", "-q", pkg)
	case "pacman":
		cmd = exec.Command("pacman", "-Q", pkg)
	case "apk":
		cmd = exec.Command("apk", "info", "-e", pkg)
	default:
		return false
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// executeFileSyncFlat handles file syncing for legacy flat execution plans.
// The repo pull is expected to have already happened in Execute().
func (e *PlanExecutor) executeFileSyncFlat(runID string, plan *ExecutionPlan, result *ApplyResult) error {
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
		if filepath.IsAbs(mapping.Target) {
			return fmt.Errorf("absolute target paths are not allowed: %s", mapping.Target)
		}

		if _, err := os.Stat(src); os.IsNotExist(err) {
			Warnln("source not found, skipping:", mapping.Source)
			result.FilesSkipped = append(result.FilesSkipped, mapping.Source)
			continue
		}

		e.reportStatus(runID, "running", fmt.Sprintf("Syncing %s -> %s", mapping.Source, target))

		if err := installFile(src, target, plan.SyncStrategy); err != nil {
			return fmt.Errorf("failed to sync %s: %w", mapping.Source, err)
		}
		result.FilesApplied = append(result.FilesApplied, fmt.Sprintf("%s → %s", mapping.Source, target))
	}

	return nil
}

func (e *PlanExecutor) reportStatus(runID, status string, message interface{}) {
	if e.brokerNotifier != nil {
		result := map[string]interface{}{"message": message}
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
	targetDir := filepath.Dir(target)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source file not found: %w", err)
	}

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

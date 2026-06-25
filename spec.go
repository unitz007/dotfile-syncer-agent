package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SyncPolicySpec is the versioned, universal spec format that carries operator
// intent from the broker (or from .dotsync.yaml in the dotfiles repo) to the
// agent.  The agent is the only component that validates and applies it.
type SyncPolicySpec struct {
	APIVersion string             `yaml:"apiVersion" json:"apiVersion"`
	Kind       string             `yaml:"kind"       json:"kind"`
	Metadata   SpecMetadata       `yaml:"metadata"   json:"metadata"`
	Spec       SyncPolicySpecBody `yaml:"spec"       json:"spec"`
}

type SpecMetadata struct {
	Name string `yaml:"name" json:"name"`
}

type SyncPolicySpecBody struct {
	Repository string                   `yaml:"repository" json:"repository"`
	Branch     string                   `yaml:"branch"     json:"branch"`
	Strategy   string                   `yaml:"strategy"   json:"strategy"`
	Mode       string                   `yaml:"mode"       json:"mode"`
	Files      []SpecFileMapping        `yaml:"files"      json:"files"`
	Packages   map[string][]SpecPackage `yaml:"packages"   json:"packages"`
	Hooks      SpecHooks                `yaml:"hooks"      json:"hooks"`
}

type SpecFileMapping struct {
	Source string `yaml:"source" json:"source"`
	Target string `yaml:"target" json:"target"`
}

type SpecPackage struct {
	Manager string `yaml:"manager" json:"manager"`
	Name    string `yaml:"name"    json:"name"`
}

type SpecHooks struct {
	PostSync []string `yaml:"post_sync" json:"post_sync"`
}

var allowedManagers = map[string]bool{
	"brew":    true,
	"apt":     true,
	"apt-get": true,
	"dnf":     true,
	"yum":     true,
	"pacman":  true,
	"apk":     true,
}

// Validate performs strict schema validation on the spec.
// Any violation causes a descriptive error; the caller must apply nothing on error.
func (s *SyncPolicySpec) Validate() error {
	if s.APIVersion != "dotsync/v1" {
		return fmt.Errorf("invalid apiVersion %q: must be \"dotsync/v1\"", s.APIVersion)
	}
	if s.Kind != "SyncPolicy" {
		return fmt.Errorf("invalid kind %q: must be \"SyncPolicy\"", s.Kind)
	}
	if strings.TrimSpace(s.Metadata.Name) == "" {
		return fmt.Errorf("metadata.name must be non-empty")
	}
	if s.Spec.Strategy != "" && s.Spec.Strategy != "symlink" && s.Spec.Strategy != "copy" {
		return fmt.Errorf("invalid spec.strategy %q: must be \"symlink\" or \"copy\"", s.Spec.Strategy)
	}
	if s.Spec.Mode != "" && s.Spec.Mode != "dry_run" && s.Spec.Mode != "enforce" {
		return fmt.Errorf("invalid spec.mode %q: must be \"dry_run\" or \"enforce\"", s.Spec.Mode)
	}
	for i, f := range s.Spec.Files {
		if strings.TrimSpace(f.Source) == "" {
			return fmt.Errorf("spec.files[%d]: source must be non-empty", i)
		}
		if strings.Contains(f.Source, "..") {
			return fmt.Errorf("spec.files[%d]: source %q must not contain \"..\"", i, f.Source)
		}
		if strings.TrimSpace(f.Target) == "" {
			return fmt.Errorf("spec.files[%d]: target must be non-empty", i)
		}
		// ~/... paths are home-relative, not absolute — they are allowed.
		if filepath.IsAbs(f.Target) {
			return fmt.Errorf("spec.files[%d]: target %q must not be an absolute path (use ~/... instead)", i, f.Target)
		}
	}
	for osKey, pkgs := range s.Spec.Packages {
		for j, pkg := range pkgs {
			if !allowedManagers[pkg.Manager] {
				return fmt.Errorf("spec.packages[%s][%d]: manager %q is not in the allowlist", osKey, j, pkg.Manager)
			}
			if strings.TrimSpace(pkg.Name) == "" {
				return fmt.Errorf("spec.packages[%s][%d]: package name must be non-empty", osKey, j)
			}
		}
	}
	return nil
}

// LoadSpecFromRepo reads .dotsync.yaml from the root of the given repo directory.
// Returns (nil, nil) if the file does not exist.
// Returns (nil, err) if the file exists but cannot be parsed.
func LoadSpecFromRepo(repoPath string) (*SyncPolicySpec, error) {
	specPath := filepath.Join(repoPath, ".dotsync.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading .dotsync.yaml: %w", err)
	}

	var spec SyncPolicySpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parsing .dotsync.yaml: %w", err)
	}
	return &spec, nil
}

// ToExecutionPlan converts the spec into an ExecutionPlan for the given platform
// (e.g. "darwin", "linux").  Package resolution is performed on the agent side.
func (s *SyncPolicySpec) ToExecutionPlan(platform string) *ExecutionPlan {
	strategy := s.Spec.Strategy
	if strategy == "" {
		strategy = "symlink"
	}

	plan := &ExecutionPlan{
		DotfilesRepo: s.Spec.Repository,
		SyncStrategy: strategy,
	}

	for _, f := range s.Spec.Files {
		plan.Files = append(plan.Files, FileMapping{
			Source: f.Source,
			Target: f.Target,
		})
	}

	for _, pkg := range s.Spec.Packages[platform] {
		plan.Install = append(plan.Install, fmt.Sprintf("%s install %s", pkg.Manager, pkg.Name))
	}

	return plan
}

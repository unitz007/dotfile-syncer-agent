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
	Repository string         `yaml:"repository" json:"repository"`
	Branch     string         `yaml:"branch"     json:"branch"`
	Strategy   string         `yaml:"strategy"   json:"strategy"`
	Mode       string         `yaml:"mode"       json:"mode"`
	Software   []SpecSoftware `yaml:"software"   json:"software"`
	Hooks      SpecHooks      `yaml:"hooks"      json:"hooks"`
}

// SpecSoftware groups a named software unit's packages and config file mappings.
// The executor installs packages first; on failure the unit's configs are skipped.
type SpecSoftware struct {
	Name     string                   `yaml:"name"     json:"name"`
	Packages map[string][]SpecPackage `yaml:"packages" json:"packages"`
	Configs  []SpecFileMapping        `yaml:"configs"  json:"configs"`
}

type SpecFileMapping struct {
	Source string `yaml:"source" json:"source"`
	Target string `yaml:"target" json:"target"`
}

type SpecPackage struct {
	Manager string `yaml:"manager" json:"manager"`
	Name    string `yaml:"name"    json:"name"`
}

type SpecHook struct {
	Source  string `yaml:"source"  json:"source"`  // path to a script in the repo
	Command string `yaml:"command" json:"command"` // inline shell command
}

type SpecHooks struct {
	PostSync []SpecHook `yaml:"post_sync" json:"post_sync"`
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
	for i, sw := range s.Spec.Software {
		if strings.TrimSpace(sw.Name) == "" {
			return fmt.Errorf("spec.software[%d]: name must be non-empty", i)
		}
		if len(sw.Packages) == 0 && len(sw.Configs) == 0 {
			return fmt.Errorf("spec.software[%d] (%q): must have at least one of packages or configs", i, sw.Name)
		}
		for osKey, pkgs := range sw.Packages {
			for j, pkg := range pkgs {
				if !allowedManagers[pkg.Manager] {
					return fmt.Errorf("spec.software[%d].packages[%s][%d]: manager %q is not in the allowlist", i, osKey, j, pkg.Manager)
				}
				if strings.TrimSpace(pkg.Name) == "" {
					return fmt.Errorf("spec.software[%d].packages[%s][%d]: package name must be non-empty", i, osKey, j)
				}
				if !validPackageName(pkg.Name) {
					return fmt.Errorf("spec.software[%d].packages[%s][%d]: package name %q is unsafe", i, osKey, j, pkg.Name)
				}
			}
		}
		for j, f := range sw.Configs {
			if strings.TrimSpace(f.Source) == "" {
				return fmt.Errorf("spec.software[%d].configs[%d]: source must be non-empty", i, j)
			}
			if filepath.IsAbs(f.Source) {
				return fmt.Errorf("spec.software[%d].configs[%d]: source %q must be relative", i, j, f.Source)
			}
			if strings.Contains(f.Source, "..") {
				return fmt.Errorf("spec.software[%d].configs[%d]: source %q must not contain \"..\"", i, j, f.Source)
			}
			if strings.TrimSpace(f.Target) == "" {
				return fmt.Errorf("spec.software[%d].configs[%d]: target must be non-empty", i, j)
			}
			if !strings.HasPrefix(f.Target, "~/") {
				return fmt.Errorf("spec.software[%d].configs[%d]: target %q must start with \"~/\"", i, j, f.Target)
			}
			if pathHasTraversal(f.Target[2:]) {
				return fmt.Errorf("spec.software[%d].configs[%d]: target %q must not contain \"..\"", i, j, f.Target)
			}
			// ~/... paths are home-relative, not absolute -- they are allowed.
			if filepath.IsAbs(f.Target) {
				return fmt.Errorf("spec.software[%d].configs[%d]: target %q must not be an absolute path (use ~/... instead)", i, j, f.Target)
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

// ParseSpec parses a raw YAML or JSON byte slice into a SyncPolicySpec.
func ParseSpec(data []byte) (*SyncPolicySpec, error) {
	var spec SyncPolicySpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return &spec, nil
}

// MergeSpecs merges two specs with the following rules:
//   - Scalar fields (repository, branch, strategy, mode): override wins when set.
//   - Files: union of both lists; when the same target appears in both, override wins.
//   - Packages: union of both lists per OS; (manager, name) duplicates are deduplicated.
//
// base is typically the repo .dotsync.yaml; override is the broker spec.
func MergeSpecs(base, override *SyncPolicySpec) *SyncPolicySpec {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	merged := &SyncPolicySpec{
		APIVersion: "dotsync/v1",
		Kind:       "SyncPolicy",
		Metadata:   SpecMetadata{Name: base.Metadata.Name},
	}

	// Scalars: override wins when non-empty.
	merged.Spec.Repository = base.Spec.Repository
	if override.Spec.Repository != "" {
		merged.Spec.Repository = override.Spec.Repository
	}
	merged.Spec.Branch = base.Spec.Branch
	if override.Spec.Branch != "" {
		merged.Spec.Branch = override.Spec.Branch
	}
	merged.Spec.Strategy = base.Spec.Strategy
	if override.Spec.Strategy != "" {
		merged.Spec.Strategy = override.Spec.Strategy
	}
	merged.Spec.Mode = base.Spec.Mode
	if override.Spec.Mode != "" {
		merged.Spec.Mode = override.Spec.Mode
	}

	// Software units: merge by name.
	// Units only in base or only in override are included as-is.
	// Units in both: union packages per OS (dedup by manager/name), union configs (dedup by target, override wins).
	unitsByName := make(map[string]*SpecSoftware)
	var order []string

	for i := range base.Spec.Software {
		sw := base.Spec.Software[i]
		unitsByName[sw.Name] = &SpecSoftware{
			Name:     sw.Name,
			Packages: clonePackages(sw.Packages),
			Configs:  append([]SpecFileMapping(nil), sw.Configs...),
		}
		order = append(order, sw.Name)
	}

	for _, sw := range override.Spec.Software {
		if existing, ok := unitsByName[sw.Name]; ok {
			// Merge packages per OS: union, dedup by manager/name.
			for osKey, pkgs := range sw.Packages {
				seen := make(map[string]bool)
				for _, p := range existing.Packages[osKey] {
					seen[p.Manager+"/"+p.Name] = true
				}
				for _, p := range pkgs {
					key := p.Manager + "/" + p.Name
					if !seen[key] {
						seen[key] = true
						existing.Packages[osKey] = append(existing.Packages[osKey], p)
					}
				}
			}
			// Merge configs: override wins on same target.
			configsByTarget := make(map[string]SpecFileMapping)
			for _, f := range existing.Configs {
				configsByTarget[f.Target] = f
			}
			for _, f := range sw.Configs {
				configsByTarget[f.Target] = f
			}
			existing.Configs = make([]SpecFileMapping, 0, len(configsByTarget))
			for _, f := range configsByTarget {
				existing.Configs = append(existing.Configs, f)
			}
		} else {
			unitsByName[sw.Name] = &SpecSoftware{
				Name:     sw.Name,
				Packages: clonePackages(sw.Packages),
				Configs:  append([]SpecFileMapping(nil), sw.Configs...),
			}
			order = append(order, sw.Name)
		}
	}

	for _, name := range order {
		merged.Spec.Software = append(merged.Spec.Software, *unitsByName[name])
	}

	return merged
}

// clonePackages deep-copies a packages map to avoid aliasing between merged specs.
func clonePackages(src map[string][]SpecPackage) map[string][]SpecPackage {
	if src == nil {
		return nil
	}
	dst := make(map[string][]SpecPackage, len(src))
	for osKey, pkgs := range src {
		dst[osKey] = append([]SpecPackage(nil), pkgs...)
	}
	return dst
}

// ToExecutionPlan converts the spec into an ExecutionPlan for the given platform
// (e.g. "darwin", "linux").  Package resolution is performed on the agent side.
// When the spec uses the software[] format, SoftwareUnits is populated and the
// legacy Install/Files fields are left empty.
func (s *SyncPolicySpec) ToExecutionPlan(platform string) *ExecutionPlan {
	strategy := s.Spec.Strategy
	if strategy == "" {
		strategy = "symlink"
	}

	plan := &ExecutionPlan{
		DotfilesRepo: s.Spec.Repository,
		SyncStrategy: strategy,
	}

	for _, sw := range s.Spec.Software {
		unit := SoftwareUnit{Name: sw.Name}
		for _, pkg := range sw.Packages[platform] {
			unit.Install = append(unit.Install, fmt.Sprintf("%s install %s", pkg.Manager, pkg.Name))
		}
		for _, cfg := range sw.Configs {
			unit.Files = append(unit.Files, FileMapping{
				Source: cfg.Source,
				Target: cfg.Target,
			})
		}
		plan.SoftwareUnits = append(plan.SoftwareUnits, unit)
	}

	return plan
}

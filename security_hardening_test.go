package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncPolicySpecValidateRejectsUnsafeTargets(t *testing.T) {
	targets := []string{
		"foo",
		"/Users/test/.zshrc",
		"~/../../etc/cron.d/evil",
	}

	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			spec := validTestSpec()
			spec.Spec.Software[0].Configs[0].Target = target

			if err := spec.Validate(); err == nil {
				t.Fatalf("expected target %q to be rejected", target)
			}
		})
	}
}

func TestSyncPolicySpecValidateRejectsUnsafePackageNames(t *testing.T) {
	packages := []string{
		"./payload.deb",
		"../payload.rpm",
		"-evil",
		"nested/package",
	}

	for _, pkg := range packages {
		t.Run(pkg, func(t *testing.T) {
			spec := validTestSpec()
			spec.Spec.Software[0].Packages = map[string][]SpecPackage{
				"linux": {{Manager: "apt", Name: pkg}},
			}

			if err := spec.Validate(); err == nil {
				t.Fatalf("expected package %q to be rejected", pkg)
			}
		})
	}
}

func TestSecureSourcePathRejectsSymlinkEscapingDotfileDir(t *testing.T) {
	dotfileDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "payload")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0600); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}

	if err := os.Symlink(outsideFile, filepath.Join(dotfileDir, "payload")); err != nil {
		t.Fatalf("failed to create source symlink: %v", err)
	}

	if _, err := secureSourcePath(dotfileDir, "payload"); err == nil {
		t.Fatal("expected source symlink escaping the dotfile dir to be rejected")
	}
}

func TestSecureSourcePathRejectsNestedSymlinkEscapingDotfileDir(t *testing.T) {
	dotfileDir := t.TempDir()
	outsideDir := t.TempDir()
	sourceDir := filepath.Join(dotfileDir, "nvim")
	if err := os.Mkdir(sourceDir, 0700); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}
	outsideFile := filepath.Join(outsideDir, "secret")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0600); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(sourceDir, "payload")); err != nil {
		t.Fatalf("failed to create nested symlink: %v", err)
	}

	if _, err := secureSourcePath(dotfileDir, "nvim"); err == nil {
		t.Fatal("expected nested source symlink escaping the dotfile dir to be rejected")
	}
}

func TestSecureSourcePathAllowsNormalRepoFile(t *testing.T) {
	dotfileDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dotfileDir, ".zshrc"), []byte("ok"), 0600); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	src, err := secureSourcePath(dotfileDir, ".zshrc")
	if err != nil {
		t.Fatalf("expected normal source file to be accepted: %v", err)
	}
	if src != filepath.Join(dotfileDir, ".zshrc") {
		t.Fatalf("unexpected source path: %s", src)
	}
}

func validTestSpec() *SyncPolicySpec {
	return &SyncPolicySpec{
		APIVersion: "dotsync/v1",
		Kind:       "SyncPolicy",
		Metadata:   SpecMetadata{Name: "test"},
		Spec: SyncPolicySpecBody{
			Strategy: "symlink",
			Software: []SpecSoftware{
				{
					Name: "shell",
					Packages: map[string][]SpecPackage{
						"darwin": {{Manager: "brew", Name: "neovim"}},
					},
					Configs: []SpecFileMapping{
						{Source: ".zshrc", Target: "~/.zshrc"},
					},
				},
			},
		},
	}
}

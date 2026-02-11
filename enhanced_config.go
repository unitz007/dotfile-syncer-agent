package main

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// EnhancedConfig represents the new structured configuration format
type EnhancedConfig struct {
	Dotfiles []DotfileEntry `yaml:"dotfiles"`
}

// DotfileEntry represents a software and its associated dotfiles
type DotfileEntry struct {
	Software string      `yaml:"software"`
	Install  interface{} `yaml:"install"` // Can be string or map[string]string
	Files    []FileSpec  `yaml:"files"`
}

// GetInstallCommand returns the install command for the current platform
func (d *DotfileEntry) GetInstallCommand() (string, error) {
	switch v := d.Install.(type) {
	case string:
		// Simple string command
		return v, nil
	case map[string]interface{}:
		// Platform-specific commands
		platform := GetPlatform()

		// Check for platform-specific command
		if cmd, ok := v[platform]; ok {
			return fmt.Sprintf("%v", cmd), nil
		}

		// Check for 'all' platform (cross-platform)
		if cmd, ok := v["all"]; ok {
			return fmt.Sprintf("%v", cmd), nil
		}

		return "", fmt.Errorf("no install command for platform: %s", platform)
	default:
		return "", fmt.Errorf("invalid install command format")
	}
}

// FileSpec represents a file or directory to sync
type FileSpec struct {
	Path   string `yaml:"path"`
	Target string `yaml:"target"`
}

// ParseEnhancedConfig reads and parses the enhanced YAML configuration
func ParseEnhancedConfig(configPath string) (*EnhancedConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config EnhancedConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// GetInstallCommands returns a map of software to installation commands
func (c *EnhancedConfig) GetInstallCommands() map[string]string {
	commands := make(map[string]string)
	for _, entry := range c.Dotfiles {
		cmd, err := entry.GetInstallCommand()
		if err == nil && cmd != "" {
			commands[entry.Software] = cmd
		}
	}
	return commands
}

// GetPlatform returns the current operating system platform
func GetPlatform() string {
	return runtime.GOOS // Returns: linux, darwin, windows, freebsd, openbsd, etc.
}

// GetConfigPaths converts the enhanced config to ConfigPathInfo format
func (c *EnhancedConfig) GetConfigPaths(repoDir string) ([]ConfigPathInfo, error) {
	var configPaths []ConfigPathInfo
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	for _, entry := range c.Dotfiles {
		for _, fileSpec := range entry.Files {
			// Replace 'home' with actual home directory
			targetPath := strings.ReplaceAll(fileSpec.Target, "home", homeDir)

			// Construct full destination path
			destPath := path.Join(targetPath, fileSpec.Path)

			// Check if it's a directory (ends with ;)
			isDir := strings.HasSuffix(fileSpec.Path, ";")
			cleanPath := strings.TrimSuffix(fileSpec.Path, ";")

			// Source file in repository
			srcPath := path.Join(repoDir, cleanPath)
			srcFile, err := os.Open(srcPath)
			if err != nil {
				// Skip if file doesn't exist in repo
				continue
			}

			configPaths = append(configPaths, ConfigPathInfo{
				Src:   srcFile,
				Dest:  strings.TrimSuffix(destPath, ";"),
				IsDir: isDir,
			})
		}
	}

	return configPaths, nil
}

// GetSoftwareList returns a list of all software defined in the config
func (c *EnhancedConfig) GetSoftwareList() []string {
	software := make([]string, 0, len(c.Dotfiles))
	for _, entry := range c.Dotfiles {
		software = append(software, entry.Software)
	}
	return software
}

// GetFilesBySoftware returns all files for a specific software
func (c *EnhancedConfig) GetFilesBySoftware(software string) []FileSpec {
	for _, entry := range c.Dotfiles {
		if entry.Software == software {
			return entry.Files
		}
	}
	return nil
}

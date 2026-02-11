package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// InstallSoftware reads the config and installs required software
// InstallSoftware reads the config and installs required software
func InstallSoftware(configPath string, interactive bool) error {
	config, err := ParseEnhancedConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	platform := GetPlatform()
	fmt.Printf("Detected platform: %s\n\n", platform)

	installCommands := config.GetInstallCommands()
	if len(installCommands) == 0 {
		fmt.Printf("No software installation commands found for platform: %s\n", platform)
		return nil
	}

	fmt.Printf("Found %d software packages to install:\n\n", len(installCommands))

	for software, command := range installCommands {
		fmt.Printf("  - %s: %s\n", software, command)
	}
	fmt.Println()

	if interactive {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Do you want to install all packages? (y/n): ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Installation cancelled")
			return nil
		}
	}

	fmt.Println("\nStarting installation...\n")

	successCount := 0
	failedPackages := []string{}

	for software, command := range installCommands {
		fmt.Printf("Installing %s...\n", software)

		// Execute installation command
		cmd := exec.Command("bash", "-c", command)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("  ✗ Failed to install %s: %v\n\n", software, err)
			failedPackages = append(failedPackages, software)
		} else {
			fmt.Printf("  ✓ Successfully installed %s\n\n", software)
			successCount++
		}
	}

	// Summary
	fmt.Println("========================================")
	fmt.Printf("Installation complete: %d succeeded, %d failed\n", successCount, len(failedPackages))

	if len(failedPackages) > 0 {
		fmt.Println("\nFailed packages:")
		for _, pkg := range failedPackages {
			fmt.Printf("  - %s\n", pkg)
		}
	}

	return nil
}

// InstallSpecificSoftware installs only specific software from the config
func InstallSpecificSoftware(configPath string, softwareNames []string) error {
	config, err := ParseEnhancedConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	installCommands := config.GetInstallCommands()

	for _, software := range softwareNames {
		command, exists := installCommands[software]
		if !exists {
			fmt.Printf("Warning: %s not found in config\n", software)
			continue
		}

		fmt.Printf("Installing %s...\n", software)

		cmd := exec.Command("bash", "-c", command)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("  ✗ Failed to install %s: %v\n", software, err)
		} else {
			fmt.Printf("  ✓ Successfully installed %s\n", software)
		}
	}

	return nil
}

// ListSoftware lists all software defined in the config
func ListSoftware(configPath string) error {
	config, err := ParseEnhancedConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	software := config.GetSoftwareList()

	fmt.Printf("Software defined in config (%d total):\n\n", len(software))
	for _, s := range software {
		fmt.Printf("  - %s\n", s)
	}

	return nil
}

// ShowPlatformInfo displays platform-specific installation commands for all software
func ShowPlatformInfo(configPath string) error {
	config, err := ParseEnhancedConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	currentPlatform := GetPlatform()
	platforms := []string{"linux", "darwin", "windows", "all"}

	fmt.Printf("Current platform: %s\n\n", currentPlatform)
	fmt.Println("Platform-specific installation commands:")
	fmt.Println("==========================================\n")

	for _, entry := range config.Dotfiles {
		fmt.Printf("%s:\n", entry.Software)

		switch v := entry.Install.(type) {
		case string:
			fmt.Printf("  all: %s\n", v)
		case map[string]interface{}:
			for _, platform := range platforms {
				if cmd, ok := v[platform]; ok {
					marker := ""
					if platform == currentPlatform || platform == "all" {
						marker = " ← current"
					}
					fmt.Printf("  %s: %s%s\n", platform, cmd, marker)
				}
			}
		}
		fmt.Println()
	}

	return nil
}

package main

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// Configurations holds all configuration settings for the dotfile agent
type Configurations struct {
	DotfilePath     string // Local directory where dotfiles repository is cloned
	WebHook         string // Git webhook URL for receiving push notifications
	Port            string // HTTP port for the agent server
	GithubToken     string // GitHub personal access token for API authentication
	ConfigPath      string // Directory for agent configuration and database files
	GitUrl          string // Full Git repository URL (e.g., https://github.com/user/repo.git)
	GitRepository   string // Repository directory name, joined with DotfilePath to locate the local clone
	RepositoryOwner string // Repository owner/organization extracted from GitUrl
	GitApiBaseUrl   string // Base URL for Git API (default: https://api.github.com)
	// GitHubRepoName is the repository name used for GitHub API calls (e.g. RemoteCommit).
	// Normally identical to GitRepository. Kept separate for standalone mode, where
	// DotfilePath already points directly at the local clone's root and GitRepository
	// must stay empty so `filepath.Join(DotfilePath, GitRepository)` keeps resolving
	// to DotfilePath itself. Falls back to GitRepository when unset.
	GitHubRepoName string
}

// InitializeConfigurations creates and validates the agent configuration.
// It reads from environment variables, command-line flags, and sets up necessary directories.
// Returns an error if required configuration (like GITHUB_TOKEN) is missing.
func InitializeConfigurations(
	dotfilePath string,
	configPath string,
) (*Configurations, error) {

	// GitHub token is optional (recommended for private repos or higher rate limits)
	gitToken, _ := os.LookupEnv("GITHUB_TOKEN")

	// Set default dotfile path if not provided
	if dotfilePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("unable to access home directory: %v", err.Error())
		}

		dotfilePath = path.Join(homeDir, "dotfiles")
	} else if strings.HasPrefix(dotfilePath, "~/") || dotfilePath == "~" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			if dotfilePath == "~" {
				dotfilePath = homeDir
			} else {
				dotfilePath = path.Join(homeDir, dotfilePath[2:])
			}
		}
	}

	// Set up configuration directory
	configPath, err := func() (string, error) {
		if configPath == "" {
			configPath, err := os.UserConfigDir()
			if err != nil {
				return "", err
			}

			// Create dotfile-agent config directory if it doesn't exist
			if _, err = os.Stat(path.Join(configPath, "dotfile-agent")); err != nil && os.IsNotExist(err) {
				err := os.Mkdir(path.Join(configPath, "dotfile-agent"), 0700)
				if err != nil {
					return "", err
				}
			} else {
				return path.Join(configPath, "dotfile-agent"), nil

			}
		} else {
			// Validate provided config path exists
			_, err := os.Stat(configPath)
			if err != nil {
				return "", err
			}
		}
		return configPath, nil
	}()

	if err != nil {
		return nil, err
	}

	// Log all configuration values for debugging
	// ################## CONFIGURATIONS ##################
	Infoln("Configuration Path ->", configPath)
	Infoln("Dotfile Path ->", dotfilePath)
	Infoln("Home Path ->", func() string {
		h, _ := os.UserHomeDir()
		return h
	}())
	// #################################################

	config := &Configurations{
		DotfilePath: dotfilePath,
		GithubToken: gitToken,
		ConfigPath:  configPath,
	}

	return config, nil

}

func ParseGitUrl(gitUrl string) (owner, repo string, err error) {
	// Handles https://github.com/owner/repo(.git) and SCP-style SSH remotes
	// like git@github.com:owner/repo(.git) — the latter is what `git clone`
	// produces by default for SSH remotes and what `git remote get-url origin`
	// returns verbatim, so it has to parse cleanly here too.
	gitUrl = strings.TrimSuffix(gitUrl, "/")
	gitUrl = strings.TrimSuffix(gitUrl, ".git")

	if !strings.Contains(gitUrl, "://") {
		if colonIdx := strings.LastIndex(gitUrl, ":"); colonIdx != -1 {
			gitUrl = gitUrl[colonIdx+1:]
		}
	}

	parts := strings.Split(gitUrl, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid git url: %s", gitUrl)
	}

	repo = parts[len(parts)-1]
	owner = parts[len(parts)-2]

	return owner, repo, nil
}

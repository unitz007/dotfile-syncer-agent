package main

import (
	"errors"
	"fmt"
	"net/url"
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
	GitRepository   string // Repository name extracted from GitUrl
	RepositoryOwner string // Repository owner/organization extracted from GitUrl
	GitApiBaseUrl   string // Base URL for Git API (default: https://api.github.com)
}

// InitializeConfigurations creates and validates the agent configuration.
// It reads from environment variables, command-line flags, and sets up necessary directories.
// Returns an error if required configuration (like GITHUB_TOKEN) is missing.
func InitializeConfigurations(
	dotfilePath string,
	webHook string,
	port string,
	configPath string,
	gitUrl string,
	githubApiBaseUrl string) (*Configurations, error) {

	// GitHub token is required for API access
	gitToken, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok {
		return nil, errors.New("no GITHUB_TOKEN environment variable found")
	}

	// Set default dotfile path if not provided
	if dotfilePath == "" {
		homeDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("unable to access home directory: %v", err.Error())
		}

		dotfilePath = path.Join(homeDir, "dotfiles")
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

	// Extract repository name from Git URL
	repoName, err := getRepoValue(gitUrl, "repository")
	if err != nil {
		return nil, err
	}

	// Extract repository owner from Git URL
	repoOwner, err := getRepoValue(gitUrl, "repoOwner")

	if err != nil {
		return nil, err
	}

	// Log all configuration values for debugging
	// ################## CONFIGURATIONS ##################
	Infoln("Configuration Path ->", configPath)
	Infoln("Dotfile Path ->", dotfilePath)
	Infoln("Git Repository ->", repoName)
	Infoln("Repository Owner ->", repoOwner)
	Infoln("Home Path ->", func() string {
		h, _ := os.UserHomeDir()
		return h
	}())
	Infoln("API Base Url ->", githubApiBaseUrl)
	Infoln("WebHook ->", webHook)
	Infoln("Git Url ->", gitUrl)
	Infoln("Port ->", port)
	// #################################################

	config := &Configurations{
		DotfilePath:     dotfilePath,
		WebHook:         webHook,
		Port:            port,
		GithubToken:     gitToken,
		ConfigPath:      configPath,
		GitUrl:          gitUrl,
		GitRepository:   repoName,
		RepositoryOwner: repoOwner,
		GitApiBaseUrl:   githubApiBaseUrl,
	}

	return config, nil

}

// getRepoValue extracts repository information from a Git URL.
// filter can be "repository" (returns repo name) or "repoOwner" (returns owner/org name).
// Expects URLs in format: https://github.com/owner/repository.git
func getRepoValue(gitUrl string, filter string) (string, error) {
	parsedURL, err := url.Parse(gitUrl)
	if err != nil {
		return "", errors.New("unable to parse git url")
	}

	if !strings.HasSuffix(parsedURL.Path, ".git") {
		return "", errors.New("not a git url")
	}

	// Get the path component of the URL
	p := strings.Trim(parsedURL.Path, "/") // Remove leading/trailing slashes

	// Split the path into segments
	segments := strings.Split(p, "/")
	if len(segments) < 2 {
		Error("invalid GitHub URL: %s", gitUrl)
		return "", fmt.Errorf("invalid GitHub URL: %s", gitUrl)
	}

	repoVal, err := func() (string, error) {
		switch filter {
		case "repository":
			return segments[len(segments)-1], nil
		case "repoOwner":
			return segments[len(segments)-2], nil
		default:
			return "", fmt.Errorf("invalid filter: %s", filter)
		}
	}()

	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(repoVal, ".git"), nil
}

package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
)

type Configurations struct {
	DotfilePath     string
	WebHook         string
	Port            string
	GithubToken     string
	ConfigPath      string
	GitUrl          string
	GitRepository   string
	RepositoryOwner string
	GitApiBaseUrl   string
}

func InitializeConfigurations(
	dotfilePath string,
	webHook string,
	port string,
	configPath string,
	gitUrl string,
	githubApiBaseUrl string) (*Configurations, error) {

	gitToken, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok {
		return nil, errors.New("no GITHUB_TOKEN environment variable found")
	}

	if dotfilePath == "" {
		homeDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("unable to access home directory: %v", err.Error())
		}

		dotfilePath = path.Join(homeDir, "dotfiles")
	}

	configPath, err := func() (string, error) {
		if configPath == "" {
			configPath, err := os.UserConfigDir()
			if err != nil {
				return "", err
			}

			if _, err = os.Stat(path.Join(configPath, "dotfile-agent")); err != nil && os.IsNotExist(err) {
				err := os.Mkdir(path.Join(configPath, "dotfile-agent"), 0700)
				if err != nil {
					return "", err
				}
			} else {
				return path.Join(configPath, "dotfile-agent"), nil

			}
		} else {
			_, err := os.Stat(configPath)
			if err != nil {
				return "", err
			}
		}
		return configPath, nil
	}()

	if gitUrl == "" {
		return nil, errors.New("no git url provided")
	}

	repoName, err := getRepoValue(gitUrl, "repository")
	if err != nil {
		return nil, err
	}

	repoOwner, err := getRepoValue(gitUrl, "repoOwner")

	if githubApiBaseUrl == "" {
		return nil, errors.New("no github api base url provided")
	}

	if err != nil {
		return nil, err
	}

	// ################## CONFIGURATIONS ##################
	Infoln("Configuration Path ->", configPath)
	Infoln("Dotfile Path ->", dotfilePath)
	Infoln("Git Repository ->", repoName)
	Infoln("Repository Owner ->", repoOwner)
	Infoln("Git Api Base Url ->", githubApiBaseUrl)
	Infoln("Home Path ->", func() string {
		h, _ := os.UserHomeDir()
		return h
	}())
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

package main

import (
	"errors"
	"fmt"
	"os"
	"path"
)

type Configurations struct {
	DotfilePath string
	WebHook     string
	Port        string
	GithubToken string
	ConfigPath  string
	GitUrl      string
}

func InitializeConfigurations(
	dotfilePath string,
	webHook string,
	port string,
	configPath string,
	gitUrl string) (*Configurations, error) {

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

	if configPath == "" {
		configPath, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		configPath = configPath + "/dotfile-agent"
	}

	if gitUrl == "" {
		return nil, errors.New("no git url provided")
	}

	// ################## CONFIGURATIONS ##################
	Info("Configuration Path ->", configPath)
	Info("Dotfile Path ->", dotfilePath)
	Info("Home Path ->", func() string {
		h, _ := os.UserHomeDir()
		return h
	}())
	Info("WebHook ->", webHook)
	Info("Git Url ->", gitUrl)
	Info("Port ->", port)
	// #################################################

	config := &Configurations{
		DotfilePath: dotfilePath,
		WebHook:     webHook,
		Port:        port,
		GithubToken: gitToken,
		ConfigPath:  configPath,
		GitUrl:      gitUrl,
	}

	return config, nil

}

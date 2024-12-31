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
	RepoName    string
	RepoOwner   string
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

	configPath = func() string {
		if configPath == "" {
			configPath, err := os.UserConfigDir()
			if err != nil {
				return ""
			}

			if _, err = os.Stat(path.Join(configPath, "dotfile-agent")); err != nil && os.IsNotExist(err) {
				err := os.Mkdir(path.Join(configPath, "dotfile-agent"), 0700)
				if err != nil {
					return ""
				}
			} else {
				return path.Join(configPath, "dotfile-agent")

			}
		} else {
		}
		return configPath

	}()

	if gitUrl == "" {
		return nil, errors.New("no git url provided")
	}

	// ################## CONFIGURATIONS ##################
	Infoln("Configuration Path ->", configPath)
	Infoln("Dotfile Path ->", dotfilePath)
	Infoln("Home Path ->", func() string {
		h, _ := os.UserHomeDir()
		return h
	}())
	Infoln("WebHook ->", webHook)
	Infoln("Git Url ->", gitUrl)
	Infoln("Port ->", port)
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

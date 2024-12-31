package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
)

type customSync struct {
	config *Configurations
}

type ConfigPathInfo struct {
	Src   *os.File
	Dest  string
	IsDir bool
}

func (c customSync) Sync() {

	// step 1: go to tmp directory
	_ = os.Chdir(os.TempDir())

	gitUrl := "https://github.com/unitz007/new-dot-file.git"
	repoName, err := getRepoFromGitLink(gitUrl)
	if err != nil {
		Error("Failed to parse git url")
		return
	}

	cmd := exec.Command("git", "clone", gitUrl)
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		Infoln("Did not clone git repository")
		_ = os.Chdir(repoName)
		cmd = exec.Command("git", "pull", "origin", "main")
		cmd.Stdout = os.Stdout
		err = cmd.Run()
		if err != nil {
			Error("Failed to pull origin")
			return
		}
	} else {
		Infoln("Cloned git repository from ", gitUrl)
	}

	_ = os.Chdir(repoName)

	wd, err := os.Getwd()

	configPaths, err := func() ([]string, error) {

		configFile, err := os.ReadFile(path.Join(wd, "dotfile-config.yaml"))
		if err != nil {
			return nil, err
		}

		yamlToPaths, err := parseYAMLToPaths(string(configFile))
		if err != nil {
			return nil, err
		}

		yamlToPaths = func() []string {
			var p []string
			for _, yamlPath := range yamlToPaths {
				home, err := os.UserHomeDir()
				if err != nil {
					break
				}
				p = append(p, strings.ReplaceAll(yamlPath, "home", home))
			}
			return p
		}()

		return yamlToPaths, nil
	}()

	if err != nil {
		Error(err.Error())
		return
	}

	dirs, err := os.ReadDir(wd)
	for _, info := range dirs {
		f, err := os.Open(path.Join(wd, info.Name()))
		if err != nil {
			fmt.Println(err)
			return
		}

		configPathsInfo := func() []ConfigPathInfo {
			var i []ConfigPathInfo
			for _, c := range configPaths {
				if strings.Contains(c, info.Name()) {
					i = append(i, ConfigPathInfo{
						Src:  f,
						Dest: strings.TrimSuffix(c, ";"),
						IsDir: func() bool {
							return strings.HasSuffix(c, ";")
						}(),
					})
				}
			}
			return i
		}()

		for _, configPathInfo := range configPathsInfo {
			u, _ := path.Split(configPathInfo.Dest)
			_, err := os.Stat(u)
			if err != nil {
				_ = os.Mkdir(u, os.ModePerm)

			}

			s, _ := path.Split(configPathInfo.Dest)

			_, err = exec.Command("cp", "-r", configPathInfo.Src.Name(), s).CombinedOutput()
			if err != nil {
				Error("could not copy", configPathInfo.Src.Name(), "to destination", s)
			}
		}
	}

	//err = os.RemoveAll(wd)
	//if err != nil {
	//	Error(err.Error())
	//} else {
	//	Infoln("Successfully removed dotfile directory from", strings.Replace(wd, "/"+repoName, "", 1))
	//}
}

func (c customSync) Consume(consumers ...Consumer) {

}

func NewCustomSync(config *Configurations) Syncer {
	return customSync{config: config}
}

// Recursively parses YAML and generates file paths.
func parseYAMLToPaths(data string) ([]string, error) {
	var rawData map[string]interface{}
	if err := yaml.Unmarshal([]byte(data), &rawData); err != nil {
		return nil, err
	}

	var paths []string
	generatePaths(rawData, "", &paths)
	return paths, nil
}

// Recursive helper to process YAML nodes and form paths.
func generatePaths(node map[string]interface{}, currentPath string, paths *[]string) {
	for key, value := range node {
		// Update the current path with the map key
		newBasePath := path.Join(currentPath, key)

		switch val := value.(type) {
		case []interface{}:
			// Process list of files or directories
			for _, v := range val {
				switch item := v.(type) {
				case string:
					// Append file paths directly
					*paths = append(*paths, path.Join(newBasePath, item))
				case map[string]interface{}:
					// Recurse into nested maps
					generatePaths(item, newBasePath, paths)
				}
			}
		}
	}
}

func getRepoFromGitLink(gitUrl string) (string, error) {
	parsedURL, err := url.Parse(gitUrl)
	if err != nil {
		return "", err
	}

	// Get the path component of the URL
	p := strings.Trim(parsedURL.Path, "/") // Remove leading/trailing slashes

	// Split the path into segments
	segments := strings.Split(p, "/")
	if len(segments) < 2 {
		Error("invalid GitHub URL: %s", gitUrl)
		return "", fmt.Errorf("invalid GitHub URL: %s", gitUrl)
	}

	// The last segment is the repository name
	repoName := segments[len(segments)-1]

	// Remove ".git" suffix if present
	repoName = strings.TrimSuffix(repoName, ".git")

	return repoName, nil
}

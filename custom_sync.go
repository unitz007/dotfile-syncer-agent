package main

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"
)

type customSync struct {
	config         *Configurations
	mutex          *sync.Mutex
	brokerNotifier *BrokerNotifier
	ch             chan SyncEvent
	git            *Git
}

type ConfigPathInfo struct {
	Src   *os.File
	Dest  string
	IsDir bool
}

func NewCustomerSyncer(
	config *Configurations,
	brokerNotifier *BrokerNotifier,
	mutex *sync.Mutex,
	git *Git) Syncer {

	return customSync{
		config:         config,
		mutex:          mutex,
		brokerNotifier: brokerNotifier,
		git:            git,
	}
}

func (c customSync) Sync(ch chan SyncEvent) {

	c.mutex.Lock()
	time.Sleep(time.Second)

	steps := syncSteps(c.config, c.git)

	constant := 100 / len(steps)
	event := SyncEvent{
		Data: struct {
			Progress  int    `json:"progress"`
			IsSuccess bool   `json:"isSuccess"`
			Step      string `json:"step"`
			Error     string `json:"error"`
			Done      bool   `json:"done"`
		}{Progress: 0, IsSuccess: true, Done: false},
	}

	//notify(&Git{c.config}, c.brokerNotifier)

	ch <- event

	for i, step := range steps {
		event.Data.Step = step.Step
		err := step.Action()
		if err != nil {
			event.Data.IsSuccess = false
			event.Data.Error = err.Error()
			ch <- event
			break
		}

		event.Data.IsSuccess = true
		event.Data.Progress += constant
		if i == len(steps)-1 { // on final step
			event.Data.Done = true
			progress := event.Data.Progress
			if progress != 100 {
				event.Data.Progress += 100 - progress
			}
		}
		ch <- event

	}
	//notify(&Git{c.config}, c.brokerNotifier)
	//c.mutex.Unlock()
	close(ch)
	c.mutex.Unlock()
}

func (c customSync) Consume(ch chan SyncEvent, consumers ...Consumer) {
	consumers = append(consumers, func(event SyncEvent) {
		c.brokerNotifier.SyncTrigger(event)
	})

	for event := range ch {
		for _, consumer := range consumers {
			go consumer(event)
		}
		time.Sleep(1 * time.Second)
	}
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

func syncSteps(config *Configurations, git *Git) []struct {
	Step   string
	Action func() error
} {

	var (
		configPathsInfo []ConfigPathInfo
	)

	return []struct {
		Step   string
		Action func() error
	}{
		{
			Step: "Git Repository checkout",
			Action: func() error {
				//_ = os.Chdir(config.DotfilePath)
				//
				//repoName := config.GitRepository
				//cmd := exec.Command("git", "clone", config.GitUrl)
				//err := cmd.Run()
				//if err != nil {
				//	//Infoln("git repository already exists")
				//	_ = os.Chdir(repoName)
				//	cmd = exec.Command("git", "pull", "origin", "main")
				//	err = cmd.Run()
				//	if err != nil {
				//		Error("Failed to pull origin")
				//	}
				//} else {
				//	Infoln("Cloned git repository from ", config.GitUrl)
				//}
				//
				//_ = os.Chdir(repoName)
				//
				//return nil

				return git.CloneOrPullRepository()
			},
		},
		{
			Step: "Read dotfile configurations",
			Action: func() error {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}

				configPaths, err := func() ([]string, error) {
					configFile, err := os.ReadFile(path.Join(wd, "dotfile-config.yaml"))
					if err != nil {
						return nil, errors.New("skipping sync because dotfile-config.yaml does not exist in this repo")
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
					return err
				}

				dirs, err := os.ReadDir(wd)
				for _, info := range dirs {
					f, err := os.Open(path.Join(wd, info.Name()))
					if err != nil {
						return err
					}

					configPathsInfo = func() []ConfigPathInfo {
						for _, c := range configPaths {
							if strings.Contains(c, info.Name()) {
								configPathsInfo = append(configPathsInfo, ConfigPathInfo{
									Src:  f,
									Dest: strings.TrimSuffix(c, ";"),
									IsDir: func() bool {
										return strings.HasSuffix(c, ";")
									}(),
								})
							}
						}
						return configPathsInfo
					}()
				}

				return nil
			},
		}, {
			Step: "Copy dotfiles to configured locations",
			Action: func() error {
				for _, configPathInfo := range configPathsInfo {
					u, _ := path.Split(configPathInfo.Dest)
					_, err := os.Stat(u)
					if err != nil {
						_ = os.MkdirAll(u, os.ModePerm)
					}

					s, _ := path.Split(configPathInfo.Dest)

					_, err = exec.Command("cp", "-r", configPathInfo.Src.Name(), s).CombinedOutput()
					if err != nil {
						return fmt.Errorf("could not copy %s to destination %s", configPathInfo.Src.Name(), s)
					}
				}

				return nil
			},
		},
	}
}

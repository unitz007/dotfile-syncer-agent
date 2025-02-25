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

func (c customSync) Sync(consumers ...Consumer) {
	c.mutex.Lock()
	ch := make(chan SyncEvent)

	notify(&Git{c.config}, c.brokerNotifier)

	go func() {
		steps := syncSteps(c.git)

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

		close(ch)
	}()

	consumers = append(consumers, func(event SyncEvent) {
		c.brokerNotifier.SyncEvent(event)
	})

	for event := range ch {
		for _, consumer := range consumers {
			consumer(event)
		}
		time.Sleep(1 * time.Second)
	}

	notify(&Git{c.config}, c.brokerNotifier)
	c.mutex.Unlock()
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

func syncSteps(git *Git) []struct {
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
				return git.CloneOrPullRepository()
			},
		},
		{
			Step: "Parse dotfile configurations",
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

func notify(git *Git, brokerNotifier *BrokerNotifier) {
	localCommit, err := git.LocalCommit()
	if err != nil {
		Error(err.Error())
		return
	}

	remoteCommit, err := git.RemoteCommit()
	if err != nil {
		Error(err.Error())
		return
	}

	response := InitGitTransform(localCommit, remoteCommit)
	brokerNotifier.SyncStatus(response)
}

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const directorySuffix = ";" // Suffix used in config to denote directories

// customSync implements the Syncer interface using a custom YAML-based configuration.
// It parses dotfile-config.yaml to determine which files to sync and where.
type customSync struct {
	config         *Configurations // Agent configuration
	mutex          *sync.Mutex     // Mutex to prevent concurrent syncs
	brokerNotifier *BrokerNotifier // Notifier for sending events to broker
	ch             chan SyncEvent  // Channel for sync events
	git            *Git            // Git instance for repository operations
}

// ConfigPathInfo contains information about a file or directory to be synced
type ConfigPathInfo struct {
	Src   *os.File // Source file handle in the repository
	Dest  string   // Destination path on the system
	IsDir bool     // Whether this is a directory (ends with ;)
}

// NewCustomerSyncer creates a new custom syncer instance
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

// Sync performs the dotfile synchronization process.
// It executes a series of steps: git checkout, config parsing, and file copying.
// Progress is reported to all registered consumers via SyncEvent messages.
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

	// Add broker notifier as a consumer
	consumers = append(consumers, func(event SyncEvent) {
		c.brokerNotifier.SyncEvent(event)
	})

	// Send events to all consumers
	for event := range ch {
		for _, consumer := range consumers {
			consumer(event)
		}
		time.Sleep(1 * time.Second)
	}

	notify(&Git{c.config}, c.brokerNotifier)
	c.mutex.Unlock()
}

// parseYAMLToPaths recursively parses the YAML configuration and generates file paths.
// It converts the nested YAML structure into flat paths like "home/.bashrc" or "home/.config/nvim;".
func parseYAMLToPaths(data string) ([]string, error) {
	var rawData map[string]interface{}
	if err := yaml.Unmarshal([]byte(data), &rawData); err != nil {
		return nil, err
	}

	var paths []string
	generatePaths(rawData, "", &paths)

	return paths, nil
}

// generatePaths is a recursive helper that processes YAML nodes and forms paths.
// It handles both files (strings) and nested directories (maps).
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

// syncSteps defines the sequence of operations for synchronization.
// Each step has a description and an action function that performs the work.
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

					// Replace 'home' with actual home directory path
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

				// Match config paths with actual files in repository
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
									Dest: strings.TrimSuffix(c, directorySuffix),
									IsDir: func() bool {
										return strings.HasSuffix(c, directorySuffix)
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
					// Create parent directory if it doesn't exist
					u, _ := path.Split(configPathInfo.Dest)
					_, err := os.Stat(u)
					if err != nil {
						_ = os.MkdirAll(u, os.ModePerm)
					}

					s, _ := path.Split(configPathInfo.Dest)

					// Copy file or directory to destination
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

// notify sends the current sync status to the broker.
// It compares local and remote commits and sends the result.
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

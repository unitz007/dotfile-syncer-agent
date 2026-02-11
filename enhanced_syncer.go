package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"
)

type enhancedSync struct {
	config         *Configurations
	mutex          *sync.Mutex
	brokerNotifier *BrokerNotifier
	git            *Git
}

func NewEnhancedSyncer(
	config *Configurations,
	brokerNotifier *BrokerNotifier,
	mutex *sync.Mutex,
	git *Git) Syncer {

	return enhancedSync{
		config:         config,
		mutex:          mutex,
		brokerNotifier: brokerNotifier,
		git:            git,
	}
}

func (e enhancedSync) Sync(consumers ...Consumer) {
	e.mutex.Lock()
	ch := make(chan SyncEvent)

	notify(&Git{e.config}, e.brokerNotifier)

	go func() {
		steps := enhancedSyncSteps(e.git)

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
			if i == len(steps)-1 {
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
		e.brokerNotifier.SyncEvent(event)
	})

	for event := range ch {
		for _, consumer := range consumers {
			consumer(event)
		}
	}

	notify(&Git{e.config}, e.brokerNotifier)
	e.mutex.Unlock()
}

func enhancedSyncSteps(git *Git) []struct {
	Step   string
	Action func() error
} {
	var (
		configPathsInfo []ConfigPathInfo
		_               *EnhancedConfig
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

				configPath := path.Join(wd, "dotfile-config.yaml")

				// Try to parse as enhanced config first
				config, err := ParseEnhancedConfig(configPath)
				if err != nil {
					return errors.New("failed to parse dotfile-config.yaml: " + err.Error())
				}

				_ = config

				// Convert to ConfigPathInfo
				configPathsInfo, err = config.GetConfigPaths(wd)
				if err != nil {
					return err
				}

				if len(configPathsInfo) == 0 {
					return errors.New("no dotfiles found to sync")
				}

				return nil
			},
		},
		{
			Step: "Copy dotfiles to configured locations",
			Action: func() error {
				for _, configPathInfo := range configPathsInfo {
					// Create parent directory if it doesn't exist
					parentDir, _ := path.Split(configPathInfo.Dest)
					if _, err := os.Stat(parentDir); err != nil {
						if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
							return fmt.Errorf("failed to create directory %s: %w", parentDir, err)
						}
					}

					// Copy file or directory
					_, err := exec.Command("cp", "-r", configPathInfo.Src.Name(), configPathInfo.Dest).CombinedOutput()
					if err != nil {
						return fmt.Errorf("could not copy %s to %s: %w", configPathInfo.Src.Name(), configPathInfo.Dest, err)
					}

					Infoln(fmt.Sprintf("Synced: %s -> %s", configPathInfo.Src.Name(), configPathInfo.Dest))
				}

				return nil
			},
		},
	}
}

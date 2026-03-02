package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"
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

	// Notify start of sync (IsSync=true)
	go func() {
		status := SyncStatus{
			IsSync:       true,
			LastSyncTime: "", // Empty indicates start
		}
		e.brokerNotifier.SyncStatus(status)
	}()

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

	var lastError string
	for event := range ch {
		// Capture error if any
		if event.Data.Error != "" {
			lastError = event.Data.Error
		}
		for _, consumer := range consumers {
			consumer(event)
		}
	}

	// Final notification with error status if applicable
	notifyStatus(&Git{e.config}, e.brokerNotifier, lastError)
	e.mutex.Unlock()
}

func notifyStatus(git *Git, notifier *BrokerNotifier, lastError string) {
	localCommit, _ := git.LocalCommit()
	remoteCommit, _ := git.RemoteCommit()

	// Handle case where commit might be nil/empty (e.g. fresh repo)
	localDate := ""
	if localCommit != nil {
		localDate = localCommit.Time
	}

	remoteDate := ""
	if remoteCommit != nil {
		remoteDate = remoteCommit.Time
	}

	localId := ""
	if localCommit != nil {
		localId = localCommit.Id
	}

	remoteId := ""
	if remoteCommit != nil {
		remoteId = remoteCommit.Id
	}

	status := SyncStatus{
		LocalCommit:      localId,
		LocalCommitTime:  localDate,
		RemoteCommit:     remoteId,
		RemoteCommitTime: remoteDate,
		LastSyncTime:     time.Now().Format(time.RFC3339), // Use current time for sync timestamp
		IsSync:           false,                           // Final status is not "syncing" (which implies in-progress)
		Error:            lastError,
	}
	notifier.SyncStatus(status)
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

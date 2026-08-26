package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
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
				var errs []error
				for _, configPathInfo := range configPathsInfo {
					src := configPathInfo.Src.Name()
					if err := copyPath(src, configPathInfo.Dest); err != nil {
						errs = append(errs, fmt.Errorf("could not copy %s to %s: %w", src, configPathInfo.Dest, err))
						continue
					}

					Infoln(fmt.Sprintf("Synced: %s -> %s", src, configPathInfo.Dest))
				}

				return errors.Join(errs...)
			},
		},
	}
}

// copyPath copies src (file or directory) to dest, always replacing whatever
// is currently at dest — including an existing symlink, which is removed
// rather than followed. This keeps the git repo as the sole source of truth:
// syncing never silently writes through a stale link into some other location,
// and never merges into an existing directory's contents.
func copyPath(src, dest string) error {
	if _, err := os.Lstat(dest); err == nil {
		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("failed to remove existing %s: %w", dest, err)
		}
	}

	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source %s: %w", src, err)
	}

	if info.IsDir() {
		return copyDir(src, dest)
	}
	return copyFile(src, dest)
}

func copyDir(src, dest string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, destPath); err != nil {
				return err
			}
			continue
		}

		_, err := entry.Info()
		if err != nil {
			return err
		}
		if err := copyFile(srcPath, destPath); err != nil {
			return err
		}
	}

	return nil
}

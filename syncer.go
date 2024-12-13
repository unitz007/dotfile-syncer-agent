package main

import (
	"os"
	"os/exec"
	"sync"
	"time"
)

type Syncer struct {
	config         *Config
	db             Database
	brokerNotifier *BrokerNotifier
	lock           *sync.Mutex
}

func (s *Syncer) Sync(dotFilePath string, syncType string, ch chan SyncEvent) {
	defer s.lock.Unlock()

	s.lock.Lock()
	steps := []struct {
		Step   string
		Action func() error
	}{
		{
			Step: "Execute Git pull command",
			Action: func() error {
				err := os.Chdir(dotFilePath)
				if err != nil {
					return err
				}
				path, err := exec.LookPath("git")
				if err != nil {
					return err
				}

				return exec.Command(path, "pull", "origin", "main", "--rebase").Run()
			},
		},
		{
			Step: "Execute Stow command",
			Action: func() error {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return err
				}

				path, err := exec.LookPath("stow")
				if err != nil {
					return err
				}

				return exec.Command(path, ".", "-t", homeDir).Run()
			},
		},
		{
			Step: "Persisting Sync Details...",
			Action: func() error {
				syncStash := SyncStash{
					LastSyncType:   syncType,
					LastSyncTime:   time.Now().Format(time.RFC3339),
					LastSyncStatus: true,
				}

				return s.db.Create(&syncStash)
			},
		},
		{
			Step: "Finalizing Sync...",
			Action: func() error {
				return nil
			},
		},
	}

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

	for i, step := range steps {
		event.Data.Step = step.Step
		err := step.Action()
		if err != nil {
			event.Data.IsSuccess = false
			event.Data.Error = err.Error()

			s.brokerNotifier.SyncTrigger(event)
			ch <- event

			close(ch)
			return
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

		s.brokerNotifier.SyncTrigger(event)
		ch <- event
		time.Sleep(time.Second)
	}

	close(ch)
}

type SyncEvent struct {
	Data struct {
		Progress  int    `json:"progress"`
		IsSuccess bool   `json:"isSuccess"`
		Step      string `json:"step"`
		Error     string `json:"error"`
		Done      bool   `json:"done"`
	} `json:"data"`
}

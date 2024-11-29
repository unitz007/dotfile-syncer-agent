package main

import (
	"os"
	"os/exec"
	"time"
)

type Syncer struct {
	config     *Config
	db         Database
	httpClient HttpClient
}

func (s *Syncer) Sync(dotFilePath string, syncType string, ch chan SyncEvent) {

	event1 := SyncEvent{
		Data: struct {
			Progress  int    `json:"progress"`
			IsSuccess bool   `json:"isSuccess"`
			Step      string `json:"step"`
			Error     string `json:"error"`
			Done      bool   `json:"done"`
		}{Progress: 0, IsSuccess: true, Done: false},
	}

	NewSyncProcess(event1).
		Add("Execute Git pull command", func() error {
			err := os.Chdir(dotFilePath)
			if err != nil {
				return err
			}
			path, err := exec.LookPath("git")
			if err != nil {
				return err
			}

			return exec.Command(path, "pull", "origin", "main", "--rebase").Run()
		}).
		Add().
		Add()

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
			Step: "Finalize sync...",
			Action: func() error {
				remoteCommits, err := s.httpClient.GetCommits()
				if err != nil {
					return err
				}

				headCommit := remoteCommits[0]
				commit := &Commit{
					Id:   headCommit.Sha,
					Time: "",
				}

				syncStash := SyncStash{
					Commit: commit,
					Type:   syncType,
					Time:   time.Now().Format(time.RFC3339),
				}

				return s.db.Create(&syncStash)
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
			ch <- event
			close(ch)
			return
		}

		event.Data.IsSuccess = true
		event.Data.Progress += constant
		event.Data.Step = step.Step
		if i == len(steps)-1 { // on final step
			event.Data.Done = true
			progress := event.Data.Progress
			if progress != 100 {
				event.Data.Progress += 100 - progress
			}
		}
		event.Data.Done = func() bool {
			return i == len(steps)-1
		}()

		ch <- event
		time.Sleep(1 * time.Second)
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

type SyncProcess struct {
	ch    chan SyncEvent
	event SyncEvent
}

func (s SyncProcess) Add(name string, action func() error) SyncProcess {
	err := action()
	if err != nil {
		s.event.Data.IsSuccess = false
		s.event.Data.Error = err.Error()
		s.ch <- s.event
		close(s.ch)
		return s
	}

	s.event.Data.IsSuccess = true
	s.event.Data.Progress += 10
	s.event.Data.Step = name
	//if  == len(s.ch)-1 { // on final step
	//	event.Data.Done = true
	//	progress := s.event.Data.Progress
	//	if progress != 100 {
	//		event.Data.Progress += 100 - progress
	//	}
	//}
	s.event.Data.Done = func() bool {
		return true
	}()

	s.ch <- s.event
	time.Sleep(1 * time.Second)

	return s
}

func NewSyncProcess(syncEvent SyncEvent) SyncProcess {
	return SyncProcess{
		ch:    make(chan SyncEvent),
		event: syncEvent,
	}
}

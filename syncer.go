package main

import (
	"os"
	"os/exec"
	"time"
)

type Syncer interface {
	Sync()
	Consume(consumers ...func(syncEvent SyncEvent))
	//Rollback(commitId string, ch chan SyncEvent)
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

type syncer struct {
	config         *Configurations
	db             Persistence
	brokerNotifier *BrokerNotifier
	ch             chan SyncEvent
}

func NewSyncer(config *Configurations, db Persistence, brokerNotifier *BrokerNotifier) Syncer {
	return &syncer{
		config:         config,
		db:             db,
		brokerNotifier: brokerNotifier,
		ch:             make(chan SyncEvent),
	}
}

func (s *syncer) Sync() {
	steps := []struct {
		Step   string
		Action func() error
	}{
		{
			Step: "Execute Git pull command",
			Action: func() error {
				err := os.Chdir(s.config.DotfilePath)
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
				//	syncStash := SyncStash{
				//		LastSyncType:   syncType,
				//		LastSyncTime:   time.Now().Format(time.RFC3339),
				//		LastSyncStatus: true,
				//	}
				//
				//	return s.db.Create(&syncStash)
				return nil
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

	notify(&Git{s.config}, s.brokerNotifier)

	for i, step := range steps {
		event.Data.Step = step.Step
		err := step.Action()
		if err != nil {
			event.Data.IsSuccess = false
			event.Data.Error = err.Error()
			s.ch <- event
			close(s.ch)
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
		s.ch <- event
		time.Sleep(time.Second)
	}

	notify(&Git{s.config}, s.brokerNotifier)
	close(s.ch)
}

func (s *syncer) Consume(consumers ...func(event SyncEvent)) {

	// default consumers
	consumers = append(consumers, func(event SyncEvent) {
		s.brokerNotifier.SyncTrigger(event)
	})

	for event := range s.ch {
		for _, consumer := range consumers {
			go consumer(event)
		}
		time.Sleep(1 * time.Second)
	}

	s.ch = make(chan SyncEvent)
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

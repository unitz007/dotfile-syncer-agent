package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
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
	//var events []SyncEvent
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

			go notifyBroker[SyncEvent](event)
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

		go notifyBroker[SyncEvent](event)
		ch <- event
		time.Sleep(time.Second)
	}

	close(ch)
}

func notifyBroker[T any](payload T) {
	machineId, mOk := os.LookupEnv("DOTFILE_MACHINE_ID")
	brokerUrl, bOk := os.LookupEnv("DOTFILE_BROKER_URL")

	if mOk && bOk {
		go func() {
			v, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", brokerUrl+"/sync-trigger/"+machineId+"/notify", bytes.NewBuffer(v))
			request.Header.Set("Content-Type", "application/json")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				Error("Failed to send notification to broker:", err.Error())
				return
			}

			if response.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(response.Body)
				Error("Failed to send notification to broker:", string(body))
				return
			}
		}()
	}

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

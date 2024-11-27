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

	constant := 12
	progress := constant
	event := SyncEvent{
		Data: struct {
			Progress  int    `json:"progress"`
			IsSuccess bool   `json:"isSuccess"`
			Step      string `json:"step"`
			Error     string `json:"error"`
			Done      bool   `json:"done"`
		}{Progress: 0, IsSuccess: true, Done: false},
	}

	Info("Sync started...")
	ch <- event

	event.Data.Step = "Change to dotfile directory"
	err := os.Chdir(dotFilePath)
	if err != nil {
		event.Data.IsSuccess = false
		event.Data.Error = err.Error()
		ch <- event
		close(ch)
		return
	}

	event.Data.IsSuccess = true
	event.Data.Progress += constant
	event.Data.Done = false
	ch <- event

	// look up git
	event.Data.Step = "Look-Up git executable"
	path, err := exec.LookPath("git")
	if err != nil {
		event.Data.IsSuccess = false
		event.Data.Error = err.Error()
		ch <- event
		close(ch)
		return
	}

	event.Data.IsSuccess = true
	event.Data.Progress += constant
	event.Data.Done = false
	ch <- event

	// `git pull origin main` command
	event.Data.Step = "Execute 'git pull' command"
	err = exec.Command(path, "pull", "origin", "main", "--rebase").Run()
	if err != nil {
		event.Data.IsSuccess = false
		event.Data.Error = err.Error()
		ch <- event
		close(ch)
		return
	}

	event.Data.IsSuccess = true
	event.Data.Progress += constant
	event.Data.Done = false
	ch <- event

	event.Data.Step = "Get system home directory"
	homeDir, err := os.UserHomeDir()
	if err != nil {
		event.Data.IsSuccess = false
		event.Data.Error = err.Error()
		ch <- event
		close(ch)
		return
	}

	event.Data.IsSuccess = true
	event.Data.Progress += progress
	event.Data.Done = false
	ch <- event

	// `stow .` command
	event.Data.Step = "Look-Up stow executable"
	path, err = exec.LookPath("stow")
	if err != nil {
		event.Data.IsSuccess = false
		event.Data.Error = err.Error()
		ch <- event
		close(ch)
		return
	}

	event.Data.IsSuccess = true
	event.Data.Progress += constant
	event.Data.Done = false
	ch <- event

	event.Data.Step = "Execute stow command"
	err = exec.Command(path, ".", "-t", homeDir).Run()
	if err != nil {
		event.Data.IsSuccess = false
		event.Data.Error = err.Error()
		ch <- event
		close(ch)
		return
	}

	event.Data.IsSuccess = true
	event.Data.Progress += progress
	event.Data.Done = false
	ch <- event

	event.Data.Step = "Get github remote commits"
	remoteCommits, err := s.httpClient.GetCommits()
	if err != nil {
		event.Data.IsSuccess = false
		event.Data.Error = err.Error()
		ch <- event
		close(ch)
		return
	}

	event.Data.IsSuccess = true
	event.Data.Progress += progress
	event.Data.Done = false
	ch <- event

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

	event.Data.Step = "Persist git sync state"
	err = s.db.Create(&syncStash)
	if err != nil {
		event.Data.IsSuccess = false
		event.Data.Error = err.Error()
		ch <- event
		close(ch)
		return
	}

	event.Data.IsSuccess = true
	event.Data.Progress += constant + 4
	event.Data.Done = true
	ch <- event

	Info("Sync completed...")
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

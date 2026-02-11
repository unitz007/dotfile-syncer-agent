package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// BrokerNotifier handles communication with an external broker service for monitoring and notifications.
// It sends sync events and status updates to a centralized broker for multi-machine coordination.
type BrokerNotifier struct {
	machine   string // Unique machine identifier from DOTFILE_MACHINE_ID env var
	brokerUrl string // Broker service URL from DOTFILE_BROKER_URL env var
	git       *Git   // Git instance for accessing repository information
}

// SyncStatus represents the synchronization state sent to the broker
type SyncStatus struct {
	LocalCommit  string `json:"local_commit"`  // Local repository commit SHA
	RemoteCommit string `json:"remote_commit"` // Remote repository commit SHA
	IsSync       bool   `json:"is_sync"`       // Whether local and remote are synchronized
}

// Machine represents a machine registered with the broker service
type Machine struct {
	Id         string     `json:"_id"`          // Unique machine identifier
	SyncStatus SyncStatus `json:"sync_details"` // Current sync status of the machine
}

// NewBrokerNotifier creates a new BrokerNotifier instance.
// It reads configuration from environment variables DOTFILE_MACHINE_ID and DOTFILE_BROKER_URL.
// If either is missing, the broker functionality is disabled but the agent continues to work.
func NewBrokerNotifier(git *Git) *BrokerNotifier {

	machine := os.Getenv("DOTFILE_MACHINE_ID")
	brokerUrl := os.Getenv("DOTFILE_BROKER_URL")

	if machine != "" || brokerUrl != "" {
		Infoln("Broker notifier is enabled")
	}

	return &BrokerNotifier{machine, brokerUrl, git}
}

// SyncEvent sends a sync progress event to the broker service.
// This allows real-time monitoring of sync operations across multiple machines.
// Only sends if both machine ID and broker URL are configured.
func (b BrokerNotifier) SyncEvent(payload SyncEvent) {
	if b.machine != "" && b.brokerUrl != "" {
		v, _ := json.Marshal(payload)
		request, err := http.NewRequest("POST", b.brokerUrl+"/machines/"+b.machine+"/sync-event", bytes.NewBuffer(v))
		if err != nil {
			Error("Failed to send notification to broker:", err.Error())
			return
		}
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
	}
}

// SyncStatus sends the current sync status to the broker service.
// This updates the broker with the latest local/remote commit information.
// Runs asynchronously in a goroutine to avoid blocking the sync process.
func (b BrokerNotifier) SyncStatus(payload any) {
	if b.machine != "" && b.brokerUrl != "" {
		go func() {
			v, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", b.brokerUrl+"/machines/"+b.machine+"/sync-status", bytes.NewBuffer(v))
			request.Header.Set("Content-Type", "application/json")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				fmt.Println(err)
				Error("Failed to send notification to broker:", err.Error())
				return
			}

			if response.StatusCode != 200 {
				Error("Failed to send notification to broker:", response.Status)
			}
		}()
	}
}

// RegisterStream registers this machine with the broker service.
// It sends the machine ID and current local commit information.
// This is called on startup to announce the machine's presence to the broker.
// Runs asynchronously in a goroutine.
func (b BrokerNotifier) RegisterStream() {
	if b.machine != "" && b.brokerUrl != "" {
		go func() {

			localCommit, err := b.git.LocalCommit()
			if err != nil {
				Error("Failed to get local commit:", err.Error())
				return
			}
			machine := Machine{
				Id: b.machine,
				SyncStatus: SyncStatus{
					LocalCommit: localCommit.Id,
				},
			}

			body, err := json.Marshal(machine)
			if err != nil {
				Error("Failed to marshal machine:", err.Error())
				return
			}

			res, err := http.Post(b.brokerUrl+"/machines", "application/json", strings.NewReader(string(body)))
			if err != nil {
				Error("Unable to send broker notifier:", err.Error())
				return
			}

			if res.StatusCode != http.StatusNoContent {
				Error("Unable to send broker notifier:", res.Status)
				return
			}
		}()
	}
}

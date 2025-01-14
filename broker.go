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

type BrokerNotifier struct {
	machine   string
	brokerUrl string
	git       *Git
}

type SyncStatus struct {
	LocalCommit  string `json:"local_commit"`
	RemoteCommit string `json:"remote_commit"`
	IsSync       bool   `json:"is_sync"`
}

type Machine struct {
	Id         string     `json:"_id"`
	SyncStatus SyncStatus `json:"sync_details"`
}

func NewBrokerNotifier(git *Git) *BrokerNotifier {

	machine := os.Getenv("DOTFILE_MACHINE_ID")
	brokerUrl := os.Getenv("DOTFILE_BROKER_URL")

	if machine != "" || brokerUrl != "" {
		Infoln("Broker notifier is enabled")
	}

	return &BrokerNotifier{machine, brokerUrl, git}
}

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

func (b BrokerNotifier) RegisterStream() {
	if b.machine != "" && b.brokerUrl != "" {
		go func() {

			localCommit, _ := b.git.LocalCommit()
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

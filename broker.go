package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type BrokerNotifier struct {
	machine   string
	brokerUrl string
}

func NewBrokerNotifier() *BrokerNotifier {

	machine := os.Getenv("DOTFILE_MACHINE_ID")
	brokerUrl := os.Getenv("DOTFILE_BROKER_URL")

	return &BrokerNotifier{machine, brokerUrl}
}

func (b BrokerNotifier) SyncTrigger(payload any) {
	if b.machine != "" && b.brokerUrl != "" {
		go func() {
			v, _ := json.Marshal(payload)
			request, err := http.NewRequest("POST", b.brokerUrl+"/sync-trigger/"+b.machine+"/notify", bytes.NewBuffer(v))
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

func (b BrokerNotifier) SyncStatus(payload any) {
	if b.machine != "" && b.brokerUrl != "" {
		go func() {
			v, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", b.brokerUrl+"/sync-status/"+b.machine+"/notify", bytes.NewBuffer(v))
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
			res, err := http.Post(b.brokerUrl+"/machines/"+b.machine, "application/json", nil)
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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/r3labs/sse/v2"
)

type SyncHandler struct {
	syncer *Syncer
	git    *Git
	server *sse.Server
}

func NewSyncHandler(syncer *Syncer, git *Git, server *sse.Server) *SyncHandler {
	return &SyncHandler{
		syncer,
		git,
		server,
	}
}

func (s SyncHandler) Sync(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	remoteCommit := func() *Commit {
		commit, err := s.git.RemoteCommit()
		if err != nil {
			Error(err.Error())
			return nil
		}

		return commit
	}

	switch request.Method {
	case http.MethodPost: // POST
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("Connection", "keep-alive")

		ch := make(chan SyncEvent)

		go s.syncer.Sync(s.syncer.config.DotfilePath, ManualSync, ch)
		fmt.Print("Manual sync triggered===(0%)")
		for x := range ch {
			fmt.Print("===(" + strconv.Itoa(x.Data.Progress) + "%)")
			isSuccessful := x.Data.IsSuccess
			if !isSuccessful {
				msg := fmt.Sprintf("'%s': [%s]", x.Data.Step, x.Data.Error)
				Error("Sync Failed: Could not", msg)
			}
			v, _ := json.Marshal(x.Data)
			_, _ = fmt.Fprintf(writer, "data: %v\n\n", string(v))
			writer.(http.Flusher).Flush() // Send the event immediately
			if x.Data.Done {
				time.Sleep(time.Second)
				fmt.Printf("===completed")
				time.Sleep(time.Second)
				fmt.Println()

			}
			time.Sleep(1 * time.Second) // Simulate periodic updates
		}

	case http.MethodGet: // GET
		stream := request.URL.Query().Get("stream")
		if stream == SyncTriggerLabel {
			s.server.ServeHTTP(writer, request)
			go func() {
				<-request.Context().Done()
				return
			}()
		} else if stream == SyncStatusLabel {
			s.server.ServeHTTP(writer, request)
			go func() {
				<-request.Context().Done()
				return
			}()
		} else {
			localCommit, err := s.git.LocalCommit()
			if err != nil {
				Error(err.Error())
				return
			}

			remoteCommit := remoteCommit()

			response := InitGitTransform(localCommit, remoteCommit)
			response.LastSyncTime = localCommit.Time

			writeResponse(writer, "Successful", response)
		}
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func writeResponse(writer io.Writer, msg string, payload any) {
	body := make(map[string]any, 2)
	body["msg"] = msg
	body["payload"] = payload
	_ = json.NewEncoder(writer).Encode(body)

}

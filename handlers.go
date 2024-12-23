package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

		d := *s.syncer

		go d.Sync()
		d.Consume(ConsoleSyncConsumer, func(event SyncEvent) {
			data := event.Data
			v, _ := json.Marshal(data)
			_, _ = fmt.Fprintf(writer, "data: %v\n\n", string(v))
			writer.(http.Flusher).Flush() // Send the event immediately
		})

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

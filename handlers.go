package main

import (
	"encoding/json"
	"fmt"
	"github.com/r3labs/sse/v2"
	"io"
	"net/http"
	"time"
)

type SyncHandler struct {
	syncer     *Syncer
	db         Database
	httpClient HttpClient
	server     *sse.Server
}

func NewSyncHandler(syncer *Syncer, db Database, httpClient HttpClient, server *sse.Server) *SyncHandler {
	return &SyncHandler{
		syncer,
		db,
		httpClient,
		server,
	}
}

func (s SyncHandler) Sync(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	remoteCommit := func(c HttpClient) *Commit {
		remoteCommitResponse, err := s.httpClient.GetCommits()
		if err != nil {
			Error(err.Error())
			return nil
		}

		commit := remoteCommitResponse[0]

		return &Commit{
			Id: commit.Sha,
		}
	}

	switch request.Method {
	case http.MethodPost: // POST
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("Connection", "keep-alive")

		ch := make(chan SyncEvent)

		go s.syncer.Sync(s.syncer.config.DotfilePath, "Manual", ch)
		for x := range ch {
			isSuccessful := x.Data.IsSuccess
			if !isSuccessful {
				msg := fmt.Sprintf("'%s': [%s]", x.Data.Step, x.Data.Error)
				Error("Sync Failed: Could not", msg)
			}
			v, _ := json.Marshal(x.Data)
			_, _ = fmt.Fprintf(writer, "data: %v\n\n", string(v))
			writer.(http.Flusher).Flush() // Send the event immediately
			time.Sleep(1 * time.Second)   // Simulate periodic updates
		}
	case http.MethodGet: // GET
		stream := request.URL.Query().Get("stream")
		if stream == "sync-trigger" {
			Info(request.UserAgent(), "is connected to stream")
			s.server.ServeHTTP(writer, request)
			go func() {
				Info(request.UserAgent(), "disconnected from stream")
				<-request.Context().Done()
				return
			}()
		} else if stream == "sync-status" {
			writer.Header().Set("Content-Type", "text/event-stream")
			writer.Header().Set("Cache-Control", "no-cache")
			writer.Header().Set("Connection", "keep-alive")
			for {
				syncStatus, err := s.db.Get(1)
				if err != nil {
					Error(err.Error())
					return
				}

				remoteCommit := remoteCommit(s.httpClient)

				response := InitGitTransform(syncStatus.Commit, remoteCommit)
				response.LastSyncTime = syncStatus.Time
				response.LastSyncType = syncStatus.Type

				v, _ := json.Marshal(response)

				_, _ = fmt.Fprintf(writer, "data: %v\n\n", string(v))
				writer.(http.Flusher).Flush() // Send the event immediately

				time.Sleep(30 * time.Second)
			}
		} else {
			syncStatus, err := s.db.Get(1)
			if err != nil {
				Error(err.Error())
				return
			}

			remoteCommit := remoteCommit(s.httpClient)

			response := InitGitTransform(syncStatus.Commit, remoteCommit)
			response.LastSyncTime = syncStatus.Time
			response.LastSyncType = syncStatus.Type

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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/r3labs/sse/v2"
)

// SyncHandler handles HTTP requests for synchronization operations.
// It supports both manual sync triggers (POST) and status queries (GET).
type SyncHandler struct {
	syncer *Syncer     // The syncer implementation to use for sync operations
	git    *Git        // Git instance for repository operations
	server *sse.Server // Server-Sent Events server for real-time updates
}

// NewSyncHandler creates a new SyncHandler with the provided dependencies
func NewSyncHandler(syncer *Syncer, git *Git, server *sse.Server) *SyncHandler {
	return &SyncHandler{
		syncer,
		git,
		server,
	}
}

// Sync handles HTTP requests to the /sync endpoint.
// POST: Triggers a manual sync and streams progress via Server-Sent Events
// GET: Returns current sync status or establishes SSE connection based on query params
//   - ?stream=sync-trigger: SSE stream for sync trigger events
//   - ?stream=sync-status: SSE stream for sync status updates
//   - No stream param: Returns JSON with current sync status
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
	case http.MethodPost: // POST - Trigger manual sync
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("Connection", "keep-alive")

		d := *s.syncer

		// Execute sync and stream progress events to client
		d.Sync(ConsoleSyncConsumer, func(event SyncEvent) {
			data := event.Data
			v, _ := json.Marshal(data)
			_, _ = fmt.Fprintf(writer, "data: %v\n\n", string(v))
			writer.(http.Flusher).Flush() // Send the event immediately
		})

	case http.MethodGet: // GET - Query sync status or establish SSE connection
		stream := request.URL.Query().Get("stream")
		if stream == SyncTriggerLabel {
			// Establish SSE connection for sync trigger events
			s.server.ServeHTTP(writer, request)
			go func() {
				<-request.Context().Done()
				return
			}()
		} else if stream == SyncStatusLabel {
			// Establish SSE connection for sync status updates
			s.server.ServeHTTP(writer, request)
			go func() {
				<-request.Context().Done()
				return
			}()
		} else {
			// Return current sync status as JSON
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

// writeResponse writes a JSON response with a message and payload
func writeResponse(writer io.Writer, msg string, payload any) {
	body := make(map[string]any, 2)
	body["msg"] = msg
	body["payload"] = payload
	_ = json.NewEncoder(writer).Encode(body)

}

package main

import (
	"encoding/json"
	"io"
	"net/http"
)

type SyncHandler struct {
	syncer     *Syncer
	db         Database
	httpClient HttpClient
}

func NewSyncHandler(syncer *Syncer, db Database, httpClient HttpClient) *SyncHandler {
	return &SyncHandler{
		syncer,
		db,
		httpClient,
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
		err := s.syncer.Sync(s.syncer.config.DotfilePath, "Manual")
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			Error(err.Error())
			writeResponse(writer, "Sync failed", err.Error())
		} else {
			writeResponse(writer, "Sync completed...", nil)
		}
	case http.MethodGet: // GET

		syncStatus, err := s.db.Get(1)
		if err != nil {
			Error(err.Error())
			return
		}

		remoteCommit := remoteCommit(s.httpClient)

		response := InitGitTransform(syncStatus.Commit, remoteCommit)
		response.LastSyncTime = syncStatus.Time
		response.LastSyncType = syncStatus.Type

		writeResponse(writer, "sync details fetched successfully", response)
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

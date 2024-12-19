package main

import "time"

type Commit struct {
	Id   string `json:"id"`
	Time string `json:"commit_time"`
}

type GitWebHookCommitResponse struct {
	Ref        string `json:"ref"`
	HeadCommit Commit `json:"head_commit"`
}

type SyncStatusResponse struct {
	IsSync       bool    `json:"is_synced"`
	LastSyncTime string  `json:"last_sync_time"`
	RemoteCommit *Commit `json:"remote_commit"`
	LocalCommit  *Commit `json:"local_commit"`
}

type GitHttpCommitResponse struct {
	Sha    string `json:"sha"`
	Commit struct {
		Author struct {
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

type SyncStash struct {
	LastSyncType   string
	LastSyncTime   string
	LastSyncStatus bool
}

func InitGitTransform(
	localCommit *Commit,
	remoteCommit *Commit,
) SyncStatusResponse {
	return SyncStatusResponse{
		LocalCommit:  localCommit,
		LastSyncTime: time.Now().UTC().Format(time.RFC3339),
		RemoteCommit: remoteCommit,
		IsSync: func() bool {
			if localCommit != nil && remoteCommit != nil {
				return localCommit.Id == remoteCommit.Id
			}
			return false
		}(),
	}
}

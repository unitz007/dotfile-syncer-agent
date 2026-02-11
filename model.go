package main

import "time"

// Commit represents a Git commit with its ID and timestamp
type Commit struct {
	Id   string `json:"id"`          // Git commit SHA hash
	Time string `json:"commit_time"` // Commit timestamp in RFC3339 format
}

// GitWebHookCommitResponse represents the payload received from Git webhook events.
// This is used when the agent receives push notifications from the Git provider.
type GitWebHookCommitResponse struct {
	Ref        string `json:"ref"`         // Git reference (e.g., "refs/heads/main")
	HeadCommit Commit `json:"head_commit"` // The latest commit information
}

// SyncStatusResponse represents the current synchronization status between local and remote repositories
type SyncStatusResponse struct {
	IsSync       bool    `json:"is_synced"`      // Whether local and remote are in sync
	LastSyncTime string  `json:"last_sync_time"` // Timestamp of last sync operation
	RemoteCommit *Commit `json:"remote_commit"`  // Latest commit on remote repository
	LocalCommit  *Commit `json:"local_commit"`   // Latest commit in local repository
}

// GitHttpCommitResponse represents the response from GitHub API when fetching commit information
type GitHttpCommitResponse struct {
	Sha    string `json:"sha"` // Commit SHA hash
	Commit struct {
		Author struct {
			Date string `json:"date"` // Author date in ISO 8601 format
		} `json:"author"`
	} `json:"commit"`
}

// SyncStash represents persisted sync metadata stored in the local database
type SyncStash struct {
	LastSyncType   string // Type of sync: "Manual" or "Automatic"
	LastSyncTime   string // Timestamp of last sync in RFC3339 format
	LastSyncStatus bool   // Whether the last sync was successful
}

// InitGitTransform creates a SyncStatusResponse from local and remote commit information.
// It compares the commits to determine if they are in sync.
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

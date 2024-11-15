package main

type Commit struct {
	Id   string `json:"id"`
	Time string `json:"commit_time"`
}

type GitWebHookCommitResponse struct {
	Event string `json:"x-github-event"`
	Body  struct {
		Id         string `json:"id"`
		HeadCommit Commit `json:"head_commit"`
	} `json:"body"`
}

type GitPullTransform struct {
	IsSync       bool    `json:"is_synced"`
	LastSyncTime string  `json:"last_sync_time"`
	LastSyncType string  `json:"last_sync_type"`
	RemoteCommit *Commit `json:"remote_commit"`
	LocalCommit  *Commit `json:"local_commit"`
}

type GitHttpCommitResponse struct {
	Sha    string `json:"sha"`
	Commit struct {
		Name string `json:"name"`
	} `json:"commit"`
}

type SyncStash struct {
	Commit *Commit
	Time   string
	Type   string
}

func InitGitTransform(
	localCommit *Commit,
	remoteCommit *Commit,
) GitPullTransform {
	return GitPullTransform{
		LocalCommit:  localCommit,
		RemoteCommit: remoteCommit,
		IsSync: func() bool {
			if localCommit != nil && remoteCommit != nil {
				return localCommit.Id == remoteCommit.Id
			}
			return false
		}(),
	}
}

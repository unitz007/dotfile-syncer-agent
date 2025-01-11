package main

import (
	"bytes"
	"encoding/json"
	"strings"
)

type SseClient struct {
	Syncer Syncer
}

func (w *SseClient) Write(p []byte) (n int, err error) {

	_, after, found := bytes.Cut(p, []byte("data:"))
	if found {
		sAfter := strings.TrimSpace(string(after))
		var commit *GitWebHookCommitResponse
		err = json.Unmarshal([]byte(sAfter), &commit)
		if err != nil {
			return 0, err
		}

		commitRef := commit.Ref
		if commitRef != "" {
			branch := strings.Split(commitRef, "/")[2]
			if branch == "main" { // only triggers sync on push to main branch
				w.Syncer.Sync(ConsoleSyncConsumer)
			}
		}
	}

	return len(p), nil
}

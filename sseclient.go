package main

import (
	"bytes"
	"encoding/json"
	"strings"
)

// SseClient implements io.Writer to parse Server-Sent Events from Git webhooks.
// It triggers automatic synchronization when commits are pushed to the main branch.
type SseClient struct {
	Syncer Syncer // The syncer to trigger when webhook events are received
}

// Write implements io.Writer interface to process SSE data from webhook responses.
// It parses the SSE data field, extracts commit information, and triggers sync
// if the commit is on the main branch.
func (w *SseClient) Write(p []byte) (n int, err error) {

	// Extract data after "data:" prefix in SSE format
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
			// Extract branch name from ref (e.g., "refs/heads/main" -> "main")
			branch := strings.Split(commitRef, "/")[2]
			if branch == "main" { // only triggers sync on push to main branch
				w.Syncer.Sync(ConsoleSyncConsumer)
			}
		}
	}

	return len(p), nil
}

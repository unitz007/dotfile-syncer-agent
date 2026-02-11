package main

// Application-wide constants used throughout the dotfile agent

const (
	// SyncStatusLabel is the query parameter value for requesting sync status via SSE
	SyncStatusLabel = "sync-status"

	// SyncTriggerLabel is the query parameter value for triggering sync via SSE
	SyncTriggerLabel = "sync-trigger"

	// DefaultPort is the default HTTP port the agent listens on
	DefaultPort = "3000"

	// ManualSync indicates a sync was triggered manually by the user
	ManualSync = "Manual"

	// AutomaticSync indicates a sync was triggered automatically (webhook or polling)
	AutomaticSync = "Automatic"
)

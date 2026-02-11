package main

// Syncer defines the interface for dotfile synchronization implementations.
// Different syncers can implement different strategies (custom, stow, etc.)
type Syncer interface {
	// Sync performs the synchronization process and notifies consumers of progress
	Sync(consumers ...Consumer)
}

// Consumer is a callback function that receives sync events during synchronization.
// Multiple consumers can be registered to handle events differently (logging, UI updates, etc.)
type Consumer func(syncEvent SyncEvent)

// SyncEvent represents a synchronization progress event sent to consumers
type SyncEvent struct {
	// Data contains the current state of the sync operation
	Data struct {
		Progress  int    `json:"progress"`  // Percentage complete (0-100)
		IsSuccess bool   `json:"isSuccess"` // Whether the current step succeeded
		Step      string `json:"step"`      // Description of current step
		Error     string `json:"error"`     // Error message if IsSuccess is false
		Done      bool   `json:"done"`      // Whether the entire sync is complete
	} `json:"data"`
}

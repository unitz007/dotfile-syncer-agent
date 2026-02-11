package main

import (
	"os"
	"path/filepath"

	"github.com/haibeey/doclite"
)

// Persistence defines the interface for storing and retrieving sync metadata
type Persistence interface {
	Create(data *SyncStash) error   // Create or update sync stash record
	Get(id int) (*SyncStash, error) // Retrieve sync stash by ID
}

// docliteImpl implements Persistence using the doclite embedded database
type docliteImpl struct {
	data   *doclite.Doclite // Doclite database instance
	config *Configurations  // Agent configuration
}

// Get retrieves a SyncStash record by its ID from the database.
// It iterates through documents to find the matching ID.
func (db *docliteImpl) Get(id int) (*SyncStash, error) {
	defer db.close()
	syncStash := &SyncStash{}

	var i int64 = 0
	var err error

	// Iterate through documents to find the one with matching ID
	for i = 0; i <= db.data.Base().GetCol().NumDocuments; i++ {

		if id == int(i) {
			err = db.data.Base().FindOne(i, syncStash)
			break
		}
	}

	return syncStash, err
}

// InitializePersistence creates and initializes the persistence layer.
// It creates the config directory if it doesn't exist and opens the doclite database.
func InitializePersistence(config *Configurations) (Persistence, error) {

	configDir := config.ConfigPath

	// Create config directory if it doesn't exist
	if _, err := os.Stat(configDir); err != nil && os.IsNotExist(err) {
		err = os.Mkdir(configDir, 0700)
		if err != nil {
			return nil, err
		}
	}

	// Connect to doclite database file
	col := doclite.Connect(filepath.Join(configDir, "dotfile-agent.doclite"))
	db := &docliteImpl{
		data: col,
	}

	return db, nil

}

// Create inserts or updates a SyncStash record in the database.
// If a record with ID 1 exists, it updates it; otherwise, it inserts a new record.
func (db *docliteImpl) Create(data *SyncStash) error {
	defer db.close()

	var err error
	localCommitId := 1

	// Check if record already exists
	isExists := func() bool {
		_, err := db.Get(localCommitId)
		if err == nil {
			return true
		}

		return false
	}()

	if isExists {
		// Update existing record
		err = db.data.Base().UpdateOneDoc(int64(localCommitId), data)
	} else {
		// Insert new record
		_, err = db.data.Base().Insert(data)
	}

	if err != nil {
		return err
	}

	return nil
}

// close closes the doclite database connection
func (db *docliteImpl) close() {
	func(data *doclite.Doclite) {
		err := data.Close()
		if err != nil {
			Error("failed to close doclite")
		}
	}(db.data)
}

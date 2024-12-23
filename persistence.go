package main

import (
	"os"
	"path/filepath"

	"github.com/haibeey/doclite"
)

type Persistence interface {
	Create(data *SyncStash) error
	Get(id int) (*SyncStash, error)
}

type docliteImpl struct {
	data   *doclite.Doclite
	config *Configurations
}

func (db *docliteImpl) Get(id int) (*SyncStash, error) {
	defer db.close()
	syncStash := &SyncStash{}

	var i int64 = 0
	var err error

	for i = 0; i <= db.data.Base().GetCol().NumDocuments; i++ {

		if id == int(i) {
			err = db.data.Base().FindOne(i, syncStash)
			break
		}
	}

	// if syncStash == nil {
	// 	err = errors.New("not found")
	// }

	return syncStash, err
}

func InitializePersistence(config *Configurations) (Persistence, error) {

	configDir := config.ConfigPath

	if _, err := os.Stat(configDir); err != nil && os.IsNotExist(err) {
		err = os.Mkdir(configDir, 0700)
		if err != nil {
			return nil, err
		}
	}

	col := doclite.Connect(filepath.Join(configDir, "dotfile-agent.doclite"))
	db := &docliteImpl{
		data: col,
	}

	return db, nil

}

func (db *docliteImpl) Create(data *SyncStash) error {
	defer db.close()

	var err error
	localCommitId := 1
	isExists := func() bool {
		_, err := db.Get(localCommitId)
		if err == nil {
			return true
		}

		return false
	}()

	if isExists {
		err = db.data.Base().UpdateOneDoc(int64(localCommitId), data)
	} else {
		_, err = db.data.Base().Insert(data)
	}

	if err != nil {
		return err
	}

	return nil
}

func (db *docliteImpl) close() {
	func(data *doclite.Doclite) {
		err := data.Close()
		if err != nil {
			Error("failed to close doclite")
		}
	}(db.data)
}

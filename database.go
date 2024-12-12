package main

import (
	"errors"
	"github.com/haibeey/doclite"
	"os"
	"path/filepath"
)

type Database interface {
	Create(data *SyncStash) error
	Get(id int) (*SyncStash, error)
}

type docliteImpl struct {
	data *doclite.Doclite
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

	if syncStash == nil {
		err = errors.New("not found")
	}

	return syncStash, err
}

func InitDB() Database {

	configDir, err := os.UserConfigDir()
	if err != nil {
		Error(err.Error())
		os.Exit(1)
	}

	configDir = filepath.Join(configDir, "dotfile-syncer")

	if _, err := os.Stat(configDir); err != nil && os.IsNotExist(err) {
		err = os.Mkdir(configDir, 0700)
		if err != nil {
			Error(err.Error())
			os.Exit(1)
		}
	}

	col := doclite.Connect(filepath.Join(configDir, "dotfile-agent.doclite"))

	return &docliteImpl{
		data: col,
	}

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

package main

import (
	"fmt"
	"os"
	"path"
)

func acquireAgentLock(config *Configurations) (func() error, error) {
	if config == nil || config.ConfigPath == "" {
		return func() error { return nil }, nil
	}

	lockPath := path.Join(config.ConfigPath, "agent.lock")

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("another agent instance appears to be running for this config directory; lock file exists at %s", lockPath)
		}
		return nil, err
	}

	_, _ = file.Write([]byte("locked"))

	release := func() error {
		err := file.Close()
		if err != nil {
			return err
		}
		removeErr := os.Remove(lockPath)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
		return nil
	}

	return release, nil
}


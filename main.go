package main

import (
	"encoding/json"
	"github.com/r3labs/sse/v2"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"path"
	"time"
)

func main() {

	var (
		db                      = InitDB()
		httpClient              = NewHttpClient()
		rootCmd                 = cobra.Command{}
		defaultDotFileDirectory = func() string {
			homeDir, err := os.UserConfigDir()
			if err != nil {
				Error("Unable to access home directory:", err.Error())
				os.Exit(1)
			}

			return path.Join(homeDir, "dotfiles")
		}()
	)

	port := rootCmd.Flags().StringP("port", "p", "3000", "HTTP port to run on")
	webhookUrl := rootCmd.Flags().StringP("webhook", "w", "https://smee.io/awFay3gs7LCGYe2", "git webhook url")
	dotFilePath := rootCmd.Flags().StringP("dotfile-path", "d", defaultDotFileDirectory, "path to dotfile directory")

	if err := rootCmd.Execute(); err != nil {
		Error(err.Error())
		os.Exit(1)
	}

	config := &Config{
		DotfilePath: *dotFilePath,
		WebHook:     *webhookUrl,
		Port:        *port,
	}

	syncer := &Syncer{config, db, httpClient}

	syncHandler := NewSyncHandler(syncer, db, httpClient)

	// first time sync
	err := syncer.Sync(*dotFilePath, "Automatic")
	if err != nil {
		Error("could not perform first start-up sync: ", err.Error())
	}

	// start server asynchronously
	go func() {
		mux := http.NewServeMux()

		// register handlers
		mux.HandleFunc("/sync", syncHandler.Sync)
		Info("Server started on port", *port)
		Error(http.ListenAndServe(":"+*port, mux).Error())
	}()

	client := sse.NewClient(config.WebHook)
	Info("Listening on webhook url", *webhookUrl)

	err = client.Subscribe("message", func(msg *sse.Event) {
		data := string(msg.Data)
		if data != "{}" {
			var commit GitWebHookCommitResponse
			_ = json.Unmarshal([]byte(data), &commit)
			if commit.Event == "push" {
				Info("changes detected...")

				err := syncer.Sync(*dotFilePath, "Automatic")
				if err != nil {
					Info("error syncing on path:", *dotFilePath, err.Error())
				} else {
					t := &Commit{
						Id:   commit.Body.HeadCommit.Id,
						Time: "",
					}

					syncStash := &SyncStash{
						Commit: t,
						Type:   "Automatic",
						Time:   time.Now().UTC().Format(time.RFC3339),
					}

					_ = db.Create(syncStash)
				}
			}
		}
	})

	if err != nil {
		Error("Failed to subscribe to webhook: ", err.Error())
		os.Exit(1)
	}
}

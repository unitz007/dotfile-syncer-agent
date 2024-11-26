package main

import (
	"context"
	"encoding/json"
	"github.com/r3labs/sse/v2"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"path"
	"time"
)

func main() {
	var (
		cronJob                 = cron.New()
		db                      = InitDB()
		httpClient              = NewHttpClient()
		rootCmd                 = cobra.Command{}
		mux                     = http.NewServeMux()
		server                  = sse.New()
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
	syncHandler := NewSyncHandler(syncer, db, httpClient, server)
	client := sse.NewClient(config.WebHook)
	server.CreateStream("messages")

	Info("Listening on webhook url", *webhookUrl)

	go func() {
		_, _ = cronJob.AddFunc("@every 5s", func() {
			ctx, cancel := context.WithCancel(context.Background())
			_ = client.SubscribeWithContext(ctx, "message", func(msg *sse.Event) {
				if msg != nil {
					data := string(msg.Data)
					if data != "{}" {
						var commit GitWebHookCommitResponse
						_ = json.Unmarshal([]byte(data), &commit)
						if commit.Event == "push" {
							Info("changes detected...")
							syncer.Sync(*dotFilePath, "Automatic", nil)

							//Info("error syncing on path:", *dotFilePath, err.Error())

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
							streamBody, _ := json.Marshal(syncStash)
							server.Publish(
								"messages",
								&sse.Event{Data: streamBody})

						}
					}
				}

				go func() {
					time.Sleep(time.Second * 4)
					cancel()
				}()

			})

		})

		cronJob.Start()

	}()

	// register handlers
	mux.HandleFunc("/sync", syncHandler.Sync)
	Info("Server started on port", *port)
	Error(http.ListenAndServe(":"+*port, mux).Error())
	os.Exit(1)
}

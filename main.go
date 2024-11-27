package main

import (
	"context"
	"encoding/json"
	"fmt"
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

	// streams

	syncStatus := "sync-status"
	syncTrigger := "sync-trigger"

	server.CreateStream(syncStatus)
	server.CreateStream(syncTrigger)

	Info("Listening on webhook url", *webhookUrl)

	go func() {
		_, _ = cronJob.AddFunc("@every 5s", func() {
			ctx, cancel := context.WithCancel(context.Background())
			ch := make(chan SyncEvent)
			_ = client.SubscribeWithContext(ctx, syncTrigger, func(msg *sse.Event) {
				if msg != nil {
					data := string(msg.Data)
					if data != "{}" {
						var commit GitWebHookCommitResponse
						_ = json.Unmarshal([]byte(data), &commit)
						if commit.Event == "push" {
							Info("changes detected...")
							go syncer.Sync(*dotFilePath, "Automatic", ch)
							for x := range ch {
								if !x.Data.IsSuccess {
									msg := fmt.Sprintf("'%s': [%s]", x.Data.Step, x.Data.Error)
									Error("Sync Failed: Could not", msg)
								}
								streamBody, _ := json.Marshal(x.Data)
								server.Publish(
									syncTrigger,
									&sse.Event{Data: streamBody})
								time.Sleep(1 * time.Second)
							}
						}
					}
				}

				//time.Sleep(1 * time.Second)
				//syncStatus, err := db.Get(1)
				//if err != nil {
				//	Error(err.Error())
				//	return
				//}
				//
				//headCommit := func(c HttpClient) *Commit {
				//	remoteCommitResponse, err := httpClient.GetCommits()
				//	if err != nil {
				//		Error(err.Error())
				//		return nil
				//	}
				//
				//	commit := remoteCommitResponse[0]
				//
				//	return &Commit{
				//		Id: commit.Sha,
				//	}
				//}(httpClient)
				//
				//response := InitGitTransform(syncStatus.Commit, headCommit)
				//response.LastSyncTime = syncStatus.Time
				//response.LastSyncType = syncStatus.Type
				//streamBody, _ := json.Marshal(response)
				//server.Publish("yes", &sse.Event{Data: streamBody})

				go func() {
					time.Sleep(time.Second * 4)
					cancel()
				}()

			})
		})

		cronJob.Start()

	}()

	//go func() {
	//	client.Subscribe(syncStatus)
	//}()

	// register handlers
	mux.HandleFunc("/sync", syncHandler.Sync)
	Info("Server started on port", *port)
	Error(http.ListenAndServe(":"+*port, mux).Error())
	os.Exit(1)
}

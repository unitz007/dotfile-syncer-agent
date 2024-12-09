package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/r3labs/sse/v2"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

func main() {
	var (
		cronJob                 = cron.New()
		db                      = InitDB()
		rootCmd                 = cobra.Command{}
		mux                     = http.NewServeMux()
		sseServer               = sse.New()
		defaultDotFileDirectory = func() string {
			homeDir, err := os.UserConfigDir()
			if err != nil {
				Error("Unable to access home directory:", err.Error())
				os.Exit(1)
			}

			return path.Join(homeDir, "dotfiles")
		}()
	)

	port := rootCmd.Flags().StringP("port", "p", DefaultPort, "HTTP port to run on")
	webhookUrl := rootCmd.Flags().StringP("webhook", "w", DefaultSMEEUrl, "git webhook url")
	dotFilePath := rootCmd.Flags().StringP("dotfile-path", "d", defaultDotFileDirectory, "path to dotfile directory")

	if err := rootCmd.Execute(); err != nil {
		Error(err.Error())
		os.Exit(1)
	}

	config := &Config{
		DotfilePath: *dotFilePath,
		WebHook:     *webhookUrl,
		Port:        *port,
		GithubToken: func() string {
			gitToken, ok := os.LookupEnv("GITHUB_TOKEN")
			if !ok {
				Error("No GITHUB_TOKEN environment variable found")
				os.Exit(1)
			}

			return gitToken
		}(),
	}

	httpClient := NewHttpClient(config)
	syncer := &Syncer{config, db, httpClient}
	syncHandler := NewSyncHandler(syncer, db, httpClient, sseServer)
	sseClient := sse.NewClient(config.WebHook)
	remoteCommit := func() *Commit {
		remoteCommitResponse, err := httpClient.GetCommits()
		if err != nil {
			Error(err.Error())
			return nil
		}

		commit := remoteCommitResponse[0]

		return &Commit{
			Id: commit.Sha,
		}
	}

	syncStatusStream := sseServer.CreateStream(SyncStatusLabel)
	syncTriggerStream := sseServer.CreateStream(SyncTriggerLabel)

	Info("Listening on webhook url", *webhookUrl)
	go func() {
		_, _ = cronJob.AddFunc("@every 5s", func() {
			syncTriggerStream.Eventlog.Clear()
			ctx, cancel := context.WithCancel(context.Background())
			_ = sseClient.SubscribeWithContext(ctx, SyncStatusLabel, func(msg *sse.Event) {
				if msg != nil {
					data := string(msg.Data)
					if data != "{}" {
						var commit GitWebHookCommitResponse
						_ = json.Unmarshal([]byte(data), &commit)
						if commit.Event == "push" {
							ch := make(chan SyncEvent)
							go syncer.Sync(*dotFilePath, "Automatic", ch)
							fmt.Print("Automatic sync triggered===(0%)")
							status := "===completed"
							for x := range ch {
								fmt.Print("===(" + strconv.Itoa(x.Data.Progress) + "%)")
								if !x.Data.IsSuccess {
									msg := fmt.Sprintf("'%s': [%s]", x.Data.Step, x.Data.Error)
									status = fmt.Sprintf("===failed (%s)", msg)
								}
								streamBody, _ := json.Marshal(x.Data)
								sseServer.Publish(SyncTriggerLabel, &sse.Event{Data: streamBody})
								time.Sleep(1 * time.Second)
							}
							fmt.Printf("%s\n", status)
						}
					}
				}

				go func() {
					time.Sleep(time.Second * 4)
					cancel()
				}()
			})
		})
		_, _ = cronJob.AddFunc("@every 5s", func() {
			syncStatusStream.Eventlog.Clear()
			syncStatus, err := db.Get(1)
			if err != nil {
				Error(err.Error())
				return
			}

			remoteCommit := remoteCommit()

			response := InitGitTransform(syncStatus.Commit, remoteCommit)
			response.LastSyncTime = syncStatus.Time
			response.LastSyncType = syncStatus.Type

			v, _ := json.Marshal(response)

			// trigger auto sync if is not sync
			if !response.IsSync {
				var ch chan SyncEvent
				go syncer.Sync(*dotFilePath, "Automatic", ch)
				status := "===completed"
				for x := range ch {
					fmt.Print("===(" + strconv.Itoa(x.Data.Progress) + "%)")
					if !x.Data.IsSuccess {
						msg := fmt.Sprintf("'%s': [%s]", x.Data.Step, x.Data.Error)
						status = fmt.Sprintf("===failed (%s)", msg)
					}
				}
				fmt.Printf("%s\n", status)
			}

			sseServer.Publish(SyncStatusLabel, &sse.Event{Data: v})
			machineId, mOk := os.LookupEnv("DOTFILE_MACHINE_ID")
			brokerUrl, bOk := os.LookupEnv("DOTFILE_BROKER_URL")

			if mOk && bOk {
				go func() {
					request, _ := http.NewRequest("POST", brokerUrl+"/sync-status/"+machineId+"/notify", bytes.NewBuffer(v))
					request.Header.Set("Content-Type", "application/json")
					response, err := http.DefaultClient.Do(request)
					if err != nil {
						Error("Failed to send notification to broker:", err.Error())
						return
					}

					if response.StatusCode != 200 {
						Error("Failed to send notification to broker:", response.Status)
					}

				}()
			}
		})

		cronJob.Start()

	}()

	// register handlers
	mux.HandleFunc("/sync", syncHandler.Sync)
	Info("Server started on port", *port)
	Error(http.ListenAndServe(":"+*port, mux).Error())
	os.Exit(1)
}

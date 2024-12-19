package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/r3labs/sse/v2"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

func main() {
	var (
		cronJob           = cron.New()
		rootCmd           = cobra.Command{}
		mux               = http.NewServeMux()
		sseServer         = sse.New()
		brokerNotifier    = NewBrokerNotifier()
		syncStatusStream  = sseServer.CreateStream(SyncStatusLabel)
		syncTriggerStream = sseServer.CreateStream(SyncTriggerLabel)
		port              = rootCmd.Flags().StringP("port", "p", DefaultPort, "HTTP port to run on")
		webhookUrl        = rootCmd.Flags().StringP("webhook", "w", "", "git webhook url")
		dotFilePath       = rootCmd.Flags().StringP("dotfile-path", "d", "", "path to dotfile directory")
		configDir         = rootCmd.Flags().StringP("config-dir", "c", "", "path to config directory")
		gitUrl            = rootCmd.Flags().StringP("git-url", "g", "", "github api url")
	)

	if err := rootCmd.Execute(); err != nil {
		Error(err.Error())
		return
	}

	config, err := InitializeConfigurations(*dotFilePath, *webhookUrl, *port, *configDir, *gitUrl)
	if err != nil {
		Error(err.Error())
		return
	}

	persistence, err := InitializePersistence(config)
	if err != nil {
		Error(err.Error())
		return
	}

	git := &Git{config}
	syncer := NewSyncer(config, persistence, brokerNotifier)
	syncHandler := NewSyncHandler(syncer, git, sseServer)
	sseClient := sse.NewClient(config.WebHook)
	brokerNotifier.RegisterStream()

	Info("Listening on webhook url", *webhookUrl)
	go func() {
		for {
			ctx, cancel := context.WithCancel(context.Background())
			_ = sseClient.SubscribeRawWithContext(ctx, func(msg *sse.Event) {
				if msg != nil {
					data := string(msg.Data)
					if data != "{}" {
						var commit GitWebHookCommitResponse
						_ = json.Unmarshal([]byte(data), &commit)
						go syncer.Sync()
						syncer.Consume(ConsoleSyncConsumer)
					}
				}

				go func() {
					time.Sleep(4 * time.Second)
					cancel()
				}()

				// check status
				//if len(syncStatusStream.Eventlog) != 0 {
				//	event := syncStatusStream.Eventlog[len(syncStatusStream.Eventlog)-1]
				//	syncStatusStream.Eventlog = []*sse.Event{event}
				//}
				//
				localCommit, err := git.LocalCommit()
				if err != nil {
					Error(err.Error())
					return
				}

				remoteCommit, err := git.RemoteCommit()
				if err != nil {
					Error(err.Error())
					return
				}

				response := InitGitTransform(localCommit, remoteCommit)
				v, _ := json.Marshal(response)

				// first event
				if len(syncStatusStream.Eventlog) == 0 {
					sseServer.Publish(SyncStatusLabel, &sse.Event{Data: v})
					brokerNotifier.SyncStatus(response)
					if !response.IsSync {
						go func() {
							go syncer.Sync()
							syncer.Consume(ConsoleSyncConsumer)
							syncTriggerStream.Eventlog.Clear()
						}()
					}
				}

				//
				//	event := syncStatusStream.Eventlog[len(syncStatusStream.Eventlog)-1]
				//
				//	var prevResponse SyncStatusResponse
				//	_ = json.Unmarshal(event.Data, &prevResponse)
				//
				//	if response.IsSync != prevResponse.IsSync {
				//		sseServer.Publish(SyncStatusLabel, &sse.Event{Data: v})
				//		brokerNotifier.SyncStatus(response)
				//	}
				//}
			})

			time.Sleep(5 * time.Second)
		}
	}()

	cronJob.Start()

	// register handlers
	mux.HandleFunc("/sync", syncHandler.Sync)
	Info("Server started on port", *port)
	Error(http.ListenAndServe(":"+*port, mux).Error())
	os.Exit(1)
}

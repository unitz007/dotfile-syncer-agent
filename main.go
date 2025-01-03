package main

import (
	"context"
	"encoding/json"
	"github.com/r3labs/sse/v2"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	var (
		cronJob        = cron.New()
		rootCmd        = cobra.Command{}
		mux            = http.NewServeMux()
		sseServer      = sse.New()
		brokerNotifier = NewBrokerNotifier()
		//syncStatusStream  = sseServer.CreateStream(SyncStatusLabel)
		//syncTriggerStream = sseServer.CreateStream(SyncTriggerLabel)
		port          = rootCmd.Flags().StringP("port", "p", DefaultPort, "HTTP port to run on")
		webhookUrl    = rootCmd.Flags().StringP("webhook", "w", "", "git webhook url")
		dotFilePath   = rootCmd.Flags().StringP("dotfile-path", "d", "", "path to dotfile directory")
		configDir     = rootCmd.Flags().StringP("config-dir", "c", "", "path to config directory")
		gitUrl        = rootCmd.Flags().StringP("git-url", "g", "", "github api url")
		gitApiBaseUrl = rootCmd.Flags().StringP("git-api-base-url", "b", "https://api.github.com", "github api url")
	)

	if err := rootCmd.Execute(); err != nil {
		Error(err.Error())
		return
	}

	config, err := InitializeConfigurations(*dotFilePath, *webhookUrl, *port, *configDir, *gitUrl, *gitApiBaseUrl)
	if err != nil {
		Error(err.Error())
		return
	}

	git := &Git{config}
	mutex := &sync.Mutex{}
	syncer := NewCustomerSyncer(config, brokerNotifier, mutex)
	syncHandler := NewSyncHandler(&syncer, git, sseServer)
	sseClient := sse.NewClient(config.WebHook)
	brokerNotifier.RegisterStream()

	Infoln("Listening on webhook url", *webhookUrl)
	go func() {
		_, _ = cronJob.AddFunc("@every 5s", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = sseClient.SubscribeRawWithContext(ctx, func(msg *sse.Event) {
				if msg != nil {
					data := string(msg.Data)
					var commit *GitWebHookCommitResponse
					err = json.Unmarshal([]byte(data), &commit)
					if err != nil {
						return
					}

					commitRef := commit.Ref
					if commitRef == "" {
						return
					}

					branch := strings.Split(commitRef, "/")[2]
					if branch == "main" { // only triggers sync on push to main branch
						ch := make(chan SyncEvent)
						go syncer.Sync(ch)
						syncer.Consume(ch, ConsoleSyncConsumer)
					}
				}
			})

			go func() {
				time.Sleep(4 * time.Second)
				cancel()
			}()
		})

		cronJob.Start()
	}()

	// register handlers
	mux.HandleFunc("/sync", syncHandler.Sync)
	Infoln("Server started on port", *port)
	Error(http.ListenAndServe(":"+*port, mux).Error())
	os.Exit(1)
}

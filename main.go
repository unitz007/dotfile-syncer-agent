package main

import (
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
		port           = rootCmd.Flags().StringP("port", "p", DefaultPort, "HTTP port to run on")
		webhookUrl     = rootCmd.Flags().StringP("webhook", "w", "", "git webhook url")
		dotFilePath    = rootCmd.Flags().StringP("dotfile-path", "d", "", "path to dotfile directory")
		configDir      = rootCmd.Flags().StringP("config-dir", "c", "", "path to config directory")
		gitUrl         = rootCmd.Flags().StringP("git-url", "g", "", "github api url")
		gitApiBaseUrl  = rootCmd.Flags().StringP("git-api-base-url", "b", "https://api.github.com", "github api url")
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
	syncer := NewCustomerSyncer(config, brokerNotifier, mutex, git)
	syncHandler := NewSyncHandler(&syncer, git, sseServer)
	sseClient := sse.NewClient(config.WebHook)
	brokerNotifier.RegisterStream()
	var lastEventChange time.Time

	var sseSub = func(sseClient *sse.Client, syncer Syncer) error {
		return sseClient.SubscribeRaw(func(msg *sse.Event) {
			if msg != nil {
				lastEventChange = time.Now()
				data := string(msg.Data)
				var commit *GitWebHookCommitResponse
				err := json.Unmarshal([]byte(data), &commit)
				if err != nil {
					return
				}

				commitRef := commit.Ref
				if commitRef == "" {
					return
				}

				branch := strings.Split(commitRef, "/")[2]
				if branch == "main" { // only triggers sync on push to main branch
					syncer.Sync(ConsoleSyncConsumer)
				}
			}
		})
	}

	Infoln("Listening on webhook url", *webhookUrl)
	go func() {
		var delta int
		_, _ = cronJob.AddFunc("@every 5s", func() {
			if delta == lastEventChange.Second() {
				_ = sseSub(sseClient, syncer)
			} else {
				delta = lastEventChange.Second()
			}
		})

		cronJob.Start()

		_ = sseSub(sseClient, syncer)
	}()

	// register handlers
	mux.HandleFunc("/sync", syncHandler.Sync)
	Infoln("Server started on port", *port)
	Error(http.ListenAndServe(":"+*port, mux).Error())
	os.Exit(1)
}

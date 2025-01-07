package main

import (
	"github.com/r3labs/sse/v2"
	"github.com/robfig/cron/v3"
	"time"

	//"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"sync"
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
	//sseClient := sse.NewClient(config.WebHook)
	brokerNotifier.RegisterStream()
	var lastEventChange time.Time
	var httpClient http.Client

	timeOut := 2 * time.Second

	var sseSub = func(syncer Syncer) error {
		httpClient = http.Client{
			Timeout: timeOut,
			Transport: &http.Transport{
				IdleConnTimeout: timeOut,
			},
		}

		res, err := httpClient.Get(config.WebHook)
		defer func() {

		}()
		if err != nil {
			return err
		}

		err = res.Write(SseClient{syncer})

		if err != nil {
			return err
		}

		return nil
	}

	Infoln("Listening on webhook url", *webhookUrl)
	go func() {
		var delta int
		_, _ = cronJob.AddFunc("@every 5s", func() {
			if delta == lastEventChange.Second() {
				_ = sseSub(syncer)

			} else {
				delta = lastEventChange.Second()
			}
		})

		cronJob.Start()

		err = sseSub(syncer)
		if err != nil {
			Error(err.Error())
			return
		}
	}()

	// register handlers
	mux.HandleFunc("/sync", syncHandler.Sync)
	Infoln("Server started on port", *port)
	Error(http.ListenAndServe(":"+*port, mux).Error())
	os.Exit(1)
}

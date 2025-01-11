package main

import (
	"context"
	"github.com/r3labs/sse/v2"
	"time"

	//"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"sync"
)

func main() {
	var (
		//cronJob        = cron.New()
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
	brokerNotifier.RegisterStream()
	lock := false

	req1Deadline := 5 * time.Second
	req2Deadline := 10 * time.Second

	go func() {

		responsePool := make([]*http.Response, 2)

		requestPool := func() []*http.Request {

			var c []*http.Request

			ctx1, _ := context.WithDeadline(context.Background(), time.Now().Add(req1Deadline))
			req1, _ := http.NewRequestWithContext(ctx1, http.MethodGet, *webhookUrl, nil)

			ctx2, _ := context.WithDeadline(context.Background(), time.Now().Add(req2Deadline))
			req2, _ := http.NewRequestWithContext(ctx2, http.MethodGet, *webhookUrl, nil)

			return append(c, req1, req2)
		}()

		go func() {
			t := time.NewTicker(2 * time.Second)
			for {
				select {
				case <-t.C:
					ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(req1Deadline))
					req, _ := http.NewRequestWithContext(ctx, http.MethodGet, *webhookUrl, nil)
					requestPool[0] = req

					ctx, _ = context.WithDeadline(context.Background(), time.Now().Add(req2Deadline))
					req, _ = http.NewRequestWithContext(ctx, http.MethodGet, *webhookUrl, nil)
					requestPool[1] = req
				}
			}
		}()

		func(httpClient *http.Client, syncer Syncer) {
			ticker := time.NewTicker(2 * time.Second)
			done := make(chan bool)
			for {
				select {
				case <-done:
					ticker.Stop()
				case <-ticker.C:
					for i, request := range requestPool {
						go func() {
							resp, err := httpClient.Do(request)
							if err == nil {
								responsePool[i] = resp
							}
							for _, response := range responsePool {
								if response != nil {
									if lock == false {
										lock = true
										err = response.Write(SseClient{i, syncer})
										lock = false
									}
								}
							}
						}()
					}
				}
			}
		}(&http.Client{}, syncer)
	}()

	Infoln("Listening on webhook url", *webhookUrl)

	// register handlers
	mux.HandleFunc("/sync", syncHandler.Sync)
	Infoln("Server started on port", *port)
	Error(http.ListenAndServe(":"+*port, mux).Error())
	os.Exit(1)
}

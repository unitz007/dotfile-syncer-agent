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
	//lock := false
	httpClient := &http.Client{}
	sseClient := &SseClient{Syncer: syncer}

	req1Deadline := 5 * time.Second
	//req2Deadline := 4 * time.Second

	var resp *struct {
		res *http.Response
		val int
	}

	go func() {
		go func() {
			t := time.NewTicker(5 * time.Second)
			for {
				select {
				case <-t.C:
					ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(req1Deadline))
					req, _ := http.NewRequestWithContext(ctx, http.MethodGet, *webhookUrl, nil)

					response, err := httpClient.Do(req)
					if err != nil {
						Error(err.Error())
						return
					}

					resp = &struct {
						res *http.Response
						val int
					}{response, 1}
				}
			}
		}()

		go func() {
			t := time.NewTicker(5 * time.Second)
			for {
				select {
				case <-t.C:
					ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(req1Deadline))
					req, _ := http.NewRequestWithContext(ctx, http.MethodGet, *webhookUrl, nil)

					response, err := httpClient.Do(req)
					if err != nil {
						Error(err.Error())
						return
					}

					resp = &struct {
						res *http.Response
						val int
					}{response, 2}
				}
			}
		}()

		go func() {
			t := time.NewTicker(1 * time.Second)
			for {
				select {
				case <-t.C:
					if resp != nil {
						sseClient.req = resp.val
						_ = resp.res.Write(sseClient)
					}
				}
			}
		}()

	}()

	Infoln("Listening on webhook url", *webhookUrl)

	// register handlers
	mux.HandleFunc("/sync", syncHandler.Sync)
	Infoln("Server started on port", *port)
	Error(http.ListenAndServe(":"+*port, mux).Error())
	os.Exit(1)
}

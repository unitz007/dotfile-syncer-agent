package main

import (
	"context"
	"github.com/r3labs/sse/v2"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	var (
		rootCmd       = cobra.Command{}
		mux           = http.NewServeMux()
		sseServer     = sse.New()
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
	brokerNotifier := NewBrokerNotifier(git)
	mutex := &sync.Mutex{}
	syncer := NewCustomerSyncer(config, brokerNotifier, mutex, git)
	syncHandler := NewSyncHandler(&syncer, git, sseServer)
	brokerNotifier.RegisterStream()
	httpClient := &http.Client{}
	sseClient := &SseClient{Syncer: syncer}
	deadline := 5 * time.Second

	var resp *http.Response
	go func() {
		go func() {
			t := time.NewTicker(5 * time.Second)
			for {
				select {
				case <-t.C:
					ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(deadline))
					req, _ := http.NewRequestWithContext(ctx, http.MethodGet, *webhookUrl, nil)
					response, err := httpClient.Do(req)
					if err != nil {
						Error(err.Error())
					} else {
						resp = response
					}
				}
			}
		}()

		go func() {
			t := time.NewTicker(5 * time.Second)
			for {
				select {
				case <-t.C:
					ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(deadline))
					req, _ := http.NewRequestWithContext(ctx, http.MethodGet, *webhookUrl, nil)
					response, err := httpClient.Do(req)
					if err != nil {
						Error(err.Error())
					} else {
						resp = response
					}
				}
			}
		}()

		go func() {
			t := time.NewTicker(1 * time.Second)
			for {
				select {
				case <-t.C:
					if resp != nil {
						_ = resp.Write(sseClient)
						_ = resp.Body.Close()
						resp = nil
					}
				}
			}
		}()
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for {
			select {
			case <-ticker.C:
				localCommit, _ := git.LocalCommit()
				remoteCommit, _ := git.RemoteCommit()
				isSync := git.IsSync(localCommit, remoteCommit)
				if !isSync {
					Infoln("Triggering Automatic Sync")
					syncer.Sync(ConsoleSyncConsumer)
				}
			}
		}
	}()

	Infoln("Listening on webhook url", *webhookUrl)

	// register handlers
	mux.HandleFunc("/sync", syncHandler.Sync)
	Infoln("Server started on port", *port)
	Error(http.ListenAndServe(":"+*port, mux).Error())
	os.Exit(1)
}

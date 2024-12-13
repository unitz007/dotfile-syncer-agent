package main

import (
	"context"
	"sync"

	//"context"
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
		brokerNotifier          = NewBrokerNotifier(os.Getenv("DOTFILE_MACHINE_ID"), os.Getenv("DOTFILE_BROKER_URL"))
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
	webhookUrl := rootCmd.Flags().StringP("webhook", "w", DefaultWebHookUrl, "git webhook url")
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

	git := &Git{config}
	syncMutex := &sync.Mutex{}
	syncer := &Syncer{config, db, brokerNotifier, syncMutex}
	syncHandler := NewSyncHandler(syncer, git, sseServer)
	sseClient := sse.NewClient(config.WebHook)

	//syncStatusStream := sseServer.CreateStream(SyncStatusLabel)
	syncTriggerStream := sseServer.CreateStream(SyncTriggerLabel)
	brokerNotifier.RegisterStream()

	notify := func(git *Git) {
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
		brokerNotifier.SyncStatus(response)
	}

	Info("Listening on webhook url", *webhookUrl)
	go func() {
		_, _ = cronJob.AddFunc("@every 5s", func() {
			ctx, cancel := context.WithCancel(context.Background())
			_ = sseClient.SubscribeRawWithContext(ctx, func(msg *sse.Event) {
				if msg != nil {
					data := string(msg.Data)
					if data != "{}" {
						go notify(git)
						var commit GitWebHookCommitResponse
						_ = json.Unmarshal([]byte(data), &commit)
						ch := make(chan SyncEvent)
						go syncer.Sync(*dotFilePath, AutomaticSync, ch)
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
						go notify(git)
						syncTriggerStream.Eventlog.Clear()
					}
				}

				go func() {
					time.Sleep(4 * time.Second)
					cancel()
				}()
			})
		})
	}()

	cronJob.Start()

	// register handlers
	mux.HandleFunc("/sync", syncHandler.Sync)
	Info("Server started on port", *port)
	Error(http.ListenAndServe(":"+*port, mux).Error())
	os.Exit(1)
}

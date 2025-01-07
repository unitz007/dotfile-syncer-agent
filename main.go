package main

import (
	"github.com/r3labs/sse/v2"
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
	//sseClient := sse.NewClient(config.WebHook)
	brokerNotifier.RegisterStream()
	//var lastEventChange time.Time

	var sseSub = func(syncer Syncer) error {
		res, err := http.Get(config.WebHook)
		if err != nil {
			return err
		}

		err = res.Write(SseClient{syncer})

		if err != nil {
			return err
		}
		//_, after, found := bytes.Cut(p, []byte("data:"))
		//if found {
		//data := w.val
		//fmt.Println(data)
		//var commit *GitWebHookCommitResponse
		//err = json.Unmarshal([]byte(data), &commit)
		//if err != nil {
		//	fmt.Println(err)
		//	return err
		//}
		//
		//commitRef := commit.Ref
		//if commitRef == "" {
		//	return errors.New("empty commit ref")
		//}
		//
		//branch := strings.Split(commitRef, "/")[2]
		//if branch == "main" { // only triggers sync on push to main branch
		//	syncer.Sync(ConsoleSyncConsumer)
		//}
		//}

		//return sseClient.SubscribeRaw(func(msg *sse.Event) {
		//	if msg != nil {
		//		lastEventChange = time.Now()
		//		data := string(msg.Data)
		//		var commit *GitWebHookCommitResponse
		//		err := json.Unmarshal([]byte(data), &commit)
		//		if err != nil {
		//			return
		//		}
		//
		//		commitRef := commit.Ref
		//		if commitRef == "" {
		//			return
		//		}
		//
		//		branch := strings.Split(commitRef, "/")[2]
		//		if branch == "main" { // only triggers sync on push to main branch
		//			syncer.Sync(ConsoleSyncConsumer)
		//		}
		//	}
		//})
		return nil
	}

	Infoln("Listening on webhook url", *webhookUrl)
	go func() {
		//var delta int
		//_, _ = cronJob.AddFunc("@every 5s", func() {
		//	if delta == lastEventChange.Second() {
		//		_ = sseSub(syncer)
		//
		//	} else {
		//		delta = lastEventChange.Second()
		//	}
		//})
		//
		//cronJob.Start()

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

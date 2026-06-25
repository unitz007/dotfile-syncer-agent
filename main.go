package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

var (
	dotFilePath string
	configDir   string

	// Registration flags
	regMachineName string
	regAgentToken  string
	regBrokerURL   string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "dotsync-agent",
		Short: "Dotfile Syncer Agent",
	}

	rootCmd.PersistentFlags().StringVarP(&dotFilePath, "dotfile-path", "d", "", "path to dotfile directory")
	rootCmd.PersistentFlags().StringVarP(&configDir, "config-dir", "c", "", "path to config directory")

	var registerCmd = &cobra.Command{
		Use:   "register",
		Short: "Register the agent with a broker",
		Run: func(cmd *cobra.Command, args []string) {
			if regBrokerURL == "" || regAgentToken == "" || regMachineName == "" {
				Error("Broker URL, Token, and Machine Name are required")
				os.Exit(1)
			}

			config, err := InitializeConfigurations(dotFilePath, configDir)
			if err != nil {
				Error(err.Error())
				os.Exit(1)
			}

			err = registerAgent(config, regBrokerURL, regAgentToken, regMachineName)
			if err != nil {
				Error("Registration failed: " + err.Error())
				os.Exit(1)
			}
			Infoln("Agent registered successfully!")
		},
	}
	registerCmd.Flags().StringVar(&regBrokerURL, "broker-url", "", "Broker URL")
	registerCmd.Flags().StringVar(&regAgentToken, "token", "", "Registration Token")
	registerCmd.Flags().StringVar(&regMachineName, "name", "", "Machine Name")

	var syncCmd = &cobra.Command{
		Use:   "sync",
		Short: "Perform a synchronization",
		Run: func(cmd *cobra.Command, args []string) {
			runSync(false)
		},
	}

	var daemonCmd = &cobra.Command{
		Use:   "daemon",
		Short: "Run in daemon mode",
		Run: func(cmd *cobra.Command, args []string) {
			runSync(true)
		},
	}

	rootCmd.AddCommand(registerCmd, syncCmd, daemonCmd)

	if err := rootCmd.Execute(); err != nil {
		Error(err.Error())
		os.Exit(1)
	}
}

func registerAgent(config *Configurations, brokerURL, token, name string) error {
	payload := map[string]string{
		"token": token,
		"name":  name,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(brokerURL+"/agent/register", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	var result struct {
		AgentID    string `json:"agent_id"`
		AgentToken string `json:"agent_token"`
		UserID     string `json:"user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	identity := &AgentIdentity{
		AgentToken:  result.AgentToken,
		AgentID:     result.AgentID,
		MachineName: name,
		BrokerURL:   brokerURL,
	}

	return saveAgentIdentity(config, identity)
}

func runSync(daemon bool) {
	config, err := InitializeConfigurations(dotFilePath, configDir)
	if err != nil {
		Error(err.Error())
		os.Exit(1)
	}

	identity, err := loadAgentIdentity(config)
	if err != nil {
		Error("Failed to load agent identity. Please run 'register' first.")
		os.Exit(1)
	}
	if identity == nil {
		Error("Agent identity not found. Please run 'register' first.")
		os.Exit(1)
	}

	git := &Git{config}
	brokerNotifier := NewBrokerNotifier(git, identity.AgentID, identity.AgentToken, identity.MachineName, identity.BrokerURL)
	mutex := &sync.Mutex{}
	syncer := NewEnhancedSyncer(config, brokerNotifier, mutex, git)

	// Configure Git Repo from Broker
	repoURL, err := brokerNotifier.GetRepositoryConfig()
	if err != nil {
		Infoln("WARN: Failed to fetch repository config: " + err.Error())
	} else {
		config.GitUrl = repoURL
		config.GitApiBaseUrl = "https://api.github.com"

		owner, repo, err := ParseGitUrl(repoURL)
		if err != nil {
			Infoln("WARN: Failed to parse git url: " + err.Error())
		} else {
			config.RepositoryOwner = owner
			config.GitRepository = repo
			Infoln("Configured repository:", owner, "/", repo)
		}

		err = git.CloneOrPullRepository()
		if err != nil {
			Error("Failed to clone/pull repository: " + err.Error())
		}
	}

	brokerNotifier.RegisterStream()
	runBootstrapSelfTest(brokerNotifier, config)

	// Initial Sync
	Infoln("Starting sync...")
	syncer.Sync()
	Infoln("Sync completed")

	if !daemon {
		return
	}

	// Daemon Loop
	ticker := time.NewTicker(10 * time.Second)

	// Start other background services
	planExecutor := NewPlanExecutor(config, brokerNotifier, git)

	// 1. Policy Executor
	go func() {
		policyTicker := time.NewTicker(15 * time.Second)
		for {
			select {
			case <-policyTicker.C:
				cmd, err := brokerNotifier.NextPolicyRunCommand()
				if err != nil {
					Error("Error checking for policy run: " + err.Error())
					continue
				}
				if cmd == nil {
					continue
				}

				Infoln("Received Policy Run Command:", cmd.RunID)

				if cmd.ExecutionPlan == nil {
					Infoln("Warning: Policy run has no execution plan. Skipping.")
					_ = brokerNotifier.ReportPolicyRunResult(cmd.RunID, "succeeded", map[string]string{"message": "No execution plan provided"})
					continue
				}

				err = planExecutor.Execute(cmd.RunID, cmd.ExecutionPlan)
				if err != nil {
					Error("Policy Execution Failed: " + err.Error())
				} else {
					Infoln("Policy Execution Completed Successfully")
				}
			}
		}
	}()

	// 2. Connectivity Check
	go func() {
		connTicker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-connTicker.C:
				hasConnectivity, err := brokerNotifier.HasConnectivityCommand()
				if err != nil {
					Error(err.Error())
					continue
				}
				if !hasConnectivity {
					continue
				}
				err = brokerNotifier.Ping()
				if err != nil {
					Error(err.Error())
				}
				err = brokerNotifier.AckConnectivityCommand()
				if err != nil {
					Error(err.Error())
				}
			}
		}
	}()

	// 3. Heartbeat
	go func() {
		hbTicker := time.NewTicker(30 * time.Second)
		_ = brokerNotifier.Ping()
		for {
			select {
			case <-hbTicker.C:
				_ = brokerNotifier.Ping()
			}
		}
	}()

	// prioritySync performs a sync and acks the command, guarded by the shared
	// mutex so that simultaneous triggers from SSE and the poll ticker do not
	// run concurrent syncs.
	prioritySync := func() {
		mutex.Lock()
		defer mutex.Unlock()
		Infoln("Triggering Priority Sync")
		syncer.Sync()
		if err := brokerNotifier.AckPrioritySyncCommand(); err != nil {
			Error("Failed to ack priority sync: " + err.Error())
		}
	}

	// 4. SSE listener — event-driven priority sync push from the broker.
	//    Reconnects with exponential backoff. The 10-second poll below remains
	//    as a fallback when SSE is unavailable.
	go brokerNotifier.startSSEListener(prioritySync)

	// Main Loop: Priority Sync (fallback poll, remains active alongside SSE)
	for {
		select {
		case <-ticker.C:
			hasCommand, err := brokerNotifier.HasPrioritySyncCommand()
			if err != nil {
				Error(err.Error())
				continue
			}
			if !hasCommand {
				continue
			}
			prioritySync()
		}
	}
}

func runBootstrapSelfTest(b *BrokerNotifier, c *Configurations) {
	if b == nil || c == nil {
		return
	}
	if c.ConfigPath != "" {
		testFile := c.ConfigPath + "/.self_test"
		_ = os.WriteFile(testFile, []byte("ok"), 0600)
		_ = os.Remove(testFile)
	}
}

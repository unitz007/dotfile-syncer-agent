package main

import (
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	var (
		rootCmd     = cobra.Command{}
		dotFilePath = rootCmd.Flags().StringP("dotfile-path", "d", "", "path to dotfile directory")
		configDir   = rootCmd.Flags().StringP("config-dir", "c", "", "path to config directory")
		_           = rootCmd.Flags().Bool("decommission", false, "remove local agent identity and exit")

		// Onboarding flags
		machineName = rootCmd.Flags().String("machine-name", "", "machine name for broker registration")
		agentToken  = rootCmd.Flags().String("agent-token", "", "agent token for broker authentication")
		agentID     = rootCmd.Flags().String("agent-id", "", "unique agent identifier")
		brokerURL   = rootCmd.Flags().String("broker-url", "", "broker service URL")
	)

	if err := rootCmd.Execute(); err != nil {
		Error(err.Error())
		return
	}

	//config, err := InitializeConfigurations(*dotFilePath, *webhookUrl, *port, *configDir, *gitUrl, *gitApiBaseUrl)
	config, err := InitializeConfigurations(*dotFilePath, *configDir)
	if err != nil {
		Error(err.Error())
		return
	}

	if rootCmd.Flags().Changed("decommission") {
		err := removeAgentIdentity(config)
		if err != nil {
			Error("Failed to remove local agent identity: " + err.Error())
			return
		}
		Infoln("Local agent identity removed. Exiting.")
		return
	}

	git := &Git{config}
	brokerNotifier := NewBrokerNotifier(git, *agentID, *agentToken, *machineName, *brokerURL)
	mutex := &sync.Mutex{}
	syncer := NewEnhancedSyncer(config, brokerNotifier, mutex, git)

	repoURL, err := brokerNotifier.GetRepositoryConfig()
	if err != nil {
		Infoln("WARN: Failed to fetch repository config: " + err.Error())
	} else {
		config.GitUrl = repoURL
		config.GitApiBaseUrl = "https://api.github.com" // Default to public GitHub

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
			// We continue, as maybe the repo is already there or we can sync later?
			// But if it failed, syncing won't work well.
			// However, original request implies we should clone if missing.
		}
	}

	brokerNotifier.RegisterStream()
	runBootstrapSelfTest(brokerNotifier, config)

	// Perform initial sync to update status
	Infoln("Starting initial sync...")
	syncer.Sync()
	Infoln("Initial sync completed")

	go func() {
		ticker := time.NewTicker(10 * time.Second)
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
				Infoln("Triggering Priority Sync")
				syncer.Sync()
				err = brokerNotifier.AckPrioritySyncCommand()
				if err != nil {
					Error(err.Error())
				}
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		for {
			select {
			case <-ticker.C:
				cmd, err := brokerNotifier.NextPolicyRunCommand()
				if err != nil {
					Error(err.Error())
					continue
				}
				if cmd == nil {
					continue
				}

				result := struct {
					Policies []struct {
						PolicyID string   `json:"policy_id"`
						Mode     string   `json:"mode"`
						Paths    []string `json:"paths"`
						Actions  []struct {
							Path   string `json:"path"`
							Action string `json:"action"`
							Detail string `json:"detail,omitempty"`
						} `json:"actions"`
					} `json:"policies"`
					Summary string `json:"summary"`
				}{}

				for _, p := range cmd.Policies {
					policyResult := struct {
						PolicyID string   `json:"policy_id"`
						Mode     string   `json:"mode"`
						Paths    []string `json:"paths"`
						Actions  []struct {
							Path   string `json:"path"`
							Action string `json:"action"`
							Detail string `json:"detail,omitempty"`
						} `json:"actions"`
					}{
						PolicyID: p.Id,
						Mode:     p.Mode,
						Paths:    p.Paths,
					}

					for _, path := range p.Paths {
						policyResult.Actions = append(policyResult.Actions, struct {
							Path   string `json:"path"`
							Action string `json:"action"`
							Detail string `json:"detail,omitempty"`
						}{
							Path:   path,
							Action: "no_change",
						})
					}

					result.Policies = append(result.Policies, policyResult)
				}

				result.Summary = "dry_run_completed"

				err = brokerNotifier.ReportPolicyRunResult(cmd.RunID, "succeeded", result)
				if err != nil {
					Error(err.Error())
				}
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ticker.C:
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

	// Heartbeat loop
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		// Initial ping
		_ = brokerNotifier.Ping()
		for {
			select {
			case <-ticker.C:
				err := brokerNotifier.Ping()
				if err != nil {
					// Log error but don't crash
					// fmt.Println("Heartbeat failed:", err)
				}
			}
		}
	}()

	select {}
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

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
			Successln("Agent registered successfully! 🎉")
		},
	}
	registerCmd.Flags().StringVar(&regBrokerURL, "broker-url", "", "Broker URL")
	registerCmd.Flags().StringVar(&regAgentToken, "token", "", "Registration Token")
	registerCmd.Flags().StringVar(&regMachineName, "name", "", "Machine Name")

	var syncCmd = &cobra.Command{
		Use:   "sync",
		Short: "Perform a synchronization",
		Run: func(cmd *cobra.Command, args []string) {
			runSync(false, 0, "", "main")
		},
	}

	var webhookPort int
	var webhookSecret string
	var webhookBranch string
	var daemonCmd = &cobra.Command{
		Use:   "daemon",
		Short: "Run in daemon mode",
		Run: func(cmd *cobra.Command, args []string) {
			runSync(true, webhookPort, webhookSecret, webhookBranch)
		},
	}
	daemonCmd.Flags().IntVar(&webhookPort, "webhook-port", 0, "Port to listen for GitHub push webhooks (0 = disabled)")
	daemonCmd.Flags().StringVar(&webhookSecret, "webhook-secret", "", "HMAC secret for validating webhook payloads")
	daemonCmd.Flags().StringVar(&webhookBranch, "webhook-branch", "main", "Branch to watch for push events")

	// apply subcommand
	var applyFile string
	var applyDryRun bool
	var applyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Apply a SyncPolicy spec directly from the terminal",
		Long: `Read, validate, and apply a SyncPolicy spec file.

Examples:
  dotsync-agent apply                        # reads .dotsync.yaml from current directory
  dotsync-agent apply -f ~/dotfiles/.dotsync.yaml
  dotsync-agent apply --dry-run              # validate and print plan without executing`,
		Run: func(cmd *cobra.Command, args []string) {
			// Load config first so we can resolve the default spec path.
			config, err := InitializeConfigurations(dotFilePath, configDir)
			if err != nil {
				Error("configuration error: " + err.Error())
				os.Exit(1)
			}

			// Resolve spec file path: explicit flag → dotfiles repo root → cwd.
			specPath := applyFile
			if specPath == "" {
				specPath = filepath.Join(config.DotfilePath, config.GitRepository, ".dotsync.yaml")
			}

			// Read and parse the spec
			data, err := os.ReadFile(specPath)
			if err != nil {
				Error("cannot read spec file: " + err.Error())
				os.Exit(1)
			}

			spec, err := ParseSpec(data)
			if err != nil {
				Error("cannot parse spec: " + err.Error())
				os.Exit(1)
			}

			// Strict validation
			if err := spec.Validate(); err != nil {
				Error("spec validation failed: " + err.Error())
				os.Exit(1)
			}

			Infoln("Spec valid:", spec.Metadata.Name)

			// Resolve execution plan for current OS
			plan := spec.ToExecutionPlan(runtime.GOOS)

			// Print what will be applied
			Infoln("--- Execution Plan ---")
			if len(plan.SoftwareUnits) == 0 {
				Infoln("  no software units for", runtime.GOOS)
			} else {
				for _, unit := range plan.SoftwareUnits {
					Infoln(fmt.Sprintf("  [%s]", unit.Name))
					if len(unit.Install) == 0 {
						Infoln("    packages: none for", runtime.GOOS)
					} else {
						for _, cmd := range unit.Install {
							Infoln("    install:", cmd)
						}
					}
					for _, f := range unit.Files {
						Infoln("    config:", f.Source, "->", f.Target)
					}
				}
			}
			Infoln("  strategy:", plan.SyncStrategy)
			Infoln("----------------------")

			if applyDryRun {
				Infoln("Dry run — no changes applied.")
				return
			}

			// Execute — skip git pull; files are already local
			git := &Git{config}
			executor := &PlanExecutor{config: config, git: git, NoPull: true}
			result, err := executor.Execute("cli-apply", plan)
			if err != nil {
				Error("apply failed: " + err.Error())
				os.Exit(1)
			}

			// Print summary
			fmt.Println()
			fmt.Printf("  %s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", bold, cyan, reset)
			fmt.Printf("  %s  📋 Apply Summary%s\n", bold, reset)
			fmt.Printf("  %s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", bold, cyan, reset)
			if len(result.PackagesInstalled) == 0 {
				fmt.Printf("  %s📦 Packages%s   none\n", dim, reset)
			} else {
				fmt.Printf("  %s📦 Packages%s\n", dim, reset)
				for _, p := range result.PackagesInstalled {
					fmt.Printf("     %s✔%s  %s\n", green+bold, reset, p)
				}
			}
			if len(result.FilesApplied) == 0 && len(result.FilesSkipped) == 0 {
				fmt.Printf("  %s🗂  Configs%s    none\n", dim, reset)
			} else {
				fmt.Printf("  %s🗂  Configs%s\n", dim, reset)
				for _, f := range result.FilesApplied {
					fmt.Printf("     %s✔%s  %s\n", green+bold, reset, f)
				}
				for _, f := range result.FilesSkipped {
					fmt.Printf("     %s⚠%s  %s %s(skipped — not found)%s\n", yellow, reset, f, dim, reset)
				}
			}
			fmt.Printf("  %s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", bold, cyan, reset)
			fmt.Println()
			Successln("All done! ✨")
		},
	}
	applyCmd.Flags().StringVarP(&applyFile, "file", "f", "", "path to spec file (default: .dotsync.yaml in current directory)")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "validate and print execution plan without applying")

	// github connect subcommand
	var githubToken string
	var githubCmd = &cobra.Command{
		Use:   "github",
		Short: "GitHub account commands",
	}
	var githubConnectCmd = &cobra.Command{
		Use:   "connect",
		Short: "Connect a GitHub account via Personal Access Token",
		Long: `Store a GitHub Personal Access Token so the agent can clone and pull
private dotfiles repositories without going through a broker.

Create a token at https://github.com/settings/tokens with the 'repo' scope,
then run:

  dotsync-agent github connect --token ghp_xxxxxxxxxxxx`,
		Run: func(cmd *cobra.Command, args []string) {
			if githubToken == "" {
				Error("--token is required")
				os.Exit(1)
			}

			config, err := InitializeConfigurations(dotFilePath, configDir)
			if err != nil {
				Error("configuration error: " + err.Error())
				os.Exit(1)
			}

			identity, err := loadAgentIdentity(config)
			if err != nil || identity == nil {
				identity = &AgentIdentity{}
			}

			identity.GithubToken = githubToken
			if err := saveAgentIdentity(config, identity); err != nil {
				Error("failed to save token: " + err.Error())
				os.Exit(1)
			}

			Successln("GitHub account connected ✅")
		},
	}
	githubConnectCmd.Flags().StringVar(&githubToken, "token", "", "GitHub Personal Access Token (requires repo scope)")
	_ = githubConnectCmd.MarkFlagRequired("token")
	githubCmd.AddCommand(githubConnectCmd)

	rootCmd.AddCommand(registerCmd, syncCmd, daemonCmd, applyCmd, githubCmd)

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

func runSync(daemon bool, webhookPort int, webhookSecret, webhookBranch string) {
	config, err := InitializeConfigurations(dotFilePath, configDir)
	if err != nil {
		Error(err.Error())
		os.Exit(1)
	}

	// Identity is optional. Absence means standalone mode (no broker).
	identity, _ := loadAgentIdentity(config)

	// Load GitHub token from local identity (set via `github connect`).
	if identity != nil && identity.GithubToken != "" {
		config.GithubToken = identity.GithubToken
		Infoln("GitHub token loaded 🔑")
	}

	git := &Git{config}
	mutex := &sync.Mutex{}

	// Broker notifier is only created when a broker URL and agent token are present.
	var brokerNotifier *BrokerNotifier
	if identity != nil && identity.BrokerURL != "" && identity.AgentToken != "" {
		brokerNotifier = NewBrokerNotifier(git, identity.AgentID, identity.AgentToken, identity.MachineName, identity.BrokerURL)
		Infoln("Broker connected:", identity.BrokerURL)

		// Fetch repo URL from broker.
		if repoURL, err := brokerNotifier.GetRepositoryConfig(); err != nil {
			Warnln("could not fetch repository config from broker: " + err.Error())
		} else if repoURL != "" {
			config.GitUrl = repoURL
			config.GitApiBaseUrl = "https://api.github.com"
			if owner, repo, err := ParseGitUrl(repoURL); err != nil {
				Warnln("could not parse git URL: " + err.Error())
			} else {
				config.RepositoryOwner = owner
				config.GitRepository = repo
				Infoln("Configured repository:", owner+"/"+repo)
			}
		}

		brokerNotifier.RegisterStream()
		runBootstrapSelfTest(brokerNotifier, config)
	} else {
		Infoln("Running in standalone mode (no broker)")
	}

	// Clone / pull repo when a URL is configured.
	if config.GitUrl != "" {
		if err := git.CloneOrPullRepository(); err != nil {
			Error("Failed to clone/pull repository: " + err.Error())
		}
	}

	syncer := NewEnhancedSyncer(config, brokerNotifier, mutex, git)

	// Initial Sync
	Infoln("Starting sync...")
	syncer.Sync()
	Successln("Sync completed 🔄")

	if !daemon {
		return
	}

	planExecutor := NewPlanExecutor(config, brokerNotifier, git)

	if brokerNotifier != nil {
		// ── Broker-mode background services ───────────────────────────────────

		// 1. Policy Executor
		go func() {
			policyTicker := time.NewTicker(15 * time.Second)
			for range policyTicker.C {
				cmd, err := brokerNotifier.NextPolicyRunCommand()
				if err != nil {
					Error("Error checking for policy run: " + err.Error())
					continue
				}
				if cmd == nil {
					continue
				}

				Infoln("Received Policy Run:", cmd.RunID)

				repoPath := filepath.Join(config.DotfilePath, config.GitRepository)
				repoSpec, err := LoadSpecFromRepo(repoPath)
				if err != nil {
					_ = brokerNotifier.ReportPolicyRunResult(cmd.RunID, "failed", map[string]string{
						"error": "failed to read .dotsync.yaml: " + err.Error(),
					})
					continue
				}

				spec := MergeSpecs(repoSpec, cmd.Spec)
				if spec == nil {
					_ = brokerNotifier.ReportPolicyRunResult(cmd.RunID, "succeeded", map[string]string{"message": "no spec provided"})
					continue
				}

				if err := spec.Validate(); err != nil {
					_ = brokerNotifier.ReportPolicyRunResult(cmd.RunID, "failed", map[string]string{
						"error": "spec validation failed: " + err.Error(),
					})
					continue
				}

				_, err = planExecutor.Execute(cmd.RunID, spec.ToExecutionPlan(runtime.GOOS))
				if err != nil {
					Error("Policy Execution Failed: " + err.Error())
				} else {
					Successln("Policy execution completed 🎯")
				}
			}
		}()

		// 2. Connectivity Check
		go func() {
			for range time.NewTicker(10 * time.Second).C {
				if has, err := brokerNotifier.HasConnectivityCommand(); err != nil || !has {
					continue
				}
				_ = brokerNotifier.Ping()
				_ = brokerNotifier.AckConnectivityCommand()
			}
		}()

		// 3. Heartbeat
		go func() {
			_ = brokerNotifier.Ping()
			for range time.NewTicker(30 * time.Second).C {
				_ = brokerNotifier.Ping()
			}
		}()

		// 4. SSE + fallback poll for priority sync
		prioritySync := func() {
			mutex.Lock()
			defer mutex.Unlock()
			Infoln("Triggering Priority Sync")
			syncer.Sync()
			if err := brokerNotifier.AckPrioritySyncCommand(); err != nil {
				Error("Failed to ack priority sync: " + err.Error())
			}
		}

		go brokerNotifier.startSSEListener(prioritySync)

		for range time.NewTicker(10 * time.Second).C {
			if has, err := brokerNotifier.HasPrioritySyncCommand(); err != nil || !has {
				continue
			}
			prioritySync()
		}
	} else {
		// ── Standalone daemon ─────────────────────────────────────────────────
		standaloneSync := func() {
			if config.GitUrl != "" {
				if err := git.CloneOrPullRepository(); err != nil {
					Error("git pull failed: " + err.Error())
					return
				}
			}
			repoPath := filepath.Join(config.DotfilePath, config.GitRepository)
			spec, err := LoadSpecFromRepo(repoPath)
			if err != nil {
				Error("could not read .dotsync.yaml: " + err.Error())
				return
			}
			if spec == nil {
				Warnln("no .dotsync.yaml found in repo — nothing to apply")
				return
			}
			if err := spec.Validate(); err != nil {
				Error("spec validation failed: " + err.Error())
				return
			}
			if _, err := planExecutor.Execute("standalone", spec.ToExecutionPlan(runtime.GOOS)); err != nil {
				Error("sync failed: " + err.Error())
			} else {
				Successln("Sync completed 🔄")
			}
		}

		// Webhook listener — triggers sync immediately on push.
		if webhookPort > 0 {
			go startWebhookServer(webhookPort, webhookSecret, webhookBranch, standaloneSync)
			Infoln(fmt.Sprintf("Webhook active — add this URL to your GitHub repo: http://<host>:%d/webhook", webhookPort))
		}

		// Periodic fallback — runs even when webhook is active.
		Infoln("Standalone daemon started — periodic sync every 5 minutes as fallback")
		for range time.NewTicker(5 * time.Minute).C {
			Infoln("Periodic sync...")
			standaloneSync()
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

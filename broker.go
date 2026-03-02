package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
)

// BrokerNotifier handles communication with an external broker service for monitoring and notifications.
// It sends sync events and status updates to a centralized broker for multi-machine coordination.
type BrokerNotifier struct {
	machine     string // Unique machine identifier (Agent ID)
	machineName string // Human-readable machine name
	brokerUrl   string // Broker service URL
	git         *Git   // Git instance for accessing repository information
	agentToken  string // Agent authentication token
}

// SyncStatus represents the synchronization state sent to the broker
type SyncStatus struct {
	LocalCommit      string `json:"local_commit"`
	LocalCommitTime  string `json:"local_commit_time"`
	RemoteCommit     string `json:"remote_commit"`
	RemoteCommitTime string `json:"remote_commit_time"`
	LastSyncTime     string `json:"last_sync_time"`
	IsSync           bool   `json:"is_sync"`
	Error            string `json:"error,omitempty"`
}

// Machine represents a machine registered with the broker service
type Machine struct {
	Id           string     `json:"_id"`            // Unique machine identifier
	Name         string     `json:"name,omitempty"` // Human-readable machine name
	Platform     string     `json:"platform"`       // OS Platform (linux, darwin)
	Distro       string     `json:"distro"`         // Distribution (ubuntu, fedora, macos)
	Version      string     `json:"version"`        // OS Version
	Manager      string     `json:"manager"`        // Package Manager (apt, dnf, brew)
	Hostname     string     `json:"hostname"`       // Hostname
	Arch         string     `json:"arch"`           // CPU Architecture
	AgentVersion string     `json:"agent_version"`  // Agent Version
	Uptime       int64      `json:"uptime"`         // Uptime in seconds
	IP           string     `json:"ip"`             // IP Address
	SyncStatus   SyncStatus `json:"sync_details"`   // Current sync status of the machine
}

type AgentIdentity struct {
	AgentToken  string `json:"agent_token"`
	AgentID     string `json:"agent_id"`
	MachineName string `json:"machine_name"`
	BrokerURL   string `json:"broker_url"`
}

type PolicyRunCommandPolicy struct {
	Id    string   `json:"id"`
	Name  string   `json:"name"`
	Mode  string   `json:"mode"`
	Paths []string `json:"paths"`
}

type PolicyRunCommand struct {
	RunID         string                   `json:"run_id"`
	Policies      []PolicyRunCommandPolicy `json:"policies"`
	ExecutionPlan *ExecutionPlan           `json:"execution_plan"`
}

func loadAgentIdentity(config *Configurations) (*AgentIdentity, error) {
	if config == nil || config.ConfigPath == "" {
		return nil, nil
	}

	identityPath := path.Join(config.ConfigPath, "agent_identity.json")

	data, err := os.ReadFile(identityPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("agent identity file is empty")
	}

	var identity AgentIdentity
	err = json.Unmarshal(data, &identity)
	if err != nil {
		return nil, err
	}

	if identity.AgentToken == "" {
		return nil, fmt.Errorf("agent identity missing token")
	}

	return &identity, nil
}

func saveAgentIdentity(config *Configurations, identity *AgentIdentity) error {
	if config == nil || config.ConfigPath == "" || identity == nil || identity.AgentToken == "" {
		return nil
	}

	identityPath := path.Join(config.ConfigPath, "agent_identity.json")

	data, err := json.Marshal(identity)
	if err != nil {
		return err
	}

	return os.WriteFile(identityPath, data, 0600)
}

func removeAgentIdentity(config *Configurations) error {
	if config == nil || config.ConfigPath == "" {
		return nil
	}
	identityPath := path.Join(config.ConfigPath, "agent_identity.json")
	err := os.Remove(identityPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func (b BrokerNotifier) GetRepositoryConfig() (string, error) {
	if b.brokerUrl == "" || b.agentToken == "" {
		return "", fmt.Errorf("broker URL and agent token are required")
	}

	request, err := http.NewRequest(http.MethodGet, b.brokerUrl+"/agent/repository", nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Agent "+b.agentToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(response.Body)

	if response.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("unauthorized repository config request")
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d", response.StatusCode)
	}

	var payload struct {
		RepoURL string `json:"repo_url"`
	}

	err = json.NewDecoder(response.Body).Decode(&payload)
	if err != nil {
		return "", err
	}

	return payload.RepoURL, nil
}

// NewBrokerNotifier creates a new BrokerNotifier instance.
// It reads configuration from command-line flags, environment variables, and persisted identity.
func NewBrokerNotifier(git *Git, agentID, agentToken, machineName, brokerURL string) *BrokerNotifier {

	// Fallback to environment variables if flags are not provided
	if brokerURL == "" {
		brokerURL = os.Getenv("DOTFILE_BROKER_URL")
	}
	if agentToken == "" {
		agentToken = os.Getenv("DOTFILE_AGENT_TOKEN")
	}

	var identity *AgentIdentity
	if git != nil && git.config != nil && git.config.ConfigPath != "" {
		var err error
		identity, err = loadAgentIdentity(git.config)
		if err != nil {
			// Log warning but continue, as we might be onboarding
		}
	}

	if identity == nil {
		identity = &AgentIdentity{}
	}

	// Update identity with provided flags/env vars
	updated := false
	if agentID != "" {
		identity.AgentID = agentID
		updated = true
	}
	if agentToken != "" {
		identity.AgentToken = agentToken
		updated = true
	}
	if machineName != "" {
		identity.MachineName = machineName
		updated = true
	}
	if brokerURL != "" {
		identity.BrokerURL = brokerURL
		updated = true
	}

	// Persist identity if updated and we have a config path
	if updated && git != nil && git.config != nil {
		err := saveAgentIdentity(git.config, identity)
		if err != nil {
			Error("Failed to persist agent identity:", err.Error())
		} else {
			Infoln("Persisted agent identity to local configuration directory")
		}
	}

	// Use identity values for the notifier
	finalAgentID := identity.AgentID
	finalBrokerURL := identity.BrokerURL
	finalAgentToken := identity.AgentToken

	if finalBrokerURL != "" && finalAgentToken == "" {
		Error("Broker notifier configured without agent identity; priority sync and connectivity commands are disabled")
	}

	if finalAgentID != "" || finalBrokerURL != "" || finalAgentToken != "" {
		Infoln("Broker notifier is enabled and listening for events on:", finalBrokerURL)
	}

	return &BrokerNotifier{
		machine:     finalAgentID,
		machineName: identity.MachineName,
		brokerUrl:   finalBrokerURL,
		git:         git,
		agentToken:  finalAgentToken,
	}
}

// SyncEvent sends a sync progress event to the broker service.
// This allows real-time monitoring of sync operations across multiple machines.
// Only sends if both machine ID and broker URL are configured.
func (b BrokerNotifier) SyncEvent(payload SyncEvent) {
	if b.machine != "" && b.brokerUrl != "" {
		v, _ := json.Marshal(payload)
		request, err := http.NewRequest("POST", b.brokerUrl+"/machines/"+b.machine+"/sync-event", bytes.NewBuffer(v))
		if err != nil {
			Error("Failed to send notification to broker:", err.Error())
			return
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Agent "+b.agentToken)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			Error("Failed to send notification to broker:", err.Error())
			return
		}

		if response.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(response.Body)
			Error("Failed to send notification to broker:", string(body))
			return
		}
	}
}

// SyncStatus sends the current sync status to the broker service.
// This updates the broker with the latest local/remote commit information.
// Runs asynchronously in a goroutine to avoid blocking the sync process.
func (b BrokerNotifier) SyncStatus(payload any) {
	if b.machine != "" && b.brokerUrl != "" {
		go func() {
			v, err := json.Marshal(payload)
			if err != nil {
				Error("Failed to marshal sync status payload:", err.Error())
				return
			}
			Infoln("Sending SyncStatus payload:", string(v))

			request, _ := http.NewRequest("POST", b.brokerUrl+"/machines/"+b.machine+"/sync-status", bytes.NewBuffer(v))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Agent "+b.agentToken)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				fmt.Println(err)
				Error("Failed to send notification to broker:", err.Error())
				return
			}

			if response.StatusCode != 200 {
				Error("Failed to send notification to broker:", response.Status)
			}
		}()
	}
}

// RegisterStream registers this machine with the broker service.
// It sends the machine ID, name, and current local commit information.
// This is called on startup to announce the machine's presence to the broker.
// Runs asynchronously in a goroutine.
func (b BrokerNotifier) RegisterStream() {
	if b.machine != "" && b.brokerUrl != "" && b.agentToken != "" {
		go func() {

			localCommit, err := b.git.LocalCommit()
			if err != nil {
				Error("Failed to get local commit:", err.Error())
				return
			}
			osInfo := GetOSInfo()

			machine := Machine{
				Id:           b.machine,
				Name:         b.machineName,
				Platform:     osInfo.Platform,
				Distro:       osInfo.Distro,
				Version:      osInfo.Version,
				Manager:      osInfo.Manager,
				Hostname:     osInfo.Hostname,
				Arch:         osInfo.Arch,
				AgentVersion: osInfo.AgentVersion,
				Uptime:       osInfo.Uptime,
				IP:           osInfo.IP,
				SyncStatus: SyncStatus{
					LocalCommit: localCommit.Id,
				},
			}

			body, err := json.Marshal(machine)
			if err != nil {
				Error("Failed to marshal machine:", err.Error())
				return
			}

			req, err := http.NewRequest("POST", b.brokerUrl+"/machines", bytes.NewBuffer(body))
			if err != nil {
				Error("Unable to create broker request:", err.Error())
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Agent "+b.agentToken)

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				Error("Unable to send broker notifier:", err.Error())
				return
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
				Error("Unable to send broker notifier:", res.Status)
				return
			}
		}()
	}
}

func (b BrokerNotifier) HasPrioritySyncCommand() (bool, error) {
	if b.machine == "" || b.brokerUrl == "" || b.agentToken == "" {
		return false, nil
	}

	request, err := http.NewRequest(http.MethodGet, b.brokerUrl+"/agent/priority-sync", nil)
	if err != nil {
		return false, err
	}
	request.Header.Set("Authorization", "Agent "+b.agentToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return false, err
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(response.Body)

	if response.StatusCode == http.StatusUnauthorized {
		return false, fmt.Errorf("unauthorized priority sync command request")
	}

	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code %d", response.StatusCode)
	}

	var payload struct {
		HasCommand bool `json:"has_command"`
	}

	err = json.NewDecoder(response.Body).Decode(&payload)
	if err != nil {
		return false, err
	}

	return payload.HasCommand, nil
}

func (b BrokerNotifier) AckPrioritySyncCommand() error {
	if b.machine == "" || b.brokerUrl == "" || b.agentToken == "" {
		return nil
	}
	request, err := http.NewRequest(http.MethodPost, b.brokerUrl+"/agent/priority-sync/ack", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Agent "+b.agentToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(response.Body)
	if response.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized priority sync command ack")
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", response.StatusCode)
	}
	return nil
}

func (b BrokerNotifier) HasConnectivityCommand() (bool, error) {
	if b.machine == "" || b.brokerUrl == "" || b.agentToken == "" {
		return false, nil
	}
	req, err := http.NewRequest(http.MethodGet, b.brokerUrl+"/agent/connectivity", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Agent "+b.agentToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func(body io.ReadCloser) { _ = body.Close() }(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return false, fmt.Errorf("unauthorized connectivity command request")
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	var payload struct {
		HasCommand bool `json:"has_command"`
	}
	err = json.NewDecoder(resp.Body).Decode(&payload)
	if err != nil {
		return false, err
	}
	return payload.HasCommand, nil
}

func (b BrokerNotifier) AckConnectivityCommand() error {
	if b.machine == "" || b.brokerUrl == "" || b.agentToken == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodPost, b.brokerUrl+"/agent/connectivity/ack", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Agent "+b.agentToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func(body io.ReadCloser) { _ = body.Close() }(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized connectivity command ack")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	return nil
}

func (b BrokerNotifier) Ping() error {
	if b.brokerUrl == "" || b.agentToken == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodPost, b.brokerUrl+"/agent/ping", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Agent "+b.agentToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func(body io.ReadCloser) { _ = body.Close() }(resp.Body)
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	return nil
}

func (b BrokerNotifier) NextPolicyRunCommand() (*PolicyRunCommand, error) {
	if b.brokerUrl == "" || b.agentToken == "" {
		return nil, nil
	}

	req, err := http.NewRequest(http.MethodGet, b.brokerUrl+"/agent/policy-run", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Agent "+b.agentToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(body io.ReadCloser) { _ = body.Close() }(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized policy run request")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	var payload struct {
		HasRun        bool                     `json:"has_run"`
		RunID         string                   `json:"run_id"`
		Policies      []PolicyRunCommandPolicy `json:"policies"`
		ExecutionPlan *ExecutionPlan           `json:"execution_plan"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if !payload.HasRun || payload.RunID == "" {
		return nil, nil
	}

	if payload.ExecutionPlan != nil {
		if err := payload.ExecutionPlan.Validate(); err != nil {
			return nil, fmt.Errorf("invalid execution plan received: %v", err)
		}
	}

	return &PolicyRunCommand{
		RunID:         payload.RunID,
		Policies:      payload.Policies,
		ExecutionPlan: payload.ExecutionPlan,
	}, nil
}

func (b BrokerNotifier) ReportPolicyRunResult(runID string, status string, result interface{}) error {
	if b.brokerUrl == "" || b.agentToken == "" || runID == "" {
		return nil
	}

	body, err := json.Marshal(struct {
		Status string      `json:"status"`
		Result interface{} `json:"result"`
	}{
		Status: status,
		Result: result,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, b.brokerUrl+"/agent/policy-run/"+runID+"/result", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Agent "+b.agentToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func(body io.ReadCloser) { _ = body.Close() }(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized policy run result submission")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}

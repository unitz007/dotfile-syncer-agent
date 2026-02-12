# Dotfile Agent Codebase Overview

This document provides a high-level overview of the dotfile agent codebase structure and components.

## Core Components

### 1. Main Application (`main.go`)
- Entry point for the application
- Sets up HTTP server, SSE server, and background processes
- Handles command-line flags using Cobra
- Manages automatic sync polling (every 30 seconds)
- Listens for webhook events

### 2. Configuration (`configurations.go`)
- `Configurations` struct: Holds all agent settings
- `InitializeConfigurations()`: Validates and initializes configuration
- Reads `GITHUB_TOKEN` from environment
- Parses Git repository URL to extract owner and repo name
- Sets up config and dotfile directories

### 3. Git Operations (`git.go`)
- `Git` struct: Handles Git repository interactions
- `RemoteCommit()`: Fetches latest commit from GitHub API
- `LocalCommit()`: Gets latest commit from local repository
- `IsSync()`: Compares local and remote commits
- `CloneOrPullRepository()`: Clones or updates the repository

### 4. Synchronization

#### Custom Syncer (`custom_sync.go`)
- `customSync`: Implements YAML-based dotfile syncing
- Parses `dotfile-config.yaml` to determine what to sync
- Three-step process:
  1. Git checkout (clone/pull)
  2. Parse configuration
  3. Copy files to destinations
- Uses mutex to prevent concurrent syncs

#### Enhanced Syncer (`enhanced_syncer.go`)
- `enhancedSync`: Works with the new enhanced config format
- Supports platform-specific installations
- Similar three-step process as custom syncer

#### Syncer Interface (`syncer.go`)
- `Syncer`: Interface for sync implementations
- `Consumer`: Callback function for sync events
- `SyncEvent`: Progress event structure

### 5. Enhanced Configuration (`enhanced_config.go`)
- `EnhancedConfig`: Structured config with software metadata
- `DotfileEntry`: Software with install commands and files
- `GetInstallCommand()`: Returns platform-specific install command
- `GetPlatform()`: Detects current OS (linux, darwin, windows)
- `GetConfigPaths()`: Converts config to file paths

### 6. Software Installation (`install_software.go`)
- `InstallSoftware()`: Installs all software from config
- `InstallSpecificSoftware()`: Installs selected packages
- `ListSoftware()`: Lists available software
- `ShowPlatformInfo()`: Shows platform-specific commands
- Interactive and non-interactive modes

### 7. HTTP Handlers (`handlers.go`)
- `SyncHandler`: Handles `/sync` endpoint
- POST: Triggers manual sync with SSE progress stream
- GET: Returns sync status or establishes SSE connection
  - `?stream=sync-trigger`: SSE for trigger events
  - `?stream=sync-status`: SSE for status updates
  - No param: JSON sync status

### 8. Broker Integration (`broker.go`)
- `BrokerNotifier`: Sends events to external broker service
- `SyncEvent()`: Sends sync progress events
- `SyncStatus()`: Sends sync status updates
- `RegisterStream()`: Registers machine with broker
- Optional feature (requires `DOTFILE_MACHINE_ID` and `DOTFILE_BROKER_URL`)

### 9. Persistence (`persistence.go`)
- `Persistence`: Interface for storing sync metadata
- `docliteImpl`: Implementation using doclite embedded database
- Stores `SyncStash` records with sync history
- Database file: `~/.config/dotfile-agent/dotfile-agent.doclite`

### 10. Models (`model.go`)
- `Commit`: Git commit information
- `SyncStatusResponse`: Sync status between local/remote
- `GitWebHookCommitResponse`: Webhook payload structure
- `GitHttpCommitResponse`: GitHub API response structure
- `SyncStash`: Persisted sync metadata

### 11. SSE Client (`sseclient.go`)
- `SseClient`: Parses webhook SSE events
- Implements `io.Writer` interface
- Triggers sync when commits pushed to main branch

### 12. I/O Utilities (`io.go`)
- `Infoln()`, `Info()`: Log informational messages
- `Error()`: Log error messages
- `ConsoleSyncConsumer()`: Displays sync progress in console

### 13. Constants (`constant.go`)
- Application-wide constants
- Default port, sync labels, sync types

## Configuration Files

### dotfile-config.yaml (Legacy Format)
```yaml
home:
  - .bashrc
  - .config:
      nvim;
```

### dotfile-config.yaml (Enhanced Format)
```yaml
dotfiles:
  - software: neovim
    install:
      linux: apt install -y neovim
      darwin: brew install neovim
    files:
      - path: nvim;
        target: home/.config
```

## Data Flow

1. **Startup**: Initialize config, Git, broker, syncer
2. **Background Polling**: Check for remote changes every 30 seconds
3. **Webhook Events**: Receive push notifications via SSE
4. **Manual Trigger**: User calls POST /sync
5. **Sync Process**:
   - Lock mutex
   - Clone/pull repository
   - Parse configuration
   - Copy files to destinations
   - Notify broker
   - Unlock mutex
6. **Progress Reporting**: Send events to consumers (console, SSE, broker)

## Key Features

- **Automatic Sync**: Polls remote every 30 seconds
- **Webhook Support**: Real-time sync on Git push
- **Manual Sync**: HTTP endpoint for on-demand sync
- **Progress Tracking**: Real-time progress via SSE
- **Broker Integration**: Multi-machine coordination
- **Platform Support**: Linux, macOS, Windows
- **Flexible Config**: YAML-based dotfile mapping
- **Software Installation**: Automated dependency installation

## Environment Variables

- `GITHUB_TOKEN`: Required - GitHub personal access token
- `DOTFILE_MACHINE_ID`: Optional - Unique machine identifier for broker
- `DOTFILE_BROKER_URL`: Optional - Broker service URL

## Command-Line Flags

- `-p, --port`: HTTP port (default: 3000)
- `-w, --webhook`: Git webhook URL
- `-d, --dotfile-path`: Dotfile directory path
- `-c, --config-dir`: Configuration directory path
- `-g, --git-url`: Git repository URL
- `-b, --git-api-base-url`: Git API base URL (default: https://api.github.com)

## Architecture Patterns

- **Interface-based design**: Syncer, Persistence, Consumer
- **Dependency injection**: Components receive dependencies via constructors
- **Event-driven**: Sync progress communicated via events
- **Concurrent-safe**: Mutex prevents simultaneous syncs
- **Modular**: Each component has single responsibility

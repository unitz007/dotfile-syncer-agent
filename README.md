# Dotfile-syncer Agent

This project is a Go application that listens to webhook events and performs synchronization tasks periodically using a cron job. It's set up to receive webhook notifications and sync data automatically.

## Features

- **Cron Job**: Runs periodically to perform synchronization tasks.
- **Webhook Listener**: Listens for webhooks and triggers synchronization.
- **File Synchronization**: Syncs files from a configured directory.
- **HTTP Server**: Starts an HTTP server to handle sync requests.

## Getting Started

### Prerequisites

- Go 1.23 or higher
- A valid webhook URL (e.g., from [smee.io](https://smee.io/))

### Installation

1. Clone the repository:

    ```sh
    git clone https://github.com/unitz007/dotfile-syncer-agent.git
    cd dotfile-syncer-agent
    ```

2. Install dependencies:

    ```sh
    go mod tidy
    ```

3. Build the project:

    ```sh
    go build -o syncer
    ```

### Usage

1. Run the application:

    ```sh
    ./syncer
    ```

   By default, the application runs on port 3000. You can change the port using the `--port` flag:

    ```sh
    ./syncer --port 8080
    ```

2. Configure the webhook URL and dotfile directory (optional):

    ```sh
    ./syncer --webhook https://your-webhook.url --dotfile-path /path/to/dotfiles
    ```

### Configuration

The application can be configured using command-line flags:

- `--port, -p`: HTTP port to run on (default: `3000`)
- `--webhook, -w`: Webhook URL to listen for events (default: `https://smee.io/your-webhook-url`)
- `--dotfile-path, -d`: Path to the dotfile directory (default: user's config directory)

### How It Works

1. On starting, the application sets up a cron job that runs every 5 seconds.
2. It listens for webhook events to trigger synchronization tasks.
3. When a relevant event is received, it syncs files to the configured directory.
4. The HTTP server provides endpoints to manually trigger syncs if needed.

### Logging

The application uses basic logging to provide info and error messages about its status and operations.

### Dependencies

- [r3labs/sse](https://github.com/r3labs/sse)
- [robfig/cron](https://github.com/robfig/cron)
- [spf13/cobra](https://github.com/spf13/cobra)

[//]: # (## License)

[//]: # (This project is licensed under the MIT License - see the [LICENSE] file for details.)

[//]: # (## Contributing)

[//]: # (Please read [CONTRIBUTING.md]&#40;CONTRIBUTING.md&#41; for details on our code of conduct and the process for submitting pull requests.)

## Acknowledgments

- Inspired by various webhook handling and synchronization tools.
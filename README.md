# Dotfile Agent

The Dotfile Agent is a robust command-line tool meticulously crafted to streamline the management and synchronization of
your dotfiles. It leverages the power of Git and a YAML configuration file to ensure that your precious configurations
are always up-to-date and readily available across your machines.

## Features

* **Git Integration:**  The agent seamlessly integrates with Git, allowing you to store your dotfiles in a dedicated
  repository. It supports cloning or pulling the latest changes from your repository, keeping your local dotfiles in
  sync with the remote version.
* **YAML Configuration:**  A simple yet powerful YAML file dictates how the agent should handle your dotfiles. You can
  specify which files or directories to track, their destination locations, and any special instructions.
* **Automatic Synchronization:**  The agent can be set up to automatically synchronize your dotfiles upon detecting
  changes in the Git repository. This ensures that your configurations are always consistent and up-to-date.
* **Manual Synchronization:**  You can also trigger synchronization manually whenever you desire, giving you complete
  control over the process.
* **Progress Tracking:**  The agent provides detailed progress updates during synchronization, letting you know exactly
  what's happening and how far along the process is.
* **Error Handling:**  In case of any errors during synchronization, the agent will provide clear error messages,
  helping you troubleshoot and resolve issues quickly.
* **Webhook Support:**  The agent listens for webhook events from your Git repository, triggering synchronization
  automatically when new commits are pushed.
* **Broker Notification:**  The agent can send notifications to a broker service, allowing you to monitor the status of
  your dotfiles and receive alerts in real time.

## Usage

### Prerequisites

* Git installed and configured on your machine.
* A Git repository containing your dotfiles.
* A `dotfile-config.yaml` file in your Git repository specifying the synchronization rules.

### Installation
* Extract the archive and move the `dotfile-agent` executable to a directory in your system's PATH (e.g.,
  `/usr/local/bin`).

### Configuration

* **dotfile-config.yaml:**  Create a `dotfile-config.yaml` file in the root of your Git repository. This file will
  define how your dotfiles should be synchronized.

* **Environment Variables:**  Set the following environment variables:
    * `GITHUB_TOKEN`:  Your GitHub personal access token (if using GitHub as your Git provider).
    * `DOTFILE_MACHINE_ID`:  A unique identifier for your machine.
    * `DOTFILE_BROKER_URL`:  The URL of your broker service (if using broker notifications).

### Options

* `-p, --port`:  Specify the HTTP port to run on (default: 3000).
* `-w, --webhook`:  Set the Git webhook URL.
* `-d, --dotfile-path`:  Set the path to your dotfile directory.
* `-c, --config-dir`:  Set the path to your configuration directory.
* `-g, --git-url`:  Set the Git URL of your dotfiles repository.
* `-b, --git-api-base-url`:  Set the base URL of the Git API (default: `https://api.github.com`).

## Examples

### dotfile-config.yaml

```yaml
home:
  - .bashrc       # $HOME/.bashrc
  - .vimrc        # $HOME/.vimrc
  - .config: # $HOME/.config
      nvim;       # designates a directory - $HOME/.config/nvim
```

This project is licensed under the MIT License.

Please note that the above README.md file is generated based on the provided source code excerpts. It assumes that the
project is intended to be used with Git and GitHub. You may need to adjust certain aspects, such as the installation
instructions or the example configuration, to align with the specific implementation details. 
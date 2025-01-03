# Dotfile Agent

## Overview

This project is a dotfile management agent designed to automate the synchronization of dotfiles between a local system and a remote Git repository. It leverages webhooks, cron jobs, and a defined series of synchronization steps to ensure that the local system remains up-to-date with the latest changes in the repository.

## Features

* **Automated Synchronization:**  Utilizes webhooks to trigger synchronization upon changes in the remote Git repository.
* **Periodic Checks:** A cron job runs every 5 seconds to check for updates and ensures regular synchronization.
* **Step-by-step Synchronization:** A structured process with defined steps:
    * **Git repository checkout:**  Clones the repository or pulls the latest changes.
    * **Read dotfile configurations:** Parses a YAML file (`dotfile-config.yaml`) to identify files for synchronization.
    * **Copy dotfiles to configured locations:** Copies files to their designated locations on the local system.
* **Event Handling:** Employs a channel-based event system to track and report the progress, success, and errors of each synchronization step.
* **Broker Notifications:** Sends notifications to a designated broker about sync triggers and status updates.
* **Persistence:**  Stores information about the last synchronization using Doclite, enabling tracking and retrieval of previous synchronization details.

## Configuration

The dotfile agent is configured using environment variables and command-line flags.

**Environment Variables:**

*   `DOTFILE_MACHINE_ID`:  Identifies the machine for broker notifications.
*   `DOTFILE_BROKER_URL`: Specifies the URL of the broker for notifications.
*   `GITHUB_TOKEN`:  GitHub personal access token for authentication.

**Command-line Flags:**

*   `-p, --port`:  HTTP port for the agent to listen on (default: 3000).
*   `-w, --webhook`: Git webhook URL.
*   `-d, --dotfile-path`: Path to the dotfiles directory.
*   `-c, --config-dir`:  Path to the configuration directory.
*   `-g, --git-url`:  Git repository URL.
*   `-b, --git-api-base-url`:  Base URL for the GitHub API.

## Usage

1. **Install Dependencies:** Ensure Go is installed and set up correctly.
2. **Clone the Repository:** Clone the dotfile-agent repository to your local machine.
3. **Set Environment Variables:** Configure the necessary environment variables.
4. **Build and Run:** Build the project and run the executable, providing the required command-line flags.

## Potential Shortcomings

*   **Error Handling:** In some areas, error handling may be insufficient, potentially leading to unnoticed synchronization failures. More robust error handling is crucial for reliability.
*   **Hardcoded Branch:** Synchronization is only triggered for changes pushed to the 'main' branch, limiting flexibility for users who prefer different branches. Configurable branch selection would enhance usability.
*   **Limited Configuration Options:**  The system offers basic configuration options. More advanced options like ignoring specific files, conflict resolution mechanisms, and flexible synchronization scheduling would be beneficial.
*   **Security:** Storing the GitHub token in an environment variable raises security concerns. Utilizing dedicated secrets management tools would be a more secure approach.
*   **Single Point of Failure:** Reliance on a single cron job for synchronization checks creates a potential single point of failure. A more robust scheduling mechanism with redundancy would improve system resilience.

## Future Improvements

*   **Enhance Error Handling:**  Implement more comprehensive error handling throughout the codebase to ensure proper detection and response to failures.
*   **Configurable Branch Selection:**  Allow users to specify the branch to use for synchronization, increasing flexibility and accommodating different workflows.
*   **Expand Configuration Options:** Introduce advanced configuration options, including file exclusion patterns, conflict resolution strategies, and customizable synchronization schedules.
*   **Improve Security:**  Replace the use of environment variables for sensitive information with a secure secrets management solution.
*   **Enhance Scheduling Mechanism:**  Implement a more robust and fault-tolerant scheduling mechanism using a distributed task queue or similar technology to ensure reliable synchronization even in case of failures.

## Contributing

Contributions to the project are welcome. Please follow these steps:

1.  Fork the repository.
2.  Create a new branch for your feature or bug fix.
3.  Make your changes and commit them with descriptive messages.
4.  Push your changes to your fork.
5.  Submit a pull request to the main repository.
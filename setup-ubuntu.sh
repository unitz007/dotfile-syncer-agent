#!/usr/bin/env bash

set -e  # Exit on error

echo "=========================================="
echo "Dotfile Agent - Ubuntu Setup Script"
echo "=========================================="
echo ""

# Configuration
REPO_URL="${REPO_URL:-https://github.com/yourusername/dotfile-agent.git}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/dotfile-agent-build}"

# Check if running on Ubuntu
if ! grep -q "Ubuntu" /etc/os-release 2>/dev/null; then
    echo "Warning: This script is designed for Ubuntu. Proceeding anyway..."
fi

# Check if running as root
if [ "$EUID" -eq 0 ]; then 
    echo "Please do not run this script as root (without sudo)"
    exit 1
fi

# Prompt for repository URL if not set
if [ "$REPO_URL" = "https://github.com/unitz007/dotfile-agent.git" ]; then
    read -p "Enter the GitHub repository URL: " USER_REPO_URL
    if [ -n "$USER_REPO_URL" ]; then
        REPO_URL="$USER_REPO_URL"
    fi
fi

echo "Repository: $REPO_URL"
echo "Install directory: $INSTALL_DIR"
echo ""

# Update package list
echo "Updating package list..."
sudo apt-get update

# Install Git if not present
if ! command -v git &> /dev/null; then
    echo "Installing Git..."
    sudo apt-get install -y git
else
    echo "Git is already installed ($(git --version))"
fi

# Install Go if not present
if ! command -v go &> /dev/null; then
    echo "Installing Go..."
    GO_VERSION="1.22.3"
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
    rm "go${GO_VERSION}.linux-amd64.tar.gz"
    
    # Add Go to PATH
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
        echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc
    fi
    export PATH=$PATH:/usr/local/go/bin
    export PATH=$PATH:$HOME/go/bin
    
    echo "Go installed successfully ($(go version))"
else
    echo "Go is already installed ($(go version))"
fi

# Clone or update the repository
echo ""
if [ -d "$INSTALL_DIR" ]; then
    echo "Directory $INSTALL_DIR already exists."
    read -p "Do you want to pull latest changes? (y/n): " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Pulling latest changes from repository..."
        cd "$INSTALL_DIR"
        git pull origin main || git pull origin master
    else
        echo "Using existing code..."
        cd "$INSTALL_DIR"
    fi
else
    echo "Cloning repository from $REPO_URL..."
    git clone "$REPO_URL" "$INSTALL_DIR"
    cd "$INSTALL_DIR"
fi

# Download Go dependencies
echo ""
echo "Downloading Go dependencies..."
go mod download

# Build the project
echo ""
echo "Building dotfile-agent..."
go build -o dotfile-agent

# Install the binary
echo "Installing dotfile-agent to /usr/local/bin..."
sudo mv dotfile-agent /usr/local/bin/
sudo chmod +x /usr/local/bin/dotfile-agent

# Verify installation
if command -v dotfile-agent &> /dev/null; then
    echo ""
    echo "=========================================="
    echo "Installation completed successfully!"
    echo "=========================================="
    echo ""
    
    # Prompt for systemd service setup
    read -p "Do you want to set up dotfile-agent as a systemd service? (y/n): " -n 1 -r
    echo ""
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo ""
        echo "Setting up systemd service..."
        echo "Please provide the following configuration:"
        echo ""
        
        read -p "GitHub Token: " GITHUB_TOKEN
        read -p "Machine ID: " DOTFILE_MACHINE_ID
        read -p "Broker URL (optional, press Enter to skip): " DOTFILE_BROKER_URL
        read -p "HTTP Port (default: 3000): " PORT
        PORT=${PORT:-3000}
        read -p "Webhook URL: " WEBHOOK_URL
        read -p "Dotfile Path (e.g., /home/$USER/dotfiles): " DOTFILE_PATH
        read -p "Config Directory (e.g., /home/$USER/.config): " CONFIG_DIR
        read -p "Git Repository URL: " GIT_URL
        
        # Create systemd service file
        SERVICE_FILE="/tmp/dotfile-agent.service"
        cat > "$SERVICE_FILE" << EOF
[Unit]
Description=Dotfile Agent - Automated Dotfile Synchronization Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$USER
Group=$USER
WorkingDirectory=$HOME

# Environment variables
Environment="GITHUB_TOKEN=$GITHUB_TOKEN"
Environment="DOTFILE_MACHINE_ID=$DOTFILE_MACHINE_ID"
Environment="DOTFILE_BROKER_URL=$DOTFILE_BROKER_URL"

# Service command
ExecStart=/usr/local/bin/dotfile-agent \\
    --port=$PORT \\
    --webhook=$WEBHOOK_URL \\
    --dotfile-path=$DOTFILE_PATH \\
    --config-dir=$CONFIG_DIR \\
    --git-url=$GIT_URL

# Restart policy
Restart=always
RestartSec=10

# Security settings
NoNewPrivileges=true
PrivateTmp=true

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=dotfile-agent

[Install]
WantedBy=multi-user.target
EOF
        
        # Install and enable service
        sudo cp "$SERVICE_FILE" /etc/systemd/system/dotfile-agent.service
        sudo systemctl daemon-reload
        sudo systemctl enable dotfile-agent.service
        
        echo ""
        echo "Systemd service installed successfully!"
        echo ""
        echo "Service management commands:"
        echo "  Start:   sudo systemctl start dotfile-agent"
        echo "  Stop:    sudo systemctl stop dotfile-agent"
        echo "  Restart: sudo systemctl restart dotfile-agent"
        echo "  Status:  sudo systemctl status dotfile-agent"
        echo "  Logs:    sudo journalctl -u dotfile-agent -f"
        echo ""
        
        read -p "Do you want to start the service now? (y/n): " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            sudo systemctl start dotfile-agent
            echo ""
            echo "Service started! Check status with: sudo systemctl status dotfile-agent"
        fi
    else
        echo ""
        echo "Skipping systemd service setup."
        echo ""
        echo "Manual usage:"
        echo "1. Set required environment variables:"
        echo "   export GITHUB_TOKEN='your_github_token'"
        echo "   export DOTFILE_MACHINE_ID='your_machine_id'"
        echo "   export DOTFILE_BROKER_URL='your_broker_url'  # (optional)"
        echo ""
        echo "2. Run the agent:"
        echo "   dotfile-agent --help"
    fi
    
    echo ""
    echo "Note: If Go was just installed, run 'source ~/.bashrc' to update your PATH"
else
    echo "Installation failed. Please check the errors above."
    exit 1
fi

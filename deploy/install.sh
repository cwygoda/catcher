#!/bin/bash
set -euo pipefail

# Catcher LaunchDaemon installer
# Usage: sudo ./install.sh [username]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
error() { echo -e "${RED}[-]${NC} $1"; exit 1; }

# Check root
[[ $EUID -eq 0 ]] || error "Run with sudo"

# Get username
if [[ $# -ge 1 ]]; then
    RUN_USER="$1"
else
    RUN_USER="${SUDO_USER:-$(whoami)}"
    read -p "Run as user [$RUN_USER]: " INPUT_USER
    RUN_USER="${INPUT_USER:-$RUN_USER}"
fi

# Validate user exists
id "$RUN_USER" &>/dev/null || error "User '$RUN_USER' does not exist"
USER_HOME=$(dscl . -read /Users/"$RUN_USER" NFSHomeDirectory | awk '{print $2}')
info "Installing for user: $RUN_USER (home: $USER_HOME)"

# Paths
PLIST_SRC="$SCRIPT_DIR/com.cwygoda.catcher.plist"
PLIST_DST="/Library/LaunchDaemons/com.cwygoda.catcher.plist"
BINARY_SRC="$PROJECT_DIR/bin/catcher"
BINARY_DST="/usr/local/bin/catcher"
CONFIG_DIR="/etc/catcher"
CONFIG_FILE="$CONFIG_DIR/config.toml"
DATA_DIR="/var/lib/catcher"
LOG_DIR="/var/log/catcher"
SERVICE="system/com.cwygoda.catcher"

# Check binary exists
[[ -f "$BINARY_SRC" ]] || error "Binary not found: $BINARY_SRC\nRun 'make build' first"

# Stop existing service if running
if launchctl print "$SERVICE" &>/dev/null; then
    warn "Stopping existing service..."
    launchctl bootout "$SERVICE" 2>/dev/null || true
    sleep 1
fi

# Create directories
info "Creating directories..."
mkdir -p "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
chown "$RUN_USER" "$DATA_DIR" "$LOG_DIR"

# Copy binary
info "Installing binary..."
cp "$BINARY_SRC" "$BINARY_DST"
chmod 755 "$BINARY_DST"

# Create config if not exists
if [[ ! -f "$CONFIG_FILE" ]]; then
    warn "No config found, creating from example..."
    sed "s|YOUR_USERNAME|$RUN_USER|g" "$SCRIPT_DIR/config.toml.example" > "$CONFIG_FILE"
    chmod 644 "$CONFIG_FILE"
    warn "Edit $CONFIG_FILE to customize processors"
fi

# Install plist with user substitution
info "Installing LaunchDaemon..."
sed "s|{{USER}}|$RUN_USER|g" "$PLIST_SRC" > "$PLIST_DST"
chmod 644 "$PLIST_DST"
chown root:wheel "$PLIST_DST"

# Load and start service
info "Starting service..."
launchctl bootstrap system "$PLIST_DST"
sleep 1

# Verify
if launchctl print "$SERVICE" &>/dev/null; then
    PID=$(launchctl print "$SERVICE" 2>/dev/null | grep -E '^\s*pid\s*=' | awk '{print $3}')
    if [[ -n "$PID" && "$PID" != "0" ]]; then
        info "Service running (PID: $PID)"
    else
        warn "Service loaded but not running yet"
    fi
else
    error "Failed to start service"
fi

echo ""
info "Installation complete!"
echo ""
echo "Commands:"
echo "  Status:  sudo launchctl print $SERVICE"
echo "  Restart: sudo launchctl kickstart -k $SERVICE"
echo "  Logs:    tail -f $LOG_DIR/catcher.log"
echo "  Config:  $CONFIG_FILE"

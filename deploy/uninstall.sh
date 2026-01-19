#!/bin/bash
set -euo pipefail

# Catcher LaunchDaemon uninstaller
# Usage: sudo ./uninstall.sh [--purge]

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

PURGE=false
[[ "${1:-}" == "--purge" ]] && PURGE=true

PLIST="/Library/LaunchDaemons/com.cwygoda.catcher.plist"
BINARY="/usr/local/bin/catcher"
CONFIG_DIR="/etc/catcher"
DATA_DIR="/var/lib/catcher"
LOG_DIR="/var/log/catcher"
SERVICE="system/com.cwygoda.catcher"

# Stop service
if launchctl print "$SERVICE" &>/dev/null; then
    info "Stopping service..."
    launchctl bootout "$SERVICE" 2>/dev/null || true
    sleep 1
fi

# Remove plist
if [[ -f "$PLIST" ]]; then
    info "Removing LaunchDaemon..."
    rm "$PLIST"
fi

# Remove binary
if [[ -f "$BINARY" ]]; then
    info "Removing binary..."
    rm "$BINARY"
fi

if $PURGE; then
    warn "Purging all data..."
    rm -rf "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
    info "Removed config, data, and logs"
else
    echo ""
    warn "Kept data directories (use --purge to remove):"
    echo "  Config: $CONFIG_DIR"
    echo "  Data:   $DATA_DIR"
    echo "  Logs:   $LOG_DIR"
fi

echo ""
info "Uninstall complete"

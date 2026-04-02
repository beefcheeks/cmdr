#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="$HOME/.local/bin"
PLIST_NAME="com.mike.cmdr.plist"
LAUNCH_AGENTS="$HOME/Library/LaunchAgents"

echo "cmdr: building..."
cd "$SCRIPT_DIR"
go build -o "$BIN_DIR/cmdr" ./cmd/cmdr
echo "cmdr: installed binary to $BIN_DIR/cmdr"

# Install launchd plist (macOS only)
if [[ "$OSTYPE" == "darwin"* ]]; then
    mkdir -p "$LAUNCH_AGENTS"

    # Stop existing service if loaded
    launchctl bootout "gui/$(id -u)/$PLIST_NAME" 2>/dev/null || true

    sed "s|__CMDR_BIN__|$BIN_DIR/cmdr|g" "$SCRIPT_DIR/$PLIST_NAME" \
        > "$LAUNCH_AGENTS/$PLIST_NAME"

    launchctl bootstrap "gui/$(id -u)" "$LAUNCH_AGENTS/$PLIST_NAME"
    echo "cmdr: launchd service installed and started"
else
    echo "cmdr: skip launchd setup (not macOS)"
    echo "cmdr: you can run 'cmdr start' manually or set up a systemd unit"
fi

echo "cmdr: done ✓"

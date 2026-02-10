#!/bin/bash

set -e

HUNTER3_DIR="$HOME/.hunter3"
CREDENTIALS_FILE="$HUNTER3_DIR/gdrive-credentials.json"

echo "=== Google Drive MCP Plugin Setup ==="
echo

# Create hunter3 directory
if [ ! -d "$HUNTER3_DIR" ]; then
    echo "Creating $HUNTER3_DIR directory..."
    mkdir -p "$HUNTER3_DIR"
fi

# Check if credentials file exists
if [ -f "$CREDENTIALS_FILE" ]; then
    echo "✓ Credentials file found: $CREDENTIALS_FILE"
else
    echo "⚠ Credentials file not found!"
    echo
    echo "To set up Google Drive credentials:"
    echo "1. Go to https://console.cloud.google.com/"
    echo "2. Create a new project (or select existing)"
    echo "3. Enable the Google Drive API"
    echo "4. Create OAuth 2.0 credentials (Desktop app)"
    echo "5. Download the credentials JSON file"
    echo "6. Save it as: $CREDENTIALS_FILE"
    echo
    read -p "Have you downloaded the credentials file? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        read -p "Enter the path to your downloaded credentials file: " DOWNLOADED_FILE
        if [ -f "$DOWNLOADED_FILE" ]; then
            cp "$DOWNLOADED_FILE" "$CREDENTIALS_FILE"
            echo "✓ Credentials copied to $CREDENTIALS_FILE"
        else
            echo "✗ File not found: $DOWNLOADED_FILE"
            exit 1
        fi
    else
        echo "Please complete the setup steps and run this script again."
        exit 1
    fi
fi

# Build the plugin
echo
echo "Building mcp-gdrive..."
cd "$(dirname "$0")/../.."
make mcp-gdrive

echo
echo "✓ Build complete!"
echo

# Register with Claude CLI
BINARY_PATH="$(pwd)/dist/mcp-gdrive"
echo "Registering with Claude CLI..."
claude mcp add --transport stdio mcp-gdrive -- "$BINARY_PATH" || {
    echo "⚠ Failed to register with Claude CLI"
    echo "You can manually register later with:"
    echo "  claude mcp add --transport stdio mcp-gdrive -- $BINARY_PATH"
}

echo
echo "=== Setup Complete! ==="
echo
echo "Next steps:"
echo "1. Start Claude CLI: claude"
echo "2. Try: 'List the files in my Google Drive'"
echo "3. On first use, you'll be prompted to authenticate"
echo
echo "For more information:"
echo "  - Quick Start: cmd/mcp-gdrive/QUICKSTART.md"
echo "  - Full README: cmd/mcp-gdrive/README.md"
echo "  - Examples: cmd/mcp-gdrive/example-usage.md"
echo "  - Logs: tail -f ~/.hunter3/logs/mcp-gdrive.log"
echo

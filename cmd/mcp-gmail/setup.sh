#!/bin/bash
set -e

echo "MCP Gmail Plugin Setup"
echo "======================"
echo

# Create config directory
CONFIG_DIR="$HOME/.hunter3"
mkdir -p "$CONFIG_DIR"
echo "✓ Created config directory: $CONFIG_DIR"

# Check for credentials file
CREDENTIALS_FILE="$CONFIG_DIR/gmail-credentials.json"
if [ ! -f "$CREDENTIALS_FILE" ]; then
    echo
    echo "❌ Gmail credentials file not found!"
    echo
    echo "Please follow these steps:"
    echo "1. Go to https://console.cloud.google.com/"
    echo "2. Create or select a project"
    echo "3. Enable the Gmail API"
    echo "4. Create OAuth 2.0 Desktop credentials"
    echo "5. Download the credentials JSON file"
    echo "6. Save it to: $CREDENTIALS_FILE"
    echo
    exit 1
fi

echo "✓ Found credentials file: $CREDENTIALS_FILE"

# Build the plugin
echo
echo "Building mcp-gmail plugin..."
cd "$(dirname "$0")/../.."
go build -o bin/mcp-gmail ./cmd/mcp-gmail
echo "✓ Built binary: $(pwd)/bin/mcp-gmail"

echo
echo "Setup complete!"
echo
echo "To run the plugin:"
echo "  ./bin/mcp-gmail"
echo
echo "On first run, you'll be prompted to authenticate with Google."
echo "The authorization token will be saved to: $CONFIG_DIR/gmail-token.json"

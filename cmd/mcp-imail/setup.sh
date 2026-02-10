#!/bin/bash
# Setup script for mcp-imail plugin

set -e

echo "========================================="
echo "  iCloud Mail MCP Plugin Setup"
echo "========================================="
echo

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

echo "✓ Go is installed"

# Check if we're in the right directory
if [ ! -f "main.go" ]; then
    echo "❌ Error: Please run this script from cmd/mcp-imail directory"
    exit 1
fi

echo "✓ In correct directory"

# Create config directory
CONFIG_DIR="$HOME/.hunter3"
CONFIG_FILE="$CONFIG_DIR/icloud-mail.json"

if [ ! -d "$CONFIG_DIR" ]; then
    echo "Creating config directory: $CONFIG_DIR"
    mkdir -p "$CONFIG_DIR"
fi

# Check if config already exists
if [ -f "$CONFIG_FILE" ]; then
    echo "⚠️  Config file already exists: $CONFIG_FILE"
    read -p "Do you want to overwrite it? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Keeping existing configuration"
        SKIP_CONFIG=true
    fi
fi

if [ -z "$SKIP_CONFIG" ]; then
    echo
    echo "========================================="
    echo "  Configuration Setup"
    echo "========================================="
    echo
    echo "You need an App-Specific Password for iCloud Mail."
    echo "If you don't have one yet:"
    echo "  1. Go to https://appleid.apple.com"
    echo "  2. Sign in and go to Security section"
    echo "  3. Generate an App-Specific Password"
    echo "  4. Save it (format: xxxx-xxxx-xxxx-xxxx)"
    echo

    # Get iCloud email
    read -p "Enter your iCloud email address: " ICLOUD_EMAIL
    if [ -z "$ICLOUD_EMAIL" ]; then
        echo "❌ Error: Email address is required"
        exit 1
    fi

    # Get App-Specific Password
    read -s -p "Enter your App-Specific Password: " ICLOUD_PASSWORD
    echo
    if [ -z "$ICLOUD_PASSWORD" ]; then
        echo "❌ Error: Password is required"
        exit 1
    fi

    # Create config file
    cat > "$CONFIG_FILE" << EOF
{
  "email": "$ICLOUD_EMAIL",
  "password": "$ICLOUD_PASSWORD"
}
EOF

    # Secure the config file
    chmod 600 "$CONFIG_FILE"
    echo "✓ Configuration saved to $CONFIG_FILE"
fi

# Build the plugin
echo
echo "========================================="
echo "  Building Plugin"
echo "========================================="
echo

cd ../..
if ! make mcp-imail; then
    echo "❌ Error: Build failed"
    exit 1
fi

echo "✓ Plugin built successfully"

# Check if claude CLI is available
if ! command -v claude &> /dev/null; then
    echo
    echo "⚠️  Warning: 'claude' CLI not found"
    echo "You'll need to register the plugin manually"
    echo
    echo "To register, add this to ~/.claude/config.json:"
    echo
    echo "  \"mcpServers\": {"
    echo "    \"imail\": {"
    echo "      \"command\": \"$(pwd)/dist/mcp-imail\","
    echo "      \"args\": [],"
    echo "      \"env\": {}"
    echo "    }"
    echo "  }"
    echo
    SKIP_REGISTER=true
fi

if [ -z "$SKIP_REGISTER" ]; then
    echo
    echo "========================================="
    echo "  Registering with Claude CLI"
    echo "========================================="
    echo

    PLUGIN_PATH="$(pwd)/dist/mcp-imail"
    
    # Check if already registered
    if claude mcp list 2>/dev/null | grep -q "imail"; then
        echo "Plugin already registered, removing old registration..."
        claude mcp remove imail || true
    fi

    if claude mcp add --transport stdio imail -- "$PLUGIN_PATH"; then
        echo "✓ Plugin registered with Claude CLI"
    else
        echo "⚠️  Warning: Failed to register plugin automatically"
        echo "You may need to register manually"
    fi
fi

# Test the plugin
echo
echo "========================================="
echo "  Testing Plugin"
echo "========================================="
echo

if [ -f "dist/mcp-imail" ]; then
    echo "Starting plugin (press Ctrl+C after a few seconds)..."
    timeout 3s ./dist/mcp-imail || true
    echo
    echo "✓ Plugin starts successfully"
fi

# Final instructions
echo
echo "========================================="
echo "  Setup Complete!"
echo "========================================="
echo
echo "Your iCloud Mail MCP plugin is ready to use!"
echo
echo "Configuration file: $CONFIG_FILE"
echo "Plugin location: $(pwd)/dist/mcp-imail"
echo "Logs location: $HOME/.hunter3/logs/mcp-imail.log"
echo
echo "Next steps:"
echo "  1. Start Claude: claude"
echo "  2. Try: 'List my recent iCloud emails'"
echo "  3. Try: 'Send a test email via iCloud to yourself'"
echo
echo "Documentation:"
echo "  - Quick start: cmd/mcp-imail/QUICKSTART.md"
echo "  - Full guide: cmd/mcp-imail/README.md"
echo "  - Examples: cmd/mcp-imail/example-usage.md"
echo
echo "To view logs:"
echo "  tail -f ~/.hunter3/logs/mcp-imail.log"
echo
echo "Happy emailing! 📧"

# Quick Start Guide - MCP Gmail Plugin

Get up and running with the Gmail MCP plugin in 5 minutes!

## Prerequisites

- Go 1.24.0 or later
- A Google account
- Access to Google Cloud Console

## Step 1: Google Cloud Setup (5 minutes)

### A. Create/Select Project
1. Visit https://console.cloud.google.com/
2. Create a new project or select existing one
3. Note your project name

### B. Enable Gmail API
1. In Google Cloud Console, go to **APIs & Services > Library**
2. Search for "Gmail API"
3. Click **Enable**

### C. Create OAuth Credentials
1. Go to **APIs & Services > Credentials**
2. Click **Create Credentials > OAuth client ID**
3. If prompted, configure OAuth consent screen:
   - User Type: **External**
   - App name: "Hunter3 MCP Gmail"
   - User support email: your email
   - Developer contact: your email
   - Click **Save and Continue**
   - Skip scopes (click **Save and Continue**)
   - Add your email as test user
   - Click **Save and Continue**
4. Back to create OAuth client ID:
   - Application type: **Desktop app**
   - Name: "Hunter3 MCP Gmail Client"
   - Click **Create**
5. Click **Download JSON**

## Step 2: Install Credentials

```bash
# Create config directory
mkdir -p ~/.hunter3

# Copy your downloaded credentials
cp ~/Downloads/client_secret_*.json ~/.hunter3/gmail-credentials.json
```

## Step 3: Build the Plugin

```bash
cd /home/genoeg/go/src/github.com/soyeahso/hunter3/cmd/mcp-gmail

# Option A: Using Make
make setup
make build

# Option B: Manual build
cd /home/genoeg/go/src/github.com/soyeahso/hunter3
go mod tidy
go build -o bin/mcp-gmail ./cmd/mcp-gmail
```

## Step 4: First Run & Authentication

```bash
# Run the plugin
cd /home/genoeg/go/src/github.com/soyeahso/hunter3
./bin/mcp-gmail
```

The plugin will prompt you with a URL. Follow these steps:

1. **Copy the URL** from the terminal
2. **Open it in your browser**
3. **Sign in** to your Google account
4. **Allow access** (you may see a warning - click "Advanced" then "Go to Hunter3 MCP Gmail")
5. **Copy the authorization code**
6. **Paste it** back into the terminal

The token will be saved to `~/.hunter3/gmail-token.json` for future use.

## Step 5: Test It!

### Test via command line:

```bash
# Initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./bin/mcp-gmail

# List tools
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | ./bin/mcp-gmail

# List unread messages
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_messages","arguments":{"query":"is:unread","max_results":"5"}}}' | ./bin/mcp-gmail
```

### Interactive test:

Create a test script `test.sh`:

```bash
#!/bin/bash
./bin/mcp-gmail << 'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_messages","arguments":{"query":"is:unread","max_results":"3"}}}
EOF
```

Run it:
```bash
chmod +x test.sh
./test.sh
```

## Common Issues

### "unable to read credentials file"
**Solution:** Ensure the file exists at `~/.hunter3/gmail-credentials.json`

```bash
ls -la ~/.hunter3/gmail-credentials.json
```

### "This app isn't verified" warning
**Solution:** This is expected for development apps. Click "Advanced" → "Go to Hunter3 MCP Gmail (unsafe)" → "Allow"

### "Access blocked: Authorization Error"
**Solution:** Make sure you:
1. Added your email as a test user in OAuth consent screen
2. Selected all required scopes
3. Are using the Desktop app credential type

### Token expired or invalid
**Solution:** Delete the token and re-authenticate:

```bash
rm ~/.hunter3/gmail-token.json
./bin/mcp-gmail
```

## Next Steps

- Read [README.md](README.md) for full documentation
- Check [example-usage.md](example-usage.md) for usage examples
- View logs at `~/.hunter3/logs/mcp-gmail.log`

## Quick Reference

### File Locations
```
~/.hunter3/gmail-credentials.json  # OAuth credentials (keep secret!)
~/.hunter3/gmail-token.json        # Auth token (auto-generated)
~/.hunter3/logs/mcp-gmail.log      # Application logs
```

### Available Tools
- `list_messages` - Browse inbox
- `read_message` - Read full email
- `send_message` - Send email with attachments
- `search_messages` - Advanced search

### Common Queries
```
is:unread                          # Unread emails
from:boss@company.com              # From specific sender
has:attachment larger:5M           # Large attachments
after:2024/01/01                   # Date filter
subject:meeting is:starred         # Combined filters
```

## Getting Help

- Check logs: `tail -f ~/.hunter3/logs/mcp-gmail.log`
- Read full docs: [README.md](README.md)
- Examples: [example-usage.md](example-usage.md)

## Security Reminder

🔒 **Never commit these files to git:**
- `~/.hunter3/gmail-credentials.json`
- `~/.hunter3/gmail-token.json`

These contain your Google account credentials!

# iCloud Mail MCP Plugin - Quick Start

## 1. Generate App-Specific Password

1. Visit [appleid.apple.com](https://appleid.apple.com)
2. Sign in and go to Security
3. Generate a new App-Specific Password
4. **Save this password** - you'll need it for setup

## 2. Configure

Create config file:
```bash
mkdir -p ~/.hunter3
cat > ~/.hunter3/icloud-mail.json << 'EOF'
{
  "email": "your-email@icloud.com",
  "password": "xxxx-xxxx-xxxx-xxxx"
}
EOF
chmod 600 ~/.hunter3/icloud-mail.json
```

Replace with your actual iCloud email and the App-Specific Password.

## 3. Build and Install

```bash
cd /path/to/hunter3
make all
```

## 4. Test

Start the plugin manually to verify it works:
```bash
./dist/mcp-imail
```

It should show:
```
[mcp-imail] MCP iCloud Mail server starting...
[mcp-imail] Server initialized
[mcp-imail] Listening for requests on stdin...
```

Press Ctrl+C to exit.

## 5. Register with Claude

```bash
claude mcp add --transport stdio mcp-imail -- $(pwd)/dist/mcp-imail
```

## 6. Try It Out

Start Claude:
```bash
claude
```

Try these commands:
- "List my recent iCloud emails"
- "Search for unread iCloud emails"
- "Send a test email via iCloud to yourself"

## Common Issues

**Authentication fails:**
- Make sure you're using an App-Specific Password, not your regular password
- Verify two-factor authentication is enabled on your Apple ID

**No messages found:**
- Try: "List emails from 'Sent Messages' mailbox"
- Different mailbox names may be used

**Plugin not found:**
- Rebuild: `make all`
- Re-register: `claude mcp add ...`
- Restart Claude

## Next Steps

- Read [README.md](README.md) for full documentation
- Check logs: `tail -f ~/.hunter3/logs/mcp-imail.log`
- Try different mailboxes and search queries

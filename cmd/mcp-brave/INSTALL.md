# Installation Guide for MCP Brave Search Plugin

## Step-by-Step Installation

### 1. Copy Files to Hunter3 Project

Copy the contents of this directory to your Hunter3 project:

```bash
# From the hunter3 project root
cp -r /path/to/mcp-brave cmd/mcp-brave
```

### 2. Update Main Makefile

Edit the main `Makefile` in the hunter3 project root and add:

```makefile
# Add to build targets
dist/mcp-brave: cmd/mcp-brave/*.go
	@mkdir -p dist
	go build -o dist/mcp-brave ./cmd/mcp-brave
```

Then update the `all` target to include `dist/mcp-brave`:

```makefile
all: dist/mcp-brave dist/mcp-other1 dist/mcp-other2 ...
```

### 3. Build the Plugin

```bash
make all
# Or build just this plugin:
make dist/mcp-brave
```

### 4. Update .mcp.json

Update your `.mcp.json` configuration file (typically in your home directory or project root):

```json
{
  "mcpServers": {
    "brave-search": {
      "command": "/absolute/path/to/hunter3/dist/mcp-brave",
      "args": [],
      "env": {
        "BRAVE_API_KEY": "BSA..."
      }
    },
    ... other servers ...
  }
}
```

**Important Notes:**
- Use the **absolute path** to the `dist/mcp-brave` binary
- Replace `BSA...` with your actual Brave Search API key
- Get your API key from: https://brave.com/search/api/

### 5. Restart Your MCP Client

Restart Claude Desktop or your MCP client to load the new server.

### 6. Verify Installation

Check the logs to ensure the server started successfully:

```bash
tail -f ~/.hunter3/logs/mcp-brave.log
```

You should see a message like:
```
2026/02/08 12:00:00 main.go:XX: Starting Brave Search MCP server
```

## Troubleshooting

### Server Not Starting

1. **Check API Key**: Ensure `BRAVE_API_KEY` is set in `.mcp.json`
2. **Check Logs**: Look at `~/.hunter3/logs/mcp-brave.log` for errors
3. **Check Path**: Ensure the `command` path in `.mcp.json` is absolute and correct
4. **Rebuild**: Try running `make clean && make all`

### API Errors

- **401 Unauthorized**: Invalid API key
- **429 Too Many Requests**: Rate limit exceeded
- **403 Forbidden**: Check your API subscription status

### Tools Not Appearing

- Ensure the server is running (check logs)
- Verify `.mcp.json` syntax is valid (use a JSON validator)
- Restart your MCP client completely

## Testing

You can test the tools directly:

```bash
# Example: Test web search
echo '{"method":"tools/call","params":{"name":"brave_web_search","arguments":{"query":"test"}}}' | dist/mcp-brave
```

## Updating

To update the plugin:

1. Make changes to `cmd/mcp-brave/main.go`
2. Run `make dist/mcp-brave`
3. Restart your MCP client

The hot-reload system may pick up changes automatically during development if you're using `autorestart`.

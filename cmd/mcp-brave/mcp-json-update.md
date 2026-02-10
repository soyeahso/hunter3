# .mcp.json Update Instructions

## Where to Find .mcp.json

The `.mcp.json` file is typically located in your home directory or project root. Common locations:

- `~/.claude/.mcp.json`
- `~/.mcp.json`
- `~/hunter3/.mcp.json`

## What to Add

Add the following entry to the `mcpServers` section:

```json
{
  "mcpServers": {
    "brave-search": {
      "command": "/absolute/path/to/hunter3/dist/mcp-brave",
      "args": [],
      "env": {
        "BRAVE_API_KEY": "YOUR_BRAVE_API_KEY_HERE"
      }
    }
  }
}
```

## Example: Complete .mcp.json File

If you already have other MCP servers configured:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "mcp-server-filesystem",
      "args": ["/home/user/documents"]
    },
    "brave-search": {
      "command": "/home/user/hunter3/dist/mcp-brave",
      "args": [],
      "env": {
        "BRAVE_API_KEY": "BSAxxxxxxxxxxxxxxxxxx"
      }
    },
    "weather": {
      "command": "dist/mcp-weather"
    }
  }
}
```

## Important Notes

1. **Use Absolute Path**: The `command` should be an absolute path to the binary
   ```bash
   # Find absolute path
   cd ~/hunter3
   pwd  # This gives you the absolute path
   # Then use: /absolute/path/dist/mcp-brave
   ```

2. **API Key Format**: Brave API keys typically start with "BSA"
   - Get yours from: https://brave.com/search/api/
   - Never commit this file with your actual API key to version control

3. **Environment Variable**: The `BRAVE_API_KEY` is passed to the server process
   - The server will fail to start if this is missing or empty
   - Check logs at `~/.hunter3/logs/mcp-brave.log` if there are issues

4. **JSON Syntax**: Ensure proper JSON formatting
   - Commas between entries (but not after the last one)
   - All strings in double quotes
   - Use a JSON validator if unsure

## Validation

After updating `.mcp.json`, validate it:

```bash
# Check JSON syntax
cat ~/.mcp.json | python3 -m json.tool
```

## Alternative: Using Relative Paths

If your MCP client runs from the hunter3 directory:

```json
{
  "mcpServers": {
    "brave-search": {
      "command": "dist/mcp-brave",
      "args": [],
      "env": {
        "BRAVE_API_KEY": "YOUR_KEY_HERE"
      }
    }
  }
}
```

However, absolute paths are more reliable.

## Testing Configuration

After updating and restarting your MCP client:

1. Check if the server started:
   ```bash
   tail -f ~/.hunter3/logs/mcp-brave.log
   ```

2. Look for: "Starting Brave Search MCP server"

3. Try using the tools:
   - `brave_web_search` with query: "test"
   - `brave_news_search` with query: "latest news"

## Troubleshooting

**Server doesn't start:**
- Check the path is correct
- Verify BRAVE_API_KEY is set
- Check file permissions: `chmod +x dist/mcp-brave`

**Tools not visible:**
- Restart MCP client completely
- Check .mcp.json syntax
- Look at client logs for connection errors

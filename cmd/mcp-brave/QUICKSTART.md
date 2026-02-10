# Quick Start Guide - MCP Brave Search Plugin

## 5-Minute Setup

### Step 1: Get API Key (2 minutes)
1. Go to https://brave.com/search/api/
2. Sign up for a free account
3. Get your API key (starts with "BSA")

### Step 2: Install Plugin (2 minutes)
```bash
# Navigate to Hunter3 project
cd ~/go/src/github.com/soyeahso/hunter3

# Copy the plugin
cp -r /path/to/mcp-brave cmd/mcp-brave

# Build it
make all
# Or just: make dist/mcp-brave
```

### Step 3: Configure (1 minute)
Edit your `.mcp.json` (usually in `~/.claude/.mcp.json` or `~/.mcp.json`):

```json
{
  "mcpServers": {
    "brave-search": {
      "command": "/home/YOUR_USERNAME/go/src/github.com/soyeahso/hunter3/dist/mcp-brave",
      "args": [],
      "env": {
        "BRAVE_API_KEY": "BSA_YOUR_KEY_HERE"
      }
    }
  }
}
```

**Replace:**
- `YOUR_USERNAME` with your actual username
- `BSA_YOUR_KEY_HERE` with your actual Brave API key

### Step 4: Restart & Test
1. Restart Claude Desktop (or your MCP client)
2. Try: "Search for 'Model Context Protocol' using brave_web_search"
3. Try: "Find news about 'AI' using brave_news_search"

## Verify It Works

```bash
# Check the logs
tail -f ~/.hunter3/logs/mcp-brave.log
```

Should see: `Starting Brave Search MCP server`

## Common Issues

| Problem | Solution |
|---------|----------|
| Server won't start | Check BRAVE_API_KEY is set in .mcp.json |
| Tools not showing | Restart MCP client completely |
| "API error 401" | Invalid API key - check it's correct |
| "command not found" | Use absolute path in .mcp.json |

## What You Get

### brave_web_search
```
Parameters:
  - query: "your search terms"
  - count: 10 (optional, 1-20)
  - country: "us" (optional)

Example: Search for "golang tutorials" count 5
```

### brave_news_search
```
Parameters:
  - query: "news topic"
  - count: 10 (optional, 1-20)
  - country: "us" (optional)

Example: Find recent news about "space exploration"
```

## That's It!

You now have web and news search capabilities in your MCP client powered by Brave Search.

---

For detailed documentation, see:
- **README.md** - Full feature documentation
- **INSTALL.md** - Detailed installation steps
- **mcp-json-update.md** - .mcp.json configuration help

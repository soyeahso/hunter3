# MCP Brave Search Plugin - Installation Checklist

## Pre-Installation

- [ ] Read QUICKSTART.md for overview
- [ ] Have access to Hunter3 project directory
- [ ] Have Go 1.25+ installed
- [ ] Have `make` installed

## Get API Key

- [ ] Visit https://brave.com/search/api/
- [ ] Create account or sign in
- [ ] Subscribe to API (free tier available)
- [ ] Copy API key (starts with "BSA")
- [ ] Keep API key secure (don't commit to git)

## Installation Steps

### 1. Copy Files
- [ ] Navigate to Hunter3 project root
- [ ] Copy `mcp-brave/` directory to `cmd/mcp-brave/`
- [ ] Verify files copied: `ls cmd/mcp-brave/`
- [ ] Should see: main.go, go.mod, README.md, etc.

### 2. Update Makefile
- [ ] Open main `Makefile` in editor
- [ ] Add build target for `dist/mcp-brave`
- [ ] Add `dist/mcp-brave` to `all:` target
- [ ] Reference `Makefile.include` for exact syntax

### 3. Build Plugin
- [ ] Run `make all` from project root
- [ ] Verify binary created: `ls dist/mcp-brave`
- [ ] Check binary permissions: `ls -la dist/mcp-brave`
- [ ] Should be executable (-rwxr-xr-x)

### 4. Configure .mcp.json
- [ ] Find your `.mcp.json` file location
  - Common: `~/.claude/.mcp.json` or `~/.mcp.json`
- [ ] Backup existing `.mcp.json` (if exists)
- [ ] Open `.mcp.json` in editor
- [ ] Add brave-search server configuration
- [ ] Use absolute path to binary
- [ ] Add BRAVE_API_KEY to env section
- [ ] Validate JSON syntax
- [ ] Save file

### 5. Verify Configuration
- [ ] Check path is correct: `ls /path/from/mcp.json`
- [ ] Verify API key is set (no placeholder text)
- [ ] Check JSON is valid: `cat ~/.mcp.json | python3 -m json.tool`
- [ ] No syntax errors should appear

## Testing

### 1. Restart MCP Client
- [ ] Close MCP client completely (e.g., Claude Desktop)
- [ ] Wait 5 seconds
- [ ] Restart MCP client

### 2. Check Logs
- [ ] Open terminal
- [ ] Run: `tail -f ~/.hunter3/logs/mcp-brave.log`
- [ ] Look for: "Starting Brave Search MCP server"
- [ ] Should see no error messages

### 3. Test Web Search
- [ ] In MCP client, try: "Search for 'test' using brave_web_search"
- [ ] Should return formatted results
- [ ] Results should include titles, URLs, descriptions

### 4. Test News Search
- [ ] Try: "Find news about 'technology' using brave_news_search"
- [ ] Should return news articles
- [ ] Results should include sources and timestamps

## Troubleshooting

### If Server Doesn't Start
- [ ] Check logs: `cat ~/.hunter3/logs/mcp-brave.log`
- [ ] Verify BRAVE_API_KEY in .mcp.json
- [ ] Check binary path is correct and absolute
- [ ] Verify binary has execute permissions
- [ ] Try rebuilding: `make clean && make all`

### If Tools Don't Appear
- [ ] Completely restart MCP client (not just reload)
- [ ] Check .mcp.json syntax (use JSON validator)
- [ ] Verify server shows in client logs
- [ ] Check MCP client configuration path

### If API Errors Occur
- [ ] Verify API key is valid (check Brave dashboard)
- [ ] Check you haven't exceeded rate limits
- [ ] Test API key with curl:
  ```bash
  curl -H "X-Subscription-Token: YOUR_KEY" \
    "https://api.search.brave.com/res/v1/web/search?q=test"
  ```

### If Results Are Empty
- [ ] Try different search queries
- [ ] Check network connectivity
- [ ] Verify Brave API status
- [ ] Look for error messages in logs

## Post-Installation

### Documentation
- [ ] Bookmark important docs:
  - QUICKSTART.md - Quick reference
  - README.md - Full documentation
  - EXAMPLES.md - Usage examples
  - mcp-json-update.md - Config help

### Optimization
- [ ] Note API rate limits for your tier
- [ ] Consider upgrading if needed
- [ ] Monitor usage in Brave dashboard

### Maintenance
- [ ] Add to your update routine
- [ ] Keep API key secure
- [ ] Check logs occasionally: `~/.hunter3/logs/mcp-brave.log`
- [ ] Update when new versions available

## Success Criteria

✅ All items checked above
✅ Server starts without errors
✅ Tools appear in MCP client
✅ Web search returns results
✅ News search returns results
✅ No errors in logs

## Quick Reference Commands

```bash
# Build
make dist/mcp-brave

# Check logs
tail -f ~/.hunter3/logs/mcp-brave.log

# Verify binary
ls -la dist/mcp-brave

# Test API key
curl -H "X-Subscription-Token: YOUR_KEY" \
  "https://api.search.brave.com/res/v1/web/search?q=test&count=1"

# Validate JSON
cat ~/.mcp.json | python3 -m json.tool

# Find .mcp.json
find ~ -name ".mcp.json" 2>/dev/null
```

## Support

If you encounter issues not covered by this checklist:

1. **Check Documentation:**
   - INSTALL.md - Detailed installation
   - mcp-json-update.md - Configuration help
   - EXAMPLES.md - Usage examples

2. **Check Logs:**
   - Server: `~/.hunter3/logs/mcp-brave.log`
   - Client: (varies by MCP client)

3. **Verify Externals:**
   - Brave API status
   - Network connectivity
   - API key validity

4. **Test Components:**
   - Build: `make dist/mcp-brave`
   - Config: JSON validation
   - API: curl test
   - Binary: Execute directly

---

**Last Updated:** February 2026
**Version:** 1.0.0

🎉 Congratulations on completing the installation!

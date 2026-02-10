# MCP Brave Search Plugin - Files Overview

## Complete File Structure

```
mcp-brave/
├── main.go                  # Main server implementation
├── go.mod                   # Go module dependencies
├── .mcp.json.example        # Example MCP configuration
├── Makefile.include         # Makefile snippet for integration
├── README.md                # Main documentation
├── INSTALL.md               # Installation guide
├── QUICKSTART.md            # 5-minute quick start
├── SUMMARY.md               # Project summary
├── mcp-json-update.md       # .mcp.json configuration help
└── FILES_OVERVIEW.md        # This file
```

## File Purposes

### Core Implementation
- **main.go** (330 lines)
  - MCP server implementation
  - Two tools: `brave_web_search` and `brave_news_search`
  - Brave Search API integration
  - Logging to `~/.hunter3/logs/mcp-brave.log`
  - Environment variable: `BRAVE_API_KEY`

- **go.mod** (10 lines)
  - Go module definition
  - Dependencies: `github.com/mark3labs/mcp-go`

### Configuration
- **.mcp.json.example** (JSON)
  - Example configuration showing BRAVE_API_KEY setup
  - Template for users to follow

- **Makefile.include** (Makefile)
  - Build rules for integration into main project
  - Shows how to update the main Makefile

### Documentation

#### Quick Start
- **QUICKSTART.md**
  - 5-minute setup guide
  - Essential steps only
  - Common issues table
  - Best for: New users wanting to get started fast

#### Installation
- **INSTALL.md**
  - Step-by-step installation instructions
  - Integration into Hunter3 project
  - Troubleshooting section
  - Testing instructions
  - Best for: First-time installation

#### Configuration Help
- **mcp-json-update.md**
  - Detailed .mcp.json configuration guide
  - Path resolution help
  - JSON syntax validation
  - Alternative configurations
  - Best for: Configuration issues

#### Main Documentation
- **README.md**
  - Complete feature overview
  - Tool specifications with examples
  - Setup instructions
  - API rate limit information
  - Best for: Understanding features and capabilities

#### Project Summary
- **SUMMARY.md**
  - What was created
  - Key features
  - Integration steps
  - Technical details
  - Best for: Project overview and technical reference

#### This File
- **FILES_OVERVIEW.md**
  - File structure explanation
  - Purpose of each file
  - Reading order recommendations

## Reading Order

### For Quick Setup (5 min):
1. QUICKSTART.md

### For First-Time Installation (15 min):
1. QUICKSTART.md
2. INSTALL.md
3. mcp-json-update.md (if issues)

### For Understanding the Project (30 min):
1. SUMMARY.md
2. README.md
3. main.go (code review)

### For Configuration Issues:
1. mcp-json-update.md
2. INSTALL.md (troubleshooting section)

### For Development/Modification:
1. main.go
2. README.md (features)
3. SUMMARY.md (technical details)

## Integration Checklist

- [ ] Copy `mcp-brave/` to `cmd/mcp-brave/` in Hunter3 project
- [ ] Update main Makefile with build rules from `Makefile.include`
- [ ] Run `make all` to build
- [ ] Get Brave Search API key from https://brave.com/search/api/
- [ ] Update `.mcp.json` with absolute path and BRAVE_API_KEY
- [ ] Restart MCP client
- [ ] Check logs: `tail -f ~/.hunter3/logs/mcp-brave.log`
- [ ] Test `brave_web_search` tool
- [ ] Test `brave_news_search` tool

## Key Code Sections in main.go

```
Lines 1-20:    Package imports
Lines 21-41:   Type definitions (BraveSearchResponse)
Lines 43-86:   Main function (setup, logging, server init)
Lines 88-142:  brave_web_search tool definition
Lines 144-198: brave_news_search tool definition
Lines 200-239: braveWebSearchHandler
Lines 241-280: braveNewsSearchHandler
Lines 282-326: performBraveSearch (core API logic)
Lines 328-370: formatResults (output formatting)
```

## Key Configuration Values

### Environment Variables
- `BRAVE_API_KEY` - Required, your Brave Search API key

### Tool Parameters
- `query` - Required string, search terms
- `count` - Optional number, 1-20 results (default: 10)
- `country` - Optional string, country code (default: "us")

### Paths
- Binary: `dist/mcp-brave`
- Logs: `~/.hunter3/logs/mcp-brave.log`
- Config: `.mcp.json` (location varies by client)

## Development Notes

### Important MCP Learnings (from MEMORY.md)
- Don't use `omitempty` on capability fields that must always be present
- Server must advertise `"tools":{}` in capabilities
- Image data expects raw base64, not data URIs
- Custom servers build to `dist/`

### Code Conventions
- Logging goes to `~/.hunter3/logs/`
- Uses `github.com/mark3labs/mcp-go` SDK
- Error handling returns `mcp.NewToolResultError()`
- Success returns `mcp.NewToolResultText()`

### Testing During Development
```bash
# Rebuild
make dist/mcp-brave

# Check logs
tail -f ~/.hunter3/logs/mcp-brave.log

# Manual test (if needed)
echo '{"method":"tools/call","params":{"name":"brave_web_search","arguments":{"query":"test"}}}' | dist/mcp-brave
```

## Support Resources

- Brave Search API: https://brave.com/search/api/
- MCP Go SDK: https://github.com/mark3labs/mcp-go
- Hunter3 Project: (internal)
- Logs: `~/.hunter3/logs/mcp-brave.log`

---

**Created**: February 2026
**Version**: 1.0.0
**Go Version**: 1.25
**MCP SDK**: github.com/mark3labs/mcp-go v0.7.0

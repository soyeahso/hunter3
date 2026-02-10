# MCP OpenClaw Skills - Quick Start Guide

Get up and running with the OpenClaw Skills MCP plugin in 5 minutes.

## Prerequisites

- Go 1.25+ installed
- Hunter3 project cloned
- OpenClaw repository cloned (or skills directory available)

## Step 1: Get the OpenClaw Skills

### Option A: Clone OpenClaw Repository

```bash
git clone https://github.com/openclaw/openclaw.git ~/openclaw
```

The skills will be in `~/openclaw/skills/`

### Option B: Use Existing OpenClaw Installation

If you have OpenClaw installed, skills are typically in:
- `~/.openclaw/skills/` (user skills)
- Or wherever you've configured OpenClaw

## Step 2: Build the Plugin

From the hunter3 project root:

```bash
make all
```

This builds `dist/mcp-openclaw-skills`

## Step 3: Configure MCP

Add to your `.mcp.json` (usually in `~/.config/mcp/` or project root):

```json
{
  "mcpServers": {
    "openclaw-skills": {
      "command": "/full/path/to/hunter3/dist/mcp-openclaw-skills",
      "args": [],
      "env": {
        "OPENCLAW_SKILLS_PATH": "/Users/yourusername/openclaw/skills"
      }
    }
  }
}
```

**Important:** Use absolute paths! Replace:
- `/full/path/to/hunter3/` with your hunter3 project path
- `/Users/yourusername/openclaw/skills` with your actual skills path

## Step 4: Restart Your MCP Client

If using Claude Desktop:
1. Quit Claude Desktop completely
2. Start Claude Desktop again
3. The plugin will load automatically

## Step 5: Test It!

In your MCP client, try:

```
List all available OpenClaw skills
```

Or:

```
Show me the documentation for the github skill
```

## Verify Installation

### Check Logs

```bash
tail -f ~/.hunter3/logs/mcp-openclaw-skills.log
```

You should see:
```
[mcp-openclaw-skills] MCP OpenClaw Skills server starting...
[mcp-openclaw-skills] Using skills path: /Users/you/openclaw/skills
[mcp-openclaw-skills] Loaded skill: github
[mcp-openclaw-skills] Loaded skill: weather
...
[mcp-openclaw-skills] Loaded 50 skills
```

### Test Manual Invocation

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | dist/mcp-openclaw-skills
```

Should return:
```json
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"resources":{},"tools":{}},"serverInfo":{"name":"openclaw-skills","version":"1.0.0"}}}
```

## Common Issues

### Skills Not Found

**Error:** `Warning: Failed to load skills: skills directory does not exist`

**Solution:** Check your `OPENCLAW_SKILLS_PATH`:
```bash
ls -la ~/openclaw/skills/
# Should show directories with SKILL.md files
```

### Permission Denied

**Error:** `Failed to open log file: permission denied`

**Solution:** Ensure `~/.hunter3/logs/` exists and is writable:
```bash
mkdir -p ~/.hunter3/logs
chmod 755 ~/.hunter3/logs
```

### Command Not Found

**Error:** MCP client shows "openclaw-skills not available"

**Solution:** 
1. Verify build succeeded: `ls -lh dist/mcp-openclaw-skills`
2. Use absolute path in `.mcp.json`
3. Restart MCP client completely

## Quick Examples

### List Skills
```
Use list_skills to show all available OpenClaw skills
```

Expected: List of ~50 skills with emojis and descriptions

### Get Skill
```
Get the full documentation for the tmux skill
```

Expected: Full SKILL.md content for tmux

### Search Skills
```
Search for skills related to "github"
```

Expected: Matching skills (github, clawhub, etc.)

## Next Steps

- Read [EXAMPLES.md](./EXAMPLES.md) for detailed usage examples
- Check [README.md](./README.md) for full documentation
- Explore the OpenClaw skills directory to see all available skills

## Directory Structure Check

Your setup should look like:

```
hunter3/
├── dist/
│   └── mcp-openclaw-skills          # Built binary
├── cmd/
│   └── mcp-openclaw-skills/
│       ├── main.go
│       ├── README.md
│       └── ...
└── .mcp.json                         # Your config

~/.hunter3/
└── logs/
    └── mcp-openclaw-skills.log       # Runtime logs

~/openclaw/                            # Or wherever you cloned it
└── skills/
    ├── github/
    │   └── SKILL.md
    ├── weather/
    │   └── SKILL.md
    └── ...
```

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `OPENCLAW_SKILLS_PATH` | `~/.openclaw/skills` | Path to skills directory |

## Supported Platforms

- macOS (tested)
- Linux (tested)
- Windows with WSL (should work)

## Getting Help

If you encounter issues:

1. Check logs: `tail -f ~/.hunter3/logs/mcp-openclaw-skills.log`
2. Verify skills path: `ls $OPENCLAW_SKILLS_PATH`
3. Test manual invocation: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | dist/mcp-openclaw-skills`
4. Check MCP client logs (e.g., Claude Desktop logs)

## Success Criteria

You know it's working when:

✅ Log shows "Loaded X skills" where X > 0
✅ `list_skills` returns a list of skills
✅ `get_skill` returns full skill documentation
✅ No errors in `~/.hunter3/logs/mcp-openclaw-skills.log`

Happy skill browsing! 🦞

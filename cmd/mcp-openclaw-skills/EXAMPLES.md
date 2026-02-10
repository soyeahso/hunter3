# MCP OpenClaw Skills - Usage Examples

This document provides practical examples of using the MCP OpenClaw Skills plugin.

## Setup Example

1. **Clone OpenClaw and locate skills:**
```bash
git clone https://github.com/openclaw/openclaw.git ~/openclaw
ls ~/openclaw/skills
```

2. **Configure the MCP server:**
```json
{
  "mcpServers": {
    "openclaw-skills": {
      "command": "dist/mcp-openclaw-skills",
      "args": [],
      "env": {
        "OPENCLAW_SKILLS_PATH": "/Users/yourusername/openclaw/skills"
      }
    }
  }
}
```

3. **Build and start:**
```bash
make all
# Restart your MCP client (e.g., Claude Desktop)
```

## Tool Usage Examples

### Example 1: List All Available Skills

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_skills",
    "arguments": {}
  }
}
```

**Response:**
```
Available OpenClaw Skills (50 total):

🐙 **github**: Interact with GitHub using the `gh` CLI. Use `gh issue`, `gh pr`, `gh run`, and `gh api` for issues, PRs, CI runs, and advanced queries.
🌤️ **weather**: Get current weather and forecasts (no API key required).
🧵 **tmux**: Remote-control tmux sessions for interactive CLIs by sending keystrokes and scraping pane output.
🧩 **coding-agent**: Run Codex CLI, Claude Code, OpenCode, or Pi Coding Agent via background process for programmatic control.
💎 **obsidian**: Work with Obsidian vaults (plain Markdown notes) and automate via obsidian-cli.
...
```

### Example 2: Get Specific Skill Documentation

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "get_skill",
    "arguments": {
      "skill_name": "coding-agent"
    }
  }
}
```

**Response:**
Returns the full SKILL.md content for the coding-agent skill, including:
- YAML frontmatter with metadata
- Complete Markdown documentation
- Usage examples
- Command reference
- Tips and best practices

### Example 3: Search Skills by Keyword

**Request (search in names/descriptions only):**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "search_skills",
    "arguments": {
      "query": "github",
      "search_content": "false"
    }
  }
}
```

**Request (search in full content):**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "search_skills",
    "arguments": {
      "query": "pull request",
      "search_content": "true"
    }
  }
}
```

**Response:**
```
Found 2 skills matching 'pull request':

🐙 **github**: Interact with GitHub using the `gh` CLI...
🔧 **clawhub**: Extended GitHub automation and workflow tools...
```

## Resource Usage Examples

### Example 4: Read Skill as Resource

**List available resources:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "resources/list",
  "params": {}
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "resources": [
      {
        "uri": "openclaw://skill/github",
        "name": "OpenClaw Skill: github",
        "description": "Interact with GitHub using the `gh` CLI...",
        "mimeType": "text/markdown"
      },
      {
        "uri": "openclaw://skill/weather",
        "name": "OpenClaw Skill: weather",
        "description": "Get current weather and forecasts...",
        "mimeType": "text/markdown"
      }
    ]
  }
}
```

**Read specific resource:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "resources/read",
  "params": {
    "uri": "openclaw://skill/tmux"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "result": {
    "contents": [
      {
        "uri": "openclaw://skill/tmux",
        "mimeType": "text/markdown",
        "text": "---\nname: tmux\ndescription: Remote-control tmux sessions...\n---\n\n# tmux Skill...\n..."
      }
    ]
  }
}
```

## AI Assistant Usage Examples

### Example 5: Conversational Queries

**User:** "What OpenClaw skills are available?"

**Assistant uses:** `list_skills` tool

---

**User:** "Show me how to use the GitHub skill"

**Assistant uses:** `get_skill` with `skill_name: "github"`

---

**User:** "Find skills related to coding"

**Assistant uses:** `search_skills` with `query: "coding"` and `search_content: "true"`

---

**User:** "What skills can help me work with tmux?"

**Assistant uses:** `search_skills` with `query: "tmux"`

## Integration Patterns

### Pattern 1: Skill Discovery Workflow

1. User asks about available tools
2. Assistant calls `list_skills` to get overview
3. User asks about specific category
4. Assistant calls `search_skills` with relevant query
5. User requests details
6. Assistant calls `get_skill` for full documentation

### Pattern 2: Task-Based Lookup

1. User describes a task (e.g., "I need to automate GitHub")
2. Assistant calls `search_skills` with task keywords
3. Assistant presents relevant skills
4. Assistant calls `get_skill` for chosen skill
5. Assistant provides guidance based on skill documentation

### Pattern 3: Resource-Based Access

1. Assistant discovers skill URIs via `resources/list`
2. Assistant reads skills as resources via `resources/read`
3. Assistant uses skill content to inform responses
4. Assistant caches commonly-used skills

## Error Handling Examples

### Example 6: Skill Not Found

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "get_skill",
    "arguments": {
      "skill_name": "nonexistent-skill"
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Skill 'nonexistent-skill' not found"
      }
    ],
    "isError": true
  }
}
```

### Example 7: Invalid Resource URI

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "resources/read",
  "params": {
    "uri": "invalid://uri/format"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "error": {
    "code": -32602,
    "message": "Invalid URI",
    "data": "URI must start with openclaw://skill/"
  }
}
```

## Advanced Usage

### Custom Skills Directory

You can point to a custom skills directory:

```json
{
  "mcpServers": {
    "openclaw-skills": {
      "command": "dist/mcp-openclaw-skills",
      "args": [],
      "env": {
        "OPENCLAW_SKILLS_PATH": "/custom/path/to/skills"
      }
    }
  }
}
```

### Multiple Skill Sources

Run multiple instances with different skill directories:

```json
{
  "mcpServers": {
    "openclaw-official": {
      "command": "dist/mcp-openclaw-skills",
      "args": [],
      "env": {
        "OPENCLAW_SKILLS_PATH": "~/openclaw/skills"
      }
    },
    "openclaw-custom": {
      "command": "dist/mcp-openclaw-skills",
      "args": [],
      "env": {
        "OPENCLAW_SKILLS_PATH": "~/.openclaw/custom-skills"
      }
    }
  }
}
```

## Testing the Plugin

### Manual Testing with MCP Inspector

1. Start the MCP server:
```bash
dist/mcp-openclaw-skills
```

2. Send JSON-RPC requests via stdin:
```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_skills","arguments":{}}}
```

3. Check logs:
```bash
tail -f ~/.hunter3/logs/mcp-openclaw-skills.log
```

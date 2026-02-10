# MCP OpenClaw Skills Plugin

An MCP (Model Context Protocol) server that provides access to OpenClaw skills documentation and metadata. This plugin enables AI assistants to discover, read, and search through OpenClaw skills stored in SKILL.md files.

## Overview

OpenClaw uses a skill-based system where each skill is documented in a `SKILL.md` file with YAML frontmatter containing metadata and Markdown documentation. This MCP plugin makes those skills accessible through the MCP protocol, allowing AI assistants to:

- List all available skills
- Read full skill documentation
- Search skills by name, description, or content
- Access skills as MCP resources

## Features

### Tools Provided

#### `list_skills`
List all available OpenClaw skills with their names, descriptions, and emoji icons.

**Parameters:** None

**Example Output:**
```
Available OpenClaw Skills (50 total):

🐙 **github**: Interact with GitHub using the `gh` CLI...
🌤️ **weather**: Get current weather and forecasts...
🧵 **tmux**: Remote-control tmux sessions for interactive CLIs...
```

#### `get_skill`
Get the full content and documentation for a specific OpenClaw skill.

**Parameters:**
- `skill_name` (required): The name of the skill to retrieve

**Example:**
```json
{
  "skill_name": "github"
}
```

#### `search_skills`
Search for skills by keyword in their name, description, or content.

**Parameters:**
- `query` (required): Search query to match against skill names, descriptions, and content
- `search_content` (optional): Whether to search in skill content ("true"/"false", default: "false")

**Example:**
```json
{
  "query": "coding",
  "search_content": "true"
}
```

### Resources Provided

Each skill is exposed as an MCP resource with the URI format: `openclaw://skill/{skill_name}`

Resources can be read through the standard MCP `resources/read` method.

## Setup

### 1. Clone OpenClaw Skills

First, clone the OpenClaw repository to access the skills:

```bash
git clone https://github.com/openclaw/openclaw.git
```

Or if you have OpenClaw installed, ensure your skills are in `~/.openclaw/skills/`

### 2. Build the Plugin

From the hunter3 project root:

```bash
make all
```

This will build the binary to `dist/mcp-openclaw-skills`

### 3. Configure MCP Server

Add the OpenClaw Skills MCP server to your `.mcp.json` configuration:

```json
{
  "mcpServers": {
    "openclaw-skills": {
      "command": "dist/mcp-openclaw-skills",
      "args": [],
      "env": {
        "OPENCLAW_SKILLS_PATH": "/path/to/openclaw/skills"
      }
    }
  }
}
```

**Configuration Options:**

- `OPENCLAW_SKILLS_PATH`: Path to the OpenClaw skills directory (default: `~/.openclaw/skills`)

### 4. Restart Your MCP Client

Restart your MCP client (e.g., Claude Desktop) to load the new server.

## OpenClaw Skills Format

Each OpenClaw skill is stored in a directory with a `SKILL.md` file containing:

### YAML Frontmatter
```yaml
---
name: skill-name
description: Brief description of the skill
homepage: https://optional-homepage.com
metadata:
  openclaw:
    emoji: "🔧"
    os: ["darwin", "linux"]
    requires:
      bins: ["command1", "command2"]
    install:
      - id: brew
        kind: brew
        formula: package-name
        bins: ["command"]
        label: "Install via Homebrew"
---
```

### Markdown Content

After the frontmatter, the rest of the file contains rich Markdown documentation with:
- Usage examples
- Command reference
- Tips and best practices
- Code snippets

## Example Usage

Once configured, you can use the tools in your MCP client:

**List all skills:**
```
Use the list_skills tool to show me all available OpenClaw skills
```

**Get a specific skill:**
```
Show me the documentation for the github skill using get_skill
```

**Search for skills:**
```
Search for skills related to "coding agents" using search_skills
```

**Read as resource:**
```
Read the resource openclaw://skill/tmux
```

## Logging

Logs are written to `~/.hunter3/logs/mcp-openclaw-skills.log`

## Skills Directory Structure

Expected directory structure:
```
~/.openclaw/skills/
├── github/
│   └── SKILL.md
├── weather/
│   └── SKILL.md
├── tmux/
│   └── SKILL.md
└── coding-agent/
    └── SKILL.md
```

Each skill directory should contain a `SKILL.md` file with YAML frontmatter and Markdown documentation.

## Integration with Hunter3

This plugin follows the Hunter3 MCP server conventions:
- Built to `dist/` directory
- Logs to `~/.hunter3/logs/`
- Uses standard MCP protocol (2024-11-05)
- Implements both tools and resources capabilities
- Properly handles YAML frontmatter parsing

## Dependencies

- Go 1.25+
- gopkg.in/yaml.v3 (for YAML frontmatter parsing)

## Development

The plugin loads skills on startup by:
1. Reading the skills directory (from `OPENCLAW_SKILLS_PATH` or default)
2. Finding all subdirectories with `SKILL.md` files
3. Parsing YAML frontmatter to extract metadata
4. Storing full content for later retrieval

Skills are cached in memory for fast access during MCP operations.

## Troubleshooting

**No skills found:**
- Check that `OPENCLAW_SKILLS_PATH` points to the correct directory
- Ensure each skill directory contains a `SKILL.md` file
- Check logs at `~/.hunter3/logs/mcp-openclaw-skills.log`

**Metadata parsing errors:**
- Ensure YAML frontmatter is properly formatted
- Check that frontmatter starts and ends with `---`
- Validate YAML syntax

## Future Enhancements

Potential improvements:
- Watch for skill file changes and reload automatically
- Support for skill validation
- Extract and expose dependency information
- Integration with OpenClaw's installer metadata
- Skill creation/editing capabilities

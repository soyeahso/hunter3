# hunter3

A personal AI assistant that bridges messaging channels with LLM providers and
external tools. It manages conversations and sessions through a configurable
WebSocket-based gateway.

hunter3 is able to be self-modifying and reprogrammable on the fly. Simply
prompt it with the changes you want and it will modify and rebuild itself,
automatically reconnecting to the IRC server. If you break it, just drop down
into your terminal and fix it using Claude or any number of other CLI tools.

If you require communication methods other than IRC, consider installing and
configuring a bridge. Otherwise, just use IRC. It's simple and well-understood.
There are dozens of trustworthy, open source IRC clients and servers that are
known to be secure and not leak your data.

## Architecture

```
 Messaging Channels          Gateway               LLM Providers
┌──────────────┐        ┌──────────────┐        ┌──────────────┐
│              │        │   Router     │        │  Claude CLI  │
│              │        │      │       │        ├──────────────┤
│     IRC      │◄──────►│   Agent      │◄──────►│  Gemini CLI  │
│              │        │      │       │        ├──────────────┤
│              │        │   Sessions   │        │  Ollama      │
└──────────────┘        └──────┬───────┘        └──────────────┘
                               │
                 ┌─────────────┼─────────────┐
                 │             │             │
          ┌──────┴──────┐ ┌───┴────┐ ┌──────┴──────┐
          │    MCP      │ │ SQLite │ │  Plugins    │
          │  Servers    │ │   DB   │ │             │
          └─────────────┘ └────────┘ └─────────────┘
```

Messages flow from a channel through the router to an agent, which calls an LLM
provider and returns the response.

LLM integration wraps existing CLI tools rather than implementing API clients
directly. This reuses each CLI's authentication, caching, and rate limiting.

## Requirements

- Go 1.24+
- One or more LLM CLI tools installed (e.g. `claude`, `gemini`, `ollama`).
Please note only claude is tested/supported currently.

## Building

```bash
make build        # Build binary to dist/hunter3
make dev          # Quick dev build without version info
make test         # Run tests with race detection
make lint         # Run golangci-lint (+ go vet)
make check        # Run vet + tests
make clean        # Remove dist/
```

The build injects version, commit hash, and build date via ldflags.

## Usage

### Start the gateway

```bash
./dist/hunter3 gateway run [--port PORT] [--bind BIND] [--config CONFIG]
```

### Other commands

```bash
hunter3 config       # Manage configuration
hunter3 agent        # Agent management
hunter3 message      # Send messages
hunter3 status       # Check gateway status
hunter3 version      # Show version info
```

## Configuration

Configuration is stored in `~/.hunter3/config.yaml`. 

The database lives at `~/.hunter3/data/hunter3.db`.

### LLM Providers

Hunter3 supports two modes for LLM integration:

**1. CLI-based (default) - `cli: claude` or `cli: copilot`**

Uses installed CLI tools which manage authentication, caching, and rate limiting:

```yaml
cli: claude   # or "copilot", "gemini", "ollama"
```

Requires the corresponding CLI tool installed and in PATH.

**2. Direct API - `cli: none`**

Uses HTTP APIs directly with explicit authentication:

```yaml
cli: none
apiProvider: claude      # "claude", "gemini", or "ollama"
apiKey: sk-ant-...       # API key (not needed for ollama)
apiModel: claude-3-5-sonnet
# apiEndpoint: http://localhost:11434  # Optional: for custom ollama endpoint
```

### Full Configuration Example

```yaml
# LLM Provider configuration
cli: claude              # "claude" | "copilot" | "none"

# If cli: none, configure API access below
# apiProvider: claude
# apiKey: sk-ant-...
# apiModel: claude-3-5-sonnet
gateway:
  port: 18789
  bind: loopback          # loopback | lan | custom | tailnet
  auth:
    mode: token           # token | password

channels:
  irc:
    server: irc.example.com
    port: 6697
    nick: mybot
    channels:
      - "#general"
    useTLS: true
    sasl: true

agents:
  defaults:
    model: sonnet
    maxTokens: 4096
  list:
    - id: default
      default: true
      name: Assistant

session:
  scope: per-sender       # per-sender | global
  idleMinutes: 30
  store: sqlite           # sqlite | memory

logging:
  level: info
  consoleStyle: pretty    # pretty | compact | json

hooks:
  messageReceived:
    - command: "echo received"
      timeout: 5000
  gatewayStart:
    - command: "echo started"

memory:
  enabled: true
  searchMode: fts         # fts | embedding
```

## Plugins

The plugin system allows extending Hunter3 with custom functionality. Plugins have access to the event hook system for intercepting messages, agent runs, and gateway lifecycle events.

```go
type Plugin interface {
    ID() string
    Name() string
    Version() string
    Init(ctx context.Context, api plugin.API) error
    Close() error
}
```

Hook events: `message_received`, `message_sending`, `before_agent_run`, `after_agent_run`, `session_start`, `session_end`, `gateway_start`, `gateway_stop`.

## MCP Servers

Hunter3 includes 14 built-in MCP (Model Context Protocol) servers in `cmd/mcp-*`. Each is a standalone Go binary that communicates via JSON-RPC 2.0 over stdio. Binaries are built to `dist/` and log to `~/.hunter3/logs/`.

### mcp-brave -- Brave Search

Web and news search via the Brave Search API.

**Tools:** `brave_web_search`, `brave_news_search`

**Config:** `BRAVE_API_KEY` env var ([get key](https://brave.com/search/api/))

**Details:** [cmd/mcp-brave/README.md](cmd/mcp-brave/README.md)

### mcp-curl -- HTTP Client

Wraps the system `curl` command with full option support.

**Tools:** `curl`

**Config:** Requires `curl` in PATH

**Details:** [cmd/mcp-curl/README.md](cmd/mcp-curl/README.md)

### mcp-digitalocean -- DigitalOcean

Manage DigitalOcean droplets, SSH keys, networking, and tags via the official API.

**Tools:** `list_droplets`, `create_droplet`, `delete_droplet`, `power_on_droplet`, `power_off_droplet`, `reboot_droplet`, `shutdown_droplet`, `power_cycle_droplet`, `resize_droplet`, `snapshot_droplet`, `get_droplet`, `get_droplet_action`, `list_ssh_keys`, `create_ssh_key`, `delete_ssh_key`, `list_regions`, `list_sizes`, `list_images`, `list_tags`, `create_tag`, `delete_tag`, `tag_resources`, `untag_resources`, `get_account`

**Config:** `DIGITALOCEAN_TOKEN` env var

**Details:** [cmd/mcp-digitalocean/README.md](cmd/mcp-digitalocean/README.md)

### mcp-docker -- Docker

Manage containers, images, networks, volumes, and Compose projects via the Docker CLI.

**Tools:** `docker_ps`, `docker_run`, `docker_start`, `docker_stop`, `docker_restart`, `docker_rm`, `docker_exec`, `docker_logs`, `docker_inspect`, `docker_stats`, `docker_images`, `docker_pull`, `docker_push`, `docker_rmi`, `docker_build`, `docker_tag`, `docker_network_ls`, `docker_network_create`, `docker_network_rm`, `docker_network_connect`, `docker_network_disconnect`, `docker_volume_ls`, `docker_volume_create`, `docker_volume_rm`, `docker_volume_inspect`, `docker_compose_up`, `docker_compose_down`, `docker_compose_ps`, `docker_compose_logs`, `docker_info`, `docker_version`, `docker_system_df`, `docker_system_prune`

**Config:** Requires `docker` in PATH

**Details:** [cmd/mcp-docker/README.md](cmd/mcp-docker/README.md)

### mcp-fetch-website -- HTTP Fetch

Fetch web pages and APIs with SSRF protection. Supports image responses (base64).

**Tools:** `fetch`

**Config:** None

**Details:** [cmd/mcp-fetch-website/README.md](cmd/mcp-fetch-website/README.md)

### mcp-filesystem -- Filesystem

Sandboxed file operations restricted to specified allowed directories. Symlink-aware path validation.

**Tools:** `read_file`, `read_text_file`, `read_media_file`, `read_multiple_files`, `write_file`, `edit_file`, `create_directory`, `list_directory`, `list_directory_with_sizes`, `directory_tree`, `move_file`, `search_files`, `get_file_info`, `list_allowed_directories`

**Config:** Pass allowed directories as CLI args

**Details:** [cmd/mcp-filesystem/README.md](cmd/mcp-filesystem/README.md)

### mcp-gdrive -- Google Drive

File management on Google Drive via OAuth2: list, upload, download, share, search.

**Tools:** `list_files`, `get_file_info`, `download_file`, `upload_file`, `create_folder`, `delete_file`, `search_files`, `share_file`

**Config:** OAuth2 credentials at `~/.hunter3/gdrive-credentials.json` (or `GDRIVE_CREDENTIALS_FILE`)

**Details:** [cmd/mcp-gdrive/README.md](cmd/mcp-gdrive/README.md)

### mcp-gh -- GitHub CLI

Wraps the `gh` CLI for repos, issues, PRs, workflows, releases, gists, and raw API calls.

**Tools:** `gh_repo_view`, `gh_repo_clone`, `gh_repo_create`, `gh_repo_fork`, `gh_repo_list`, `gh_issue_list`, `gh_issue_view`, `gh_issue_create`, `gh_issue_close`, `gh_issue_reopen`, `gh_pr_list`, `gh_pr_view`, `gh_pr_create`, `gh_pr_checkout`, `gh_pr_merge`, `gh_pr_close`, `gh_pr_review`, `gh_pr_diff`, `gh_run_list`, `gh_run_view`, `gh_run_rerun`, `gh_workflow_list`, `gh_workflow_run`, `gh_release_list`, `gh_release_view`, `gh_release_create`, `gh_release_download`, `gh_gist_list`, `gh_gist_view`, `gh_gist_create`, `gh_auth_status`, `gh_auth_login`, `gh_search_repos`, `gh_search_issues`, `gh_api`

**Config:** Requires `gh` in PATH and `gh auth login`. Optional `HUNTER3_GH_ALLOWED_PATHS` for path restriction.

**Details:** [cmd/mcp-gh/README.md](cmd/mcp-gh/README.md)

### mcp-git -- Git

Wraps the `git` CLI with 25+ commands. Sanitizes dangerous flags and restricts paths.

**Tools:** `git_status`, `git_log`, `git_diff`, `git_show`, `git_blame`, `git_add`, `git_commit`, `git_reset`, `git_restore`, `git_rm`, `git_mv`, `git_branch`, `git_checkout`, `git_switch`, `git_merge`, `git_rebase`, `git_cherry_pick`, `git_remote`, `git_fetch`, `git_pull`, `git_push`, `git_clone`, `git_tag`, `git_stash`, `git_clean`, `git_init`, `git_rev_parse`, `git_ls_files`

**Config:** Optional `HUNTER3_GIT_ALLOWED_PATHS` (defaults to `$HOME`)

### mcp-gmail -- Gmail

Gmail integration via OAuth2: read, send (with attachments), and search emails.

**Tools:** `list_messages`, `read_message`, `send_message`, `search_messages`

**Config:** OAuth2 credentials at `~/.hunter3/gmail-credentials.json` (or `GMAIL_CREDENTIALS_FILE`)

**Details:** [cmd/mcp-gmail/README.md](cmd/mcp-gmail/README.md)

### mcp-imail -- iCloud Mail

iCloud Mail via IMAP/SMTP. Simpler setup than Gmail (no OAuth -- uses App-Specific Passwords).

**Tools:** `list_messages`, `read_message`, `send_message`, `search_messages`, `list_mailboxes`

**Config:** `ICLOUD_EMAIL`/`ICLOUD_PASSWORD` env vars or `~/.hunter3/icloud-mail.json`

**Details:** [cmd/mcp-imail/README.md](cmd/mcp-imail/README.md)

### mcp-make -- Build Tool

Runs `make` targets in the project directory.

**Tools:** `build`

**Config:** Optional `HUNTER3_PROJECT_ROOT` (auto-detects by looking for Makefile)

### mcp-openclaw-skills -- OpenClaw Skills

Read and search OpenClaw SKILL.md files. Also exposes skills as MCP resources (`openclaw://skill/{name}`).

**Tools:** `list_skills`, `get_skill`, `search_skills`

**Config:** Optional `OPENCLAW_SKILLS_PATH` (defaults to `~/.openclaw/skills`)

**Details:** [cmd/mcp-openclaw-skills/README.md](cmd/mcp-openclaw-skills/README.md)

### mcp-weather -- NOAA Weather

US weather data from the National Weather Service API.

**Tools:** `get_forecast`, `get_alerts`, `get_observation`

**Config:** None (uses public NWS API)

## Environment Variables

| Variable | Description |
|---|---|
| `HUNTER3_IRCV3_MULTILINE` | Set to any non-empty value to enable IRCv3 `draft/multiline` support. When enabled, multi-line responses are sent as a single batch instead of being split into individual messages. The server must also advertise the capability. |
| `BRAVE_API_KEY` | API key for the Brave Search MCP server. |
| `DIGITALOCEAN_TOKEN` | API token for the DigitalOcean MCP server. |
| `GMAIL_CREDENTIALS_FILE` | Custom path to Gmail OAuth2 credentials (default: `~/.hunter3/gmail-credentials.json`). |
| `GDRIVE_CREDENTIALS_FILE` | Custom path to Google Drive OAuth2 credentials (default: `~/.hunter3/gdrive-credentials.json`). |
| `ICLOUD_EMAIL` | iCloud email address for the iCloud Mail MCP server. |
| `ICLOUD_PASSWORD` | App-Specific Password for the iCloud Mail MCP server. |
| `HUNTER3_GIT_ALLOWED_PATHS` | Comma-separated allowed directories for git operations (default: `$HOME`). |
| `HUNTER3_GH_ALLOWED_PATHS` | Comma-separated allowed directories for gh operations (default: `$HOME`). |
| `HUNTER3_PROJECT_ROOT` | Project root for the make MCP server (auto-detected if unset). |
| `OPENCLAW_SKILLS_PATH` | Path to OpenClaw skills directory (default: `~/.openclaw/skills`). |

## Project Structure

```
cmd/hunter3/       CLI entry point
internal/
  agent/                 Agent runner, tool execution loop, session management
  channel/               Channel interface + implementations
    irc/                 IRC channel implementation
  cli/                   Cobra command definitions
  config/                YAML config loading, validation, defaults
  domain/                Core domain types (Agent, Channel, Message, Session)
  gateway/               WebSocket server, auth, RPC protocol
  hooks/                 Event hook system
  llm/                   LLM provider abstraction (Claude, Gemini, Ollama)
  logging/               Structured logging (zerolog)
  plugin/                Plugin registry and lifecycle
  routing/               Message routing between channels and agents
  store/                 SQLite persistence + migrations
  version/               Build-time version info
```

## Key Design Decisions

- **CLI-wrapped LLM providers** -- Instead of HTTP API clients, LLM providers are wrapped CLI tools (`claude`, `gemini`, `ollama`). This reuses their auth flows, caching, and stays current with API changes.
- **Pure-Go SQLite** -- Uses `modernc.org/sqlite` for zero-CGO, portable database support.
- **Session scoping** -- Sessions can be scoped per-sender or globally, with configurable idle timeouts.
- **Event hooks** -- Lifecycle hooks (message received/sending, gateway start/stop) allow shell command integrations at key points.
- **Streaming support** -- The agent runner supports streaming output with tool execution loops. Use `RunStream()` instead of `Run()` to receive incremental responses with real-time tool execution feedback.

## Streaming Features

The Claude runner now supports streaming output with full tool execution capabilities:

### Runner API

```go
// Standard blocking execution
result, err := runner.Run(ctx, msg)

// Streaming execution with callback
result, err := runner.RunStream(ctx, msg, func(evt llm.StreamEvent) {
    switch evt.Type {
    case "delta":
        // Incremental text output
        fmt.Print(evt.Content)
    case "tool_start":
        // Tool execution beginning
        fmt.Println(evt.Content)
    case "tool_result":
        // Tool completed successfully
        fmt.Println(evt.Content)
    case "tool_error":
        // Tool execution failed
        fmt.Println(evt.Content)
    case "done":
        // Final response available in evt.Response
    case "error":
        // Stream error: evt.Error
    }
})
```

### Router API

```go
// Standard routing
router.HandleInbound(ctx, msg)

// Streaming routing (logs tool events)
router.HandleInboundStream(ctx, msg)
```

The streaming implementation maintains the same tool execution loop as the standard `Run()` method, supporting up to 5 iterations of tool calls with intermediate results streamed to the callback.

## License

See repository for license details.

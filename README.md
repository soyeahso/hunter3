# hunter3

A personal AI assistant that bridges messaging channels with LLM providers and
external tools. It manages conversations and sessions through a configurable
WebSocket-based gateway.

If you require communication methods other than IRC, consider installing and
configuring bitlbee. Otherwise, just use IRC.

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

The database lives at `~/.hunter3/hunter3.db`.

```yaml
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

## Environment Variables

| Variable | Description |
|---|---|
| `HUNTER3_IRCV3_MULTILINE` | Set to any non-empty value to enable IRCv3 `draft/multiline` support. When enabled, multi-line responses are sent as a single batch instead of being split into individual messages. The server must also advertise the capability. |

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
# hunter3

package config

// Config is the root configuration for Hunter3.
// Fields mirror the TypeScript Hunter3Config, scoped to what the Go rewrite needs.
type Config struct {
	CLI          string         `yaml:"cli,omitempty"` // "claude" | "copilot" | "none" — selects the primary CLI provider or "none" for direct API
	APIProvider  string         `yaml:"apiProvider,omitempty"` // "claude" | "gemini" | "ollama" — used when cli: none
	APIKey       string         `yaml:"apiKey,omitempty"` // API key for direct API access
	APIModel     string         `yaml:"apiModel,omitempty"` // Model ID when using direct API
	APIEndpoint  string         `yaml:"apiEndpoint,omitempty"` // Custom API endpoint (for Ollama)
	Gateway      GatewayConfig  `yaml:"gateway,omitempty"`
	Models       ModelsConfig   `yaml:"models,omitempty"`
	Agents       AgentsConfig   `yaml:"agents,omitempty"`
	Channels     ChannelsConfig `yaml:"channels,omitempty"`
	Session      SessionConfig  `yaml:"session,omitempty"`
	Logging      LoggingConfig  `yaml:"logging,omitempty"`
	Hooks        HooksConfig    `yaml:"hooks,omitempty"`
	Memory       MemoryConfig   `yaml:"memory,omitempty"`
}

// GatewayConfig controls the gateway HTTP/WebSocket server.
type GatewayConfig struct {
	Port           int              `yaml:"port,omitempty"`
	Mode           string           `yaml:"mode,omitempty"` // "local" | "remote"
	Bind           string           `yaml:"bind,omitempty"` // "auto" | "lan" | "loopback" | "custom" | "tailnet"
	CustomBindHost string           `yaml:"customBindHost,omitempty"`
	Auth           GatewayAuth      `yaml:"auth,omitempty"`
	TLS            GatewayTLS       `yaml:"tls,omitempty"`
	ControlUI      GatewayControlUI `yaml:"controlUi,omitempty"`
}

// GatewayAuth configures gateway authentication.
type GatewayAuth struct {
	Mode     string `yaml:"mode,omitempty"` // "token" | "password"
	Token    string `yaml:"token,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// GatewayTLS configures TLS for the gateway.
type GatewayTLS struct {
	Enabled      bool   `yaml:"enabled,omitempty"`
	AutoGenerate bool   `yaml:"autoGenerate,omitempty"`
	CertPath     string `yaml:"certPath,omitempty"`
	KeyPath      string `yaml:"keyPath,omitempty"`
}

// GatewayControlUI configures the gateway control panel.
type GatewayControlUI struct {
	Enabled        bool     `yaml:"enabled,omitempty"`
	BasePath       string   `yaml:"basePath,omitempty"`
	AllowedOrigins []string `yaml:"allowedOrigins,omitempty"`
}

// ModelsConfig defines model providers and their models.
type ModelsConfig struct {
	Mode      string                       `yaml:"mode,omitempty"` // "merge" | "replace"
	Providers map[string]ModelProviderEntry `yaml:"providers,omitempty"`
}

// ModelProviderEntry defines a model provider.
type ModelProviderEntry struct {
	BaseURL    string                 `yaml:"baseUrl"`
	APIKey     string                 `yaml:"apiKey,omitempty"`
	Auth       string                 `yaml:"auth,omitempty"` // "api-key" | "oauth" | "token"
	API        string                 `yaml:"api,omitempty"`  // "openai-completions" | "anthropic-messages" | "google-generative-ai"
	Headers    map[string]string      `yaml:"headers,omitempty"`
	AuthHeader bool                   `yaml:"authHeader,omitempty"`
	Models     []ModelDefinitionEntry `yaml:"models,omitempty"`
}

// ModelDefinitionEntry defines a single model.
type ModelDefinitionEntry struct {
	ID            string         `yaml:"id"`
	Name          string         `yaml:"name"`
	API           string         `yaml:"api,omitempty"`
	Reasoning     bool           `yaml:"reasoning,omitempty"`
	Input         []string       `yaml:"input,omitempty"` // ["text", "image"]
	Cost          ModelCost      `yaml:"cost,omitempty"`
	ContextWindow int            `yaml:"contextWindow,omitempty"`
	MaxTokens     int            `yaml:"maxTokens,omitempty"`
	Headers       map[string]string `yaml:"headers,omitempty"`
}

// ModelCost defines per-token costs.
type ModelCost struct {
	Input      float64 `yaml:"input"`
	Output     float64 `yaml:"output"`
	CacheRead  float64 `yaml:"cacheRead"`
	CacheWrite float64 `yaml:"cacheWrite"`
}

// AgentsConfig defines agent defaults and agent list.
type AgentsConfig struct {
	Defaults AgentDefaults `yaml:"defaults,omitempty"`
	List     []AgentEntry  `yaml:"list,omitempty"`
}

// AgentDefaults defines default settings for all agents.
type AgentDefaults struct {
	Model       string `yaml:"model,omitempty"`
	MaxTokens   int    `yaml:"maxTokens,omitempty"`
	Temperature *float64 `yaml:"temperature,omitempty"`
}

// AgentEntry defines a single agent.
type AgentEntry struct {
	ID        string        `yaml:"id"`
	Default   bool          `yaml:"default,omitempty"`
	Name      string        `yaml:"name,omitempty"`
	Workspace string        `yaml:"workspace,omitempty"`
	Model     string        `yaml:"model,omitempty"`
	Identity  IdentityEntry `yaml:"identity,omitempty"`
}

// IdentityEntry provides display info for an agent.
type IdentityEntry struct {
	Name   string `yaml:"name,omitempty"`
	Emoji  string `yaml:"emoji,omitempty"`
	Avatar string `yaml:"avatar,omitempty"`
}

// ChannelsConfig defines channel-specific configurations.
type ChannelsConfig struct {
	Defaults ChannelDefaults `yaml:"defaults,omitempty"`
	IRC      *IRCConfig      `yaml:"irc,omitempty"`
}

// ChannelDefaults defines default settings for all channels.
type ChannelDefaults struct {
	GroupPolicy string `yaml:"groupPolicy,omitempty"` // "open" | "disabled" | "allowlist"
}

// IRCConfig defines IRC channel settings.
type IRCConfig struct {
	Server   string   `yaml:"server"`
	Port     int      `yaml:"port,omitempty"`
	Nick     string   `yaml:"nick"`
	Password string   `yaml:"password,omitempty"`
	Channels []string `yaml:"channels"`
	UseTLS   bool     `yaml:"useTLS,omitempty"`
	SASL     bool     `yaml:"sasl,omitempty"`
	OpOnly   *bool    `yaml:"opOnly,omitempty"` // restrict to channel operators; defaults to true
	Owner    *string  `yaml:"owner,omitempty"`  // only accept messages from this nick; default "soyeahso", set "" to disable
	Stream   bool     `yaml:"stream,omitempty"` // enable incremental streaming to IRC
}

// SessionConfig defines session behavior.
type SessionConfig struct {
	Scope       string `yaml:"scope,omitempty"` // "per-sender" | "global"
	IdleMinutes int    `yaml:"idleMinutes,omitempty"`
	Store       string `yaml:"store,omitempty"`
}

// LoggingConfig controls logging behavior.
type LoggingConfig struct {
	Level         string `yaml:"level,omitempty"`        // "silent" | "fatal" | "error" | "warn" | "info" | "debug" | "trace"
	File          string `yaml:"file,omitempty"`
	ConsoleLevel  string `yaml:"consoleLevel,omitempty"`
	ConsoleStyle  string `yaml:"consoleStyle,omitempty"` // "pretty" | "compact" | "json"
}

// HooksConfig defines event hooks.
type HooksConfig struct {
	MessageReceived []HookEntry `yaml:"messageReceived,omitempty"`
	MessageSending  []HookEntry `yaml:"messageSending,omitempty"`
	GatewayStart    []HookEntry `yaml:"gatewayStart,omitempty"`
	GatewayStop     []HookEntry `yaml:"gatewayStop,omitempty"`
}

// HookEntry defines a single hook action.
type HookEntry struct {
	Command string `yaml:"command"`
	Timeout int    `yaml:"timeout,omitempty"` // milliseconds
}

// MemoryConfig configures the memory/knowledge system.
type MemoryConfig struct {
	Enabled    bool   `yaml:"enabled,omitempty"`
	Store      string `yaml:"store,omitempty"` // "sqlite" | "memory"
	SearchMode string `yaml:"searchMode,omitempty"` // "fts" | "embedding"
}



package llm

import (
	"fmt"
	"strings"
	"sync"

	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/logging"
)

// ProviderError is returned when an LLM provider fails.
type ProviderError struct {
	Provider string
	Message  string
	Code     int // HTTP-like status code (401, 429, 500, etc.)
}

func (e *ProviderError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("%s: %d %s", e.Provider, e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Provider, e.Message)
}

// Registry manages LLM provider clients and resolves model references to clients.
type Registry struct {
	mu       sync.RWMutex
	clients  map[string]Client // provider name → client
	aliases  map[string]string // model alias → provider name
	fallback string            // default provider name
	log      *logging.Logger
}

// NewRegistry creates an empty provider registry.
func NewRegistry(log *logging.Logger) *Registry {
	return &Registry{
		clients:  make(map[string]Client),
		aliases:  make(map[string]string),
		log:      log.Sub("llm.registry"),
	}
}

// Register adds a client under the given provider name.
func (r *Registry) Register(name string, client Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[name] = client
	r.log.Info().Str("provider", name).Msg("registered LLM provider")
}

// Alias maps a model name/alias to a provider.
// e.g., Alias("sonnet", "claude") means "sonnet" resolves to the "claude" provider.
func (r *Registry) Alias(model, provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.aliases[model] = provider
}

// SetFallback sets the default provider used when no model/provider match is found.
func (r *Registry) SetFallback(provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallback = provider
}

// Resolve returns the Client for the given model reference.
// Resolution order: exact provider name → alias → fallback.
func (r *Registry) Resolve(model string) (Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Direct provider name match
	if c, ok := r.clients[model]; ok {
		return c, nil
	}

	// Alias lookup
	if provider, ok := r.aliases[model]; ok {
		if c, ok := r.clients[provider]; ok {
			return c, nil
		}
	}

	// Fallback
	if r.fallback != "" {
		if c, ok := r.clients[r.fallback]; ok {
			return c, nil
		}
	}

	return nil, fmt.Errorf("no LLM provider for model %q", model)
}

// List returns all registered provider names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.clients))
	for n := range r.clients {
		names = append(names, n)
	}
	return names
}

// NewRegistryFromConfig builds a Registry by auto-detecting available CLI tools or using API clients.
// The cli parameter selects the primary CLI provider ("claude", "copilot", "none", or ""
// for auto-detect). When cli="none", uses API clients configured via apiProvider, apiKey, apiModel.
func NewRegistryFromConfig(cfg config.ModelsConfig, cliMode string, apiProvider, apiKey, apiModel, apiEndpoint string, log *logging.Logger) *Registry {
	reg := NewRegistry(log)

	cliMode = strings.ToLower(strings.TrimSpace(cliMode))

	// If cli: none, register API client instead of CLI
	if cliMode == "none" {
		switch apiProvider {
		case "claude":
			if apiKey != "" && apiModel != "" {
				client := NewClaudeAPIClient(apiKey, apiModel)
				reg.Register("claude", client)
				reg.SetFallback("claude")
				for _, alias := range []string{"sonnet", "opus", "haiku", "claude-sonnet", "claude-opus", "claude-haiku"} {
					reg.Alias(alias, "claude")
				}
			}

		case "gemini":
			if apiKey != "" && apiModel != "" {
				client := NewGeminiAPIClient(apiKey, apiModel)
				reg.Register("gemini", client)
				reg.SetFallback("gemini")
				for _, alias := range []string{"gemini-pro", "gemini-2.0"} {
					reg.Alias(alias, "gemini")
				}
			}

		case "ollama":
			if apiModel != "" {
				// apiEndpoint defaults to http://localhost:11434 if not provided
				client := NewOllamaAPIClient(apiEndpoint, apiModel)
				reg.Register("ollama", client)
				reg.SetFallback("ollama")
				for _, alias := range []string{"llama", "llama2", "llama3", "mistral"} {
					reg.Alias(alias, "ollama")
				}
			}
		}
		return reg
	}

	// Register Claude Code CLI
	if (cliMode == "" || cliMode == "claude") && CLIExists("claude") {
		client := NewClaudeClient(log)
		reg.Register("claude", client)
		if cliMode == "" || cliMode == "claude" {
			reg.SetFallback("claude")
		}
		for _, alias := range []string{"sonnet", "opus", "haiku", "claude-sonnet", "claude-opus", "claude-haiku"} {
			reg.Alias(alias, "claude")
		}
	}

	// Register GitHub Copilot CLI
	if (cliMode == "" || cliMode == "copilot") && CLIExists("copilot") {
		client := NewCopilotClient(log)
		reg.Register("copilot", client)
		if cliMode == "copilot" {
			reg.SetFallback("copilot")
		}
		for _, alias := range []string{"gpt-5", "claude-sonnet-4.5", "copilot-agent"} {
			reg.Alias(alias, "copilot")
		}
	}

	// Register configured providers
	for name, provider := range cfg.Providers {
		if _, exists := reg.clients[name]; exists {
			continue // don't override auto-detected
		}

		ecfg := externalConfigFromProvider(name, provider)
		if ecfg != nil && CLIExists(ecfg.Command) {
			client := NewExternalCLIClient(*ecfg, log)
			reg.Register(name, client)
		}
	}

	return reg
}

// externalConfigFromProvider maps known provider config patterns to ExternalCLIConfig.
func externalConfigFromProvider(name string, p config.ModelProviderEntry) *ExternalCLIConfig {
	switch {
	case name == "gemini" || p.API == "google-generative-ai":
		return &ExternalCLIConfig{
			Command:         "gemini",
			Name:            "gemini",
			ModelFlag:       "--model",
			SystemFlag:      "--system",
			JSONFlag:        []string{"--format", "json"},
			StreamFlag:      []string{"--format", "stream-json"},
			ResultField:     "result",
			StreamTextField: "content",
		}

	case name == "ollama":
		return &ExternalCLIConfig{
			Command:         "ollama",
			Name:            "ollama",
			BaseArgs:        []string{"run"},
			ResultField:     "response",
			StreamTextField: "response",
		}

	default:
		return nil
	}
}

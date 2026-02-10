package plugin

import (
	"context"
	"fmt"
	"sync"

	"github.com/soyeahso/hunter3/internal/hooks"
	"github.com/soyeahso/hunter3/internal/logging"
)

// Registry manages plugin lifecycle.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	order   []string // insertion order for deterministic lifecycle
	hooks   *hooks.Manager
	log     *logging.Logger
}

// NewRegistry creates a plugin registry.
func NewRegistry(hm *hooks.Manager, log *logging.Logger) *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
		hooks:   hm,
		log:     log.Sub("plugins"),
	}
}

// Register adds a plugin to the registry without initializing it.
func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[p.ID()]; exists {
		return fmt.Errorf("plugin already registered: %s", p.ID())
	}

	r.plugins[p.ID()] = p
	r.order = append(r.order, p.ID())

	r.log.Info().
		Str("id", p.ID()).
		Str("name", p.Name()).
		Str("version", p.Version()).
		Msg("plugin registered")

	return nil
}

// InitAll initializes all registered plugins in registration order.
func (r *Registry) InitAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, id := range r.order {
		p := r.plugins[id]
		api := API{
			Hooks: r.hooks,
			Log:   r.log.Sub(id),
		}

		r.log.Info().Str("id", id).Msg("initializing plugin")
		if err := p.Init(ctx, api); err != nil {
			return fmt.Errorf("init plugin %s: %w", id, err)
		}
	}
	return nil
}

// CloseAll shuts down all plugins in reverse registration order.
func (r *Registry) CloseAll() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := len(r.order) - 1; i >= 0; i-- {
		id := r.order[i]
		p := r.plugins[id]
		r.log.Info().Str("id", id).Msg("closing plugin")
		if err := p.Close(); err != nil {
			r.log.Error().Err(err).Str("id", id).Msg("plugin close error")
		}
	}
}

// Get returns a plugin by ID, or nil if not found.
func (r *Registry) Get(id string) Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[id]
}

// List returns all registered plugin IDs in registration order.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Count returns the number of registered plugins.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}

// Info returns summary information about all registered plugins.
func (r *Registry) Info() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(r.order))
	for _, id := range r.order {
		p := r.plugins[id]
		infos = append(infos, PluginInfo{
			ID:      p.ID(),
			Name:    p.Name(),
			Version: p.Version(),
		})
	}
	return infos
}

// PluginInfo holds summary data about a plugin.
type PluginInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Package channel provides channel management for messaging integrations.
package channel

import (
	"context"
	"sync"

	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/logging"
)

// Registry manages a set of messaging channels.
type Registry struct {
	mu       sync.RWMutex
	channels map[string]domain.Channel
	log      *logging.Logger
}

// NewRegistry creates a channel registry.
func NewRegistry(log *logging.Logger) *Registry {
	return &Registry{
		channels: make(map[string]domain.Channel),
		log:      log.Sub("channels"),
	}
}

// Register adds a channel to the registry.
func (r *Registry) Register(ch domain.Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[ch.ID()] = ch
	r.log.Info().Str("channel", ch.ID()).Msg("channel registered")
}

// Get returns a channel by ID.
func (r *Registry) Get(id string) (domain.Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, ok := r.channels[id]
	return ch, ok
}

// List returns all channel IDs.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.channels))
	for id := range r.channels {
		ids = append(ids, id)
	}
	return ids
}

// Status returns the status of all registered channels.
func (r *Registry) Status() []domain.ChannelStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	statuses := make([]domain.ChannelStatus, 0, len(r.channels))
	for _, ch := range r.channels {
		if sc, ok := ch.(interface{ Status() domain.ChannelStatus }); ok {
			statuses = append(statuses, sc.Status())
		} else {
			statuses = append(statuses, domain.ChannelStatus{
				ChannelID: ch.ID(),
				Running:   true,
			})
		}
	}
	return statuses
}

// StartAll starts all registered channels in background goroutines.
// Channel Start methods may block (e.g. IRC's Connect), so each is
// launched concurrently to avoid preventing subsequent initialization.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for id, ch := range r.channels {
		r.log.Info().Str("channel", id).Msg("starting channel")
		go func(id string, ch domain.Channel) {
			if err := ch.Start(ctx); err != nil {
				r.log.Error().Err(err).Str("channel", id).Msg("channel exited with error")
			}
		}(id, ch)
	}
	return nil
}

// StopAll stops all registered channels.
func (r *Registry) StopAll(ctx context.Context) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for id, ch := range r.channels {
		r.log.Info().Str("channel", id).Msg("stopping channel")
		if err := ch.Stop(ctx); err != nil {
			r.log.Error().Err(err).Str("channel", id).Msg("failed to stop channel")
		}
	}
}

// Count returns the number of registered channels.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.channels)
}

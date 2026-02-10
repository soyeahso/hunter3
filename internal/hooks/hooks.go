// Package hooks provides an event-driven hook system for Hunter3 lifecycle events.
package hooks

import (
	"context"
	"sync"

	"github.com/soyeahso/hunter3/internal/logging"
)

// Event names for the hook system.
const (
	EventMessageReceived = "message_received"
	EventMessageSending  = "message_sending"
	EventBeforeAgentRun  = "before_agent_run"
	EventAfterAgentRun   = "after_agent_run"
	EventSessionStart    = "session_start"
	EventSessionEnd      = "session_end"
	EventGatewayStart    = "gateway_start"
	EventGatewayStop     = "gateway_stop"
)

// AllEvents lists all known hook event names.
var AllEvents = []string{
	EventMessageReceived,
	EventMessageSending,
	EventBeforeAgentRun,
	EventAfterAgentRun,
	EventSessionStart,
	EventSessionEnd,
	EventGatewayStart,
	EventGatewayStop,
}

// Payload carries event data to hook handlers.
type Payload struct {
	Event string         `json:"event"`
	Data  map[string]any `json:"data,omitempty"`
}

// Handler is a function that handles a hook event.
// Returning an error logs the failure but does not stop processing.
type Handler func(ctx context.Context, p Payload) error

// Manager manages hook registrations and dispatches events.
type Manager struct {
	mu       sync.RWMutex
	handlers map[string][]namedHandler
	log      *logging.Logger
}

type namedHandler struct {
	name    string
	handler Handler
}

// NewManager creates a hook manager.
func NewManager(log *logging.Logger) *Manager {
	return &Manager{
		handlers: make(map[string][]namedHandler),
		log:      log.Sub("hooks"),
	}
}

// On registers a handler for the given event.
// The name identifies the handler for logging and debugging.
func (m *Manager) On(event, name string, handler Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[event] = append(m.handlers[event], namedHandler{name: name, handler: handler})
	m.log.Debug().Str("event", event).Str("handler", name).Msg("hook registered")
}

// Off removes all handlers with the given name from the event.
func (m *Manager) Off(event, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	handlers := m.handlers[event]
	filtered := make([]namedHandler, 0, len(handlers))
	for _, h := range handlers {
		if h.name != name {
			filtered = append(filtered, h)
		}
	}
	m.handlers[event] = filtered
}

// Emit dispatches an event to all registered handlers synchronously.
// Handlers are called in registration order. Errors are logged but do not
// prevent subsequent handlers from running.
func (m *Manager) Emit(ctx context.Context, event string, data map[string]any) {
	m.mu.RLock()
	handlers := make([]namedHandler, len(m.handlers[event]))
	copy(handlers, m.handlers[event])
	m.mu.RUnlock()

	if len(handlers) == 0 {
		return
	}

	payload := Payload{Event: event, Data: data}

	for _, h := range handlers {
		if err := h.handler(ctx, payload); err != nil {
			m.log.Warn().
				Err(err).
				Str("event", event).
				Str("handler", h.name).
				Msg("hook handler error")
		}
	}
}

// EmitAsync dispatches an event to all registered handlers concurrently.
// Returns immediately; handler errors are logged.
func (m *Manager) EmitAsync(ctx context.Context, event string, data map[string]any) {
	m.mu.RLock()
	handlers := make([]namedHandler, len(m.handlers[event]))
	copy(handlers, m.handlers[event])
	m.mu.RUnlock()

	if len(handlers) == 0 {
		return
	}

	payload := Payload{Event: event, Data: data}

	for _, h := range handlers {
		go func(h namedHandler) {
			if err := h.handler(ctx, payload); err != nil {
				m.log.Warn().
					Err(err).
					Str("event", event).
					Str("handler", h.name).
					Msg("async hook handler error")
			}
		}(h)
	}
}

// Count returns the number of handlers registered for an event.
func (m *Manager) Count(event string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.handlers[event])
}

// Events returns the list of events that have at least one handler registered.
func (m *Manager) Events() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]string, 0, len(m.handlers))
	for event, handlers := range m.handlers {
		if len(handlers) > 0 {
			events = append(events, event)
		}
	}
	return events
}

// Package plugin provides the plugin interface and lifecycle management for Hunter3.
package plugin

import (
	"context"

	"github.com/soyeahso/hunter3/internal/hooks"
	"github.com/soyeahso/hunter3/internal/logging"
)

// Plugin is the interface that all Hunter3 plugins must implement.
type Plugin interface {
	// ID returns a unique identifier for the plugin (e.g., "my-plugin").
	ID() string

	// Name returns a human-readable name.
	Name() string

	// Version returns the plugin version string.
	Version() string

	// Init initializes the plugin with the given context.
	// Plugins should register hooks and set up resources here.
	Init(ctx context.Context, api API) error

	// Close shuts down the plugin and releases resources.
	Close() error
}

// API is the interface exposed to plugins for interacting with Hunter3.
type API struct {
	Hooks  *hooks.Manager
	Log    *logging.Logger
}

package domain

import "context"

// ChannelCapabilities describes what a channel implementation supports.
type ChannelCapabilities struct {
	ChatTypes []ChatType `json:"chatTypes"`
	Media     bool       `json:"media,omitempty"`
	Reactions bool       `json:"reactions,omitempty"`
	Edit      bool       `json:"edit,omitempty"`
	Threads   bool       `json:"threads,omitempty"`
	Reply     bool       `json:"reply,omitempty"`
}

// ChannelStatus reports the runtime state of a channel.
type ChannelStatus struct {
	ChannelID   string `json:"channelId"`
	AccountID   string `json:"accountId,omitempty"`
	Connected   bool   `json:"connected"`
	Running     bool   `json:"running"`
	LastError   string `json:"lastError,omitempty"`
}

// Channel is the interface that all messaging channel implementations must satisfy.
type Channel interface {
	// ID returns the channel identifier (e.g., "irc", "discord").
	ID() string

	// Capabilities returns what this channel supports.
	Capabilities() ChannelCapabilities

	// Start connects the channel and begins listening for messages.
	Start(ctx context.Context) error

	// Stop gracefully disconnects the channel.
	Stop(ctx context.Context) error

	// Send delivers an outbound message through this channel.
	Send(ctx context.Context, msg OutboundMessage) error

	// OnMessage registers a handler for inbound messages.
	OnMessage(handler func(msg InboundMessage))
}

// Package routing connects messaging channels to the agent runner.
package routing

import (
	"context"
	"fmt"

	"github.com/soyeahso/hunter3/internal/agent"
	"github.com/soyeahso/hunter3/internal/channel"
	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/llm"
	"github.com/soyeahso/hunter3/internal/logging"
)

// StreamEvent is an alias for llm.StreamEvent for convenience.
type StreamEvent = llm.StreamEvent

// Router routes inbound messages to the agent and outbound responses to channels.
type Router struct {
	channels *channel.Registry
	runner   *agent.Runner
	scope    string // "per-sender" | "global"
	ircOwner string // IRC owner name for completion messages
	log      *logging.Logger
}

// NewRouter creates a message router.
func NewRouter(
	channels *channel.Registry,
	runner *agent.Runner,
	scope string,
	ircOwner string,
	log *logging.Logger,
) *Router {
	if scope == "" {
		scope = "per-sender"
	}
	if ircOwner == "" {
		ircOwner = "soyeahso" // default owner
	}
	return &Router{
		channels: channels,
		runner:   runner,
		scope:    scope,
		ircOwner: ircOwner,
		log:      log.Sub("routing"),
	}
}

// HandleInbound processes an inbound message from any channel.
// It runs the agent and sends the response back through the originating channel.
func (r *Router) HandleInbound(ctx context.Context, msg domain.InboundMessage) {
	r.log.Info().
		Str("channel", msg.ChannelID).
		Str("from", msg.From).
		Str("chatId", msg.ChatID).
		Str("chatType", string(msg.ChatType)).
		Msg("routing inbound message")

	if r.runner == nil {
		r.log.Warn().Msg("no agent runner configured, dropping message")
		return
	}

	// Resolve the session key based on scope
	key := ResolveSessionKey(msg, r.scope)

	// Override the message fields with the resolved key for session lookup
	routed := msg
	routed.ChatID = key.ChatID
	if r.scope == "global" {
		routed.From = ""
	}

	result, err := r.runner.Run(ctx, routed)
	if err != nil {
		r.log.Error().Err(err).
			Str("channel", msg.ChannelID).
			Str("from", msg.From).
			Msg("agent run failed")
		return
	}

	// Build outbound message
	reply := domain.OutboundMessage{
		ChannelID: msg.ChannelID,
		To:        replyTarget(msg),
		Body:      result.Response,
		ReplyToID: msg.ID,
	}

	// Send via the originating channel
	ch, ok := r.channels.Get(msg.ChannelID)
	if !ok {
		r.log.Error().Str("channel", msg.ChannelID).Msg("channel not found for reply")
		return
	}

	if err := ch.Send(ctx, reply); err != nil {
		r.log.Error().Err(err).
			Str("channel", msg.ChannelID).
			Str("to", reply.To).
			Msg("failed to send reply")
		return
	}

	r.log.Info().
		Str("channel", msg.ChannelID).
		Str("to", reply.To).
		Str("sessionId", result.SessionID).
		Str("model", result.Model).
		Dur("duration", result.Duration).
		Msg("reply sent")

	// Send completion message for IRC channels
	if msg.ChannelID == "irc" {
		owner := msg.FromName
		if owner == "" {
			owner = r.ircOwner
		}
		completionMsg := domain.OutboundMessage{
			ChannelID: msg.ChannelID,
			To:        reply.To,
			Body:      fmt.Sprintf("I'm done with my task, %s", owner),
		}
		if err := ch.Send(ctx, completionMsg); err != nil {
			r.log.Error().Err(err).
				Str("channel", msg.ChannelID).
				Str("to", reply.To).
				Msg("failed to send completion message")
		}
	}
}

// HandleInboundStream processes an inbound message with streaming output.
// Deltas are incrementally flushed to the channel at natural text boundaries.
func (r *Router) HandleInboundStream(ctx context.Context, msg domain.InboundMessage) {
	r.log.Info().
		Str("channel", msg.ChannelID).
		Str("from", msg.From).
		Str("chatId", msg.ChatID).
		Str("chatType", string(msg.ChatType)).
		Msg("routing inbound message with streaming")

	if r.runner == nil {
		r.log.Warn().Msg("no agent runner configured, dropping message")
		return
	}

	// Resolve the session key based on scope
	key := ResolveSessionKey(msg, r.scope)

	routed := msg
	routed.ChatID = key.ChatID
	if r.scope == "global" {
		routed.From = ""
	}

	// Resolve channel for incremental sending
	ch, ok := r.channels.Get(msg.ChannelID)
	if !ok {
		r.log.Error().Str("channel", msg.ChannelID).Msg("channel not found for streaming reply")
		return
	}

	target := replyTarget(msg)

	flusher := NewStreamFlusher(
		ctx,
		StreamFlusherConfig{},
		ch,
		msg.ChannelID,
		target,
		r.log,
	)

	callback := func(evt llm.StreamEvent) {
		switch evt.Type {
		case "delta":
			flusher.OnDelta(evt.Content)
		case "tool_start", "tool_result", "tool_error":
			r.log.Debug().
				Str("eventType", evt.Type).
				Str("content", evt.Content).
				Msg("tool event")
		case "error":
			r.log.Error().
				Str("error", evt.Error).
				Msg("stream error")
		}
	}

	result, err := r.runner.RunStream(ctx, routed, callback)
	if err != nil {
		r.log.Error().Err(err).
			Str("channel", msg.ChannelID).
			Str("from", msg.From).
			Msg("agent stream run failed")
		// Flush anything that was buffered before the error
		flusher.Flush()
		return
	}

	// Flush remaining buffered content
	flusher.Flush()

	r.log.Info().
		Str("channel", msg.ChannelID).
		Str("to", target).
		Str("sessionId", result.SessionID).
		Str("model", result.Model).
		Dur("duration", result.Duration).
		Bool("streamed", flusher.Flushed()).
		Msg("streaming reply completed")

	// Send completion message for IRC channels
	if msg.ChannelID == "irc" {
		owner := msg.FromName
		if owner == "" {
			owner = r.ircOwner
		}
		completionMsg := domain.OutboundMessage{
			ChannelID: msg.ChannelID,
			To:        target,
			Body:      fmt.Sprintf("I'm done with my task, %s", owner),
		}
		if err := ch.Send(ctx, completionMsg); err != nil {
			r.log.Error().Err(err).
				Str("channel", msg.ChannelID).
				Str("to", target).
				Msg("failed to send completion message")
		}
	}
}

// Wire registers the router's HandleInbound as the message handler on all channels.
func (r *Router) Wire() {
	for _, id := range r.channels.List() {
		ch, ok := r.channels.Get(id)
		if !ok {
			continue
		}
		ch.OnMessage(func(msg domain.InboundMessage) {
			go r.HandleInbound(context.Background(), msg)
		})
		r.log.Debug().Str("channel", id).Msg("wired message handler")
	}
}

// WireStream registers HandleInboundStream as the message handler on all channels.
// This enables incremental streaming output for channels like IRC.
func (r *Router) WireStream() {
	for _, id := range r.channels.List() {
		ch, ok := r.channels.Get(id)
		if !ok {
			continue
		}
		ch.OnMessage(func(msg domain.InboundMessage) {
			go r.HandleInboundStream(context.Background(), msg)
		})
		r.log.Debug().Str("channel", id).Msg("wired streaming message handler")
	}
}

// replyTarget determines where to send the response.
func replyTarget(msg domain.InboundMessage) string {
	switch msg.ChatType {
	case domain.ChatTypeDM:
		return msg.From
	case domain.ChatTypeGroup:
		return msg.ChatID
	default:
		return msg.ChatID
	}
}

// SendTo sends a message to a specific channel.
func (r *Router) SendTo(ctx context.Context, channelID, target, body string) error {
	ch, ok := r.channels.Get(channelID)
	if !ok {
		return fmt.Errorf("channel not found: %s", channelID)
	}
	return ch.Send(ctx, domain.OutboundMessage{
		ChannelID: channelID,
		To:        target,
		Body:      body,
	})
}

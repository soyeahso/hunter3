// Package irc implements the IRC messaging channel using the girc library.
package irc

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lrstanley/girc"
	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/logging"
)

// Channel implements domain.Channel for IRC.
type Channel struct {
	cfg    config.IRCConfig
	client *girc.Client
	log    *logging.Logger

	mu      sync.RWMutex
	handler func(msg domain.InboundMessage)
	running bool
	lastErr string

	batches          *batchTracker
	mlCaps           multilineCaps
	mlCapsMu         sync.RWMutex
	multilineEnabled bool
}

// New creates an IRC channel from configuration.
func New(cfg config.IRCConfig, log *logging.Logger) *Channel {
	return &Channel{
		cfg:     cfg,
		log:     log.Sub("irc"),
		batches: newBatchTracker(),
	}
}

func (c *Channel) ID() string { return "irc" }

func (c *Channel) Capabilities() domain.ChannelCapabilities {
	return domain.ChannelCapabilities{
		ChatTypes: []domain.ChatType{domain.ChatTypeDM, domain.ChatTypeGroup},
		Media:     false,
		Reactions: false,
		Edit:      false,
		Threads:   false,
		Reply:     false,
	}
}

func (c *Channel) OnMessage(handler func(msg domain.InboundMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// Status returns the current runtime status.
func (c *Channel) Status() domain.ChannelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return domain.ChannelStatus{
		ChannelID: "irc",
		Connected: c.client != nil && c.client.IsConnected(),
		Running:   c.running,
		LastError: c.lastErr,
	}
}

// Start connects to the IRC server and begins processing messages.
func (c *Channel) Start(ctx context.Context) error {
	port := c.cfg.Port
	if port == 0 {
		if c.cfg.UseTLS {
			port = 6697
		} else {
			port = 6667
		}
	}

	c.multilineEnabled = os.Getenv("HUNTER3_IRCV3_MULTILINE") != ""

	gircCfg := girc.Config{
		Server:  c.cfg.Server,
		Port:    port,
		Nick:    c.cfg.Nick,
		User:    c.cfg.Nick,
		Name:    "Hunter3 IRC Bot",
		SSL:     c.cfg.UseTLS,
		Version: "Hunter3/1.0",
	}

	if c.multilineEnabled {
		gircCfg.SupportedCaps = map[string][]string{
			capMultiline: nil,
		}
		c.log.Info().Msg("IRCv3 draft/multiline enabled via HUNTER3_IRCV3_MULTILINE")
	}

	if c.cfg.UseTLS {
		gircCfg.TLSConfig = &tls.Config{
			ServerName: c.cfg.Server,
		}
	}

	if c.cfg.SASL && c.cfg.Password != "" {
		gircCfg.SASL = &girc.SASLPlain{
			User: c.cfg.Nick,
			Pass: c.cfg.Password,
		}
	} else if c.cfg.Password != "" {
		gircCfg.ServerPass = c.cfg.Password
	}

	c.client = girc.New(gircCfg)
	c.registerHandlers()

	c.mu.Lock()
	c.running = true
	c.lastErr = ""
	c.mu.Unlock()

	c.log.Info().
		Str("server", c.cfg.Server).
		Int("port", port).
		Str("nick", c.cfg.Nick).
		Strs("channels", c.cfg.Channels).
		Bool("tls", c.cfg.UseTLS).
		Msg("connecting to IRC")

	// Run connection in a goroutine — Connect() blocks
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.client.Connect()
	}()

	// Wait for either context cancellation or connection error
	select {
	case err := <-errCh:
		c.mu.Lock()
		c.running = false
		if err != nil {
			c.lastErr = err.Error()
		}
		c.mu.Unlock()
		if err != nil {
			return fmt.Errorf("irc connect: %w", err)
		}
		return nil
	case <-ctx.Done():
		c.client.Close()
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
		return ctx.Err()
	}
}

// Stop gracefully disconnects from the IRC server.
func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil && c.client.IsConnected() {
		c.log.Info().Msg("disconnecting from IRC")
		c.client.Quit("Hunter3 shutting down")
	}
	c.running = false
	return nil
}

// Send delivers a message to an IRC channel or user.
func (c *Channel) Send(ctx context.Context, msg domain.OutboundMessage) error {
	if c.client == nil || !c.client.IsConnected() {
		return fmt.Errorf("irc: not connected")
	}

	target := msg.To
	if target == "" {
		return fmt.Errorf("irc: no target specified")
	}

	// Use multiline batch if the server supports it and the message has newlines.
	if strings.Contains(msg.Body, "\n") && c.hasMultiline() {
		c.mlCapsMu.RLock()
		caps := c.mlCaps
		c.mlCapsMu.RUnlock()

		sendMultiline(c.client, target, msg.Body, caps)
		c.log.Debug().
			Str("to", target).
			Msg("sent IRC multiline batch")
		return nil
	}

	// Fallback: split long messages into chunks (IRC has ~512 byte line limit)
	lines := splitMessage(msg.Body, 400)
	for _, line := range lines {
		c.client.Cmd.Message(target, line)
	}

	c.log.Debug().
		Str("to", target).
		Int("lines", len(lines)).
		Msg("sent IRC message")

	return nil
}

// hasMultiline returns true if multiline is enabled and the server has acknowledged draft/multiline.
func (c *Channel) hasMultiline() bool {
	if !c.multilineEnabled {
		return false
	}
	if c.client == nil || !c.client.IsConnected() {
		return false
	}
	return c.client.HasCapability(capMultiline)
}

// registerHandlers sets up all IRC event handlers.
func (c *Channel) registerHandlers() {
	c.client.Handlers.Add(girc.CONNECTED, c.onConnected)
	c.client.Handlers.Add(girc.PRIVMSG, c.onPrivmsg)
	c.client.Handlers.Add(girc.JOIN, c.onJoin)
	c.client.Handlers.Add(girc.PART, c.onPart)
	c.client.Handlers.Add(girc.DISCONNECTED, c.onDisconnected)
	c.client.Handlers.Add(cmdBATCH, c.onBatch)
	c.client.Handlers.Add(girc.CAP, c.onCAP)
	c.client.Handlers.Add("FAIL", c.onFail)
}

func (c *Channel) onConnected(_ *girc.Client, e girc.Event) {
	c.log.Info().Str("nick", c.client.GetNick()).Msg("connected to IRC")

	for _, ch := range c.cfg.Channels {
		c.log.Info().Str("channel", ch).Msg("joining channel")
		c.client.Cmd.Join(ch)
		c.log.Info().Str("channel", ch).Msg("joined channel")
	}
}

func (c *Channel) onPrivmsg(_ *girc.Client, e girc.Event) {
	if e.Source == nil {
		return
	}

	// Ignore messages from ourselves
	if e.Source.Name == c.client.GetNick() {
		return
	}

	// If this PRIVMSG is part of a multiline batch, buffer it instead of dispatching.
	if batchID, ok := isBatchEvent(e); ok && c.batches.has(batchID) {
		c.batches.addLine(batchID, e.Last(), hasConcatTag(e))
		return
	}

	c.dispatchMessage(e)
}

// opOnly returns whether the operator-only restriction is enabled.
// Defaults to true when not explicitly configured.
func (c *Channel) opOnly() bool {
	if c.cfg.OpOnly == nil {
		return true
	}
	return *c.cfg.OpOnly
}

// owner returns the configured owner nick. Defaults to "soyeahso" when not
// configured. An empty string disables owner-only filtering.
func (c *Channel) owner() string {
	if c.cfg.Owner == nil {
		return "soyeahso"
	}
	return *c.cfg.Owner
}

// isChannelOp checks whether the given nick has operator (or higher) permissions
// in the specified channel. Returns false if the user or channel cannot be found.
func (c *Channel) isChannelOp(nick, channel string) bool {
	user := c.client.LookupUser(nick)
	if user == nil {
		return false
	}
	perms, ok := user.Perms.Lookup(channel)
	if !ok {
		return false
	}
	return perms.IsAdmin()
}

// dispatchMessage constructs and dispatches an InboundMessage from a PRIVMSG event.
func (c *Channel) dispatchMessage(e girc.Event) {
	// Ignore direct messages entirely.
	if !e.IsFromChannel() {
		c.log.Debug().
			Str("nick", e.Source.Name).
			Msg("ignoring direct message")
		return
	}

	chatID := e.Params[0]
	body := e.Last()

	// Only respond to messages that mention us.
	if !strings.Contains(strings.ToLower(body), "hunter3") {
		return
	}

	// Only respond to the configured owner when owner filtering is enabled.
	if ownerNick := c.owner(); ownerNick != "" && !strings.EqualFold(e.Source.Name, ownerNick) {
		c.log.Debug().
			Str("nick", e.Source.Name).
			Str("owner", ownerNick).
			Str("channel", chatID).
			Msg("ignoring message from non-owner")
		return
	}

	// Only respond to channel operators when opOnly is enabled.
	if c.opOnly() && !c.isChannelOp(e.Source.Name, chatID) {
		c.log.Debug().
			Str("nick", e.Source.Name).
			Str("channel", chatID).
			Msg("ignoring message from non-operator")
		c.client.Cmd.Message(chatID, "I'm sorry, Dave. I'm afraid I can't do that.")
		return
	}

	if e.IsAction() {
		body = e.StripAction()
	}

	c.deliverInbound(e.Source.Name, chatID, domain.ChatTypeGroup, body)
}

// dispatchBatch constructs and dispatches an InboundMessage from a completed multiline batch.
func (c *Channel) dispatchBatch(source *girc.Source, target, body string) {
	if source == nil {
		return
	}

	// Ignore direct messages entirely.
	if !girc.IsValidChannel(target) {
		c.log.Debug().
			Str("nick", source.Name).
			Msg("ignoring direct batch message")
		return
	}

	// Only respond to messages that mention us.
	if !strings.Contains(strings.ToLower(body), "hunter3") {
		return
	}

	// Only respond to the configured owner when owner filtering is enabled.
	if ownerNick := c.owner(); ownerNick != "" && !strings.EqualFold(source.Name, ownerNick) {
		c.log.Debug().
			Str("nick", source.Name).
			Str("owner", ownerNick).
			Str("channel", target).
			Msg("ignoring batch message from non-owner")
		return
	}

	// Only respond to channel operators when opOnly is enabled.
	if c.opOnly() && !c.isChannelOp(source.Name, target) {
		c.log.Debug().
			Str("nick", source.Name).
			Str("channel", target).
			Msg("ignoring batch message from non-operator")
		c.client.Cmd.Message(target, "I'm sorry, Dave. I'm afraid I can't do that.")
		return
	}

	c.deliverInbound(source.Name, target, domain.ChatTypeGroup, body)
}

func (c *Channel) deliverInbound(from, chatID string, chatType domain.ChatType, body string) {
	msg := domain.InboundMessage{
		ID:        uuid.New().String(),
		ChannelID: "irc",
		From:      from,
		FromName:  from,
		ChatID:    chatID,
		ChatType:  chatType,
		Body:      body,
		Timestamp: time.Now(),
	}

	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()

	if handler != nil {
		handler(msg)
	}
}

func (c *Channel) onJoin(_ *girc.Client, e girc.Event) {
	if e.Source == nil {
		return
	}
	c.log.Debug().
		Str("nick", e.Source.Name).
		Str("channel", e.Params[0]).
		Msg("user joined")
}

func (c *Channel) onPart(_ *girc.Client, e girc.Event) {
	if e.Source == nil {
		return
	}
	c.log.Debug().
		Str("nick", e.Source.Name).
		Str("channel", e.Params[0]).
		Msg("user parted")
}

func (c *Channel) onDisconnected(_ *girc.Client, e girc.Event) {
	c.log.Warn().Msg("disconnected from IRC")
	c.mu.Lock()
	c.running = false
	c.mu.Unlock()
}

// onBatch handles incoming BATCH start/end commands for multiline message reassembly.
func (c *Channel) onBatch(_ *girc.Client, e girc.Event) {
	if len(e.Params) < 1 {
		return
	}

	ref := e.Params[0]
	if len(ref) < 2 {
		return
	}

	switch ref[0] {
	case '+':
		id, batchType, target, ok := parseBatchStart(e)
		if !ok {
			return
		}
		if c.batches.start(id, batchType, target, e.Source) {
			c.log.Debug().
				Str("batchID", id).
				Str("target", target).
				Msg("multiline batch started")
		}

	case '-':
		id, ok := parseBatchEnd(e)
		if !ok {
			return
		}
		target, source, body, found := c.batches.end(id)
		if !found {
			return
		}
		c.log.Debug().
			Str("batchID", id).
			Int("bodyLen", len(body)).
			Msg("multiline batch completed")
		c.dispatchBatch(source, target, body)
	}
}

// onCAP watches for CAP LS responses to extract draft/multiline parameters.
func (c *Channel) onCAP(_ *girc.Client, e girc.Event) {
	if len(e.Params) < 3 {
		return
	}
	sub := e.Params[1]
	if sub != "LS" && sub != "NEW" && sub != "ACK" {
		return
	}

	caps, found := extractMultilineCapsFromLS(e.Last())
	if !found {
		return
	}

	c.mlCapsMu.Lock()
	c.mlCaps = caps
	c.mlCapsMu.Unlock()

	c.log.Info().
		Int("maxBytes", caps.maxBytes).
		Int("maxLines", caps.maxLines).
		Msg("draft/multiline capability detected")
}

// onFail handles FAIL responses, logging multiline-specific errors.
func (c *Channel) onFail(_ *girc.Client, e girc.Event) {
	if code, ok := isMultilineFail(e); ok {
		c.log.Warn().
			Str("code", code).
			Str("detail", formatFailMessage(e)).
			Msg("multiline batch rejected by server")
	}
}

// splitMessage breaks a long message into chunks suitable for IRC.
// Each newline in the input produces a separate chunk because IRC
// PRIVMSG does not support embedded newlines. Lines longer than
// maxLen are further split at the byte boundary.
func splitMessage(text string, maxLen int) []string {
	var chunks []string
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			//continue
			chunks = append(chunks, line)
		}
		for len(line) > maxLen {
			chunks = append(chunks, line[:maxLen])
			line = line[maxLen:]
		}
		if line != "" {
			chunks = append(chunks, line)
		}
	}
	if len(chunks) == 0 {
		return []string{text}
	}
	return chunks
}

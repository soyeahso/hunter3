package routing

import (
	"context"
	"testing"
	"time"

	"github.com/soyeahso/hunter3/internal/agent"
	"github.com/soyeahso/hunter3/internal/channel"
	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/llm"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *logging.Logger {
	return logging.New(nil, "silent")
}

// mockChannel is a test double for domain.Channel.
type mockChannel struct {
	id      string
	sent    []domain.OutboundMessage
	handler func(domain.InboundMessage)
}

func (m *mockChannel) ID() string { return m.id }
func (m *mockChannel) Capabilities() domain.ChannelCapabilities {
	return domain.ChannelCapabilities{ChatTypes: []domain.ChatType{domain.ChatTypeDM}}
}
func (m *mockChannel) Start(_ context.Context) error           { return nil }
func (m *mockChannel) Stop(_ context.Context) error            { return nil }
func (m *mockChannel) Send(_ context.Context, msg domain.OutboundMessage) error {
	m.sent = append(m.sent, msg)
	return nil
}
func (m *mockChannel) OnMessage(handler func(domain.InboundMessage)) {
	m.handler = handler
}

func newTestRunner(log *logging.Logger) *agent.Runner {
	mock := &llm.MockClient{
		ProviderName: "test-model",
		CompleteFunc: func(_ context.Context, _ llm.CompletionRequest) (*llm.CompletionResponse, error) {
			return &llm.CompletionResponse{
				Content: "Hello from the agent!",
				Model:   "test-model",
				Usage:   llm.Usage{InputTokens: 10, OutputTokens: 5},
			}, nil
		},
	}
	reg := llm.NewRegistry(log)
	reg.Register("test-model", mock)

	return agent.NewRunner(
		agent.RunnerConfig{
			AgentID:   "test",
			AgentName: "TestBot",
			Model:     "test-model",
		},
		reg,
		agent.NewMemorySessionStore(),
		agent.NewToolRegistry(),
		log,
	)
}

func TestRouter_HandleInbound_DM(t *testing.T) {
	log := testLogger()
	ch := &mockChannel{id: "irc"}

	reg := channel.NewRegistry(log)
	reg.Register(ch)

	runner := newTestRunner(log)
	router := NewRouter(reg, runner, "per-sender", "", log)

	msg := domain.InboundMessage{
		ID:        "msg-1",
		ChannelID: "irc",
		From:      "alice",
		FromName:  "Alice",
		ChatID:    "alice",
		ChatType:  domain.ChatTypeDM,
		Body:      "Hi there",
		Timestamp: time.Now(),
	}

	router.HandleInbound(context.Background(), msg)

	require.Len(t, ch.sent, 1)
	assert.Equal(t, "alice", ch.sent[0].To)
	assert.Equal(t, "Hello from the agent!", ch.sent[0].Body)
	assert.Equal(t, "irc", ch.sent[0].ChannelID)
}

func TestRouter_HandleInbound_Group(t *testing.T) {
	log := testLogger()
	ch := &mockChannel{id: "irc"}

	reg := channel.NewRegistry(log)
	reg.Register(ch)

	runner := newTestRunner(log)
	router := NewRouter(reg, runner, "per-sender", "", log)

	msg := domain.InboundMessage{
		ID:        "msg-2",
		ChannelID: "irc",
		From:      "bob",
		FromName:  "Bob",
		ChatID:    "#general",
		ChatType:  domain.ChatTypeGroup,
		Body:      "Hello channel",
		Timestamp: time.Now(),
	}

	router.HandleInbound(context.Background(), msg)

	require.Len(t, ch.sent, 1)
	assert.Equal(t, "#general", ch.sent[0].To)
	assert.Equal(t, "Hello from the agent!", ch.sent[0].Body)
}

func TestRouter_HandleInbound_NoRunner(t *testing.T) {
	log := testLogger()
	ch := &mockChannel{id: "irc"}

	reg := channel.NewRegistry(log)
	reg.Register(ch)

	router := NewRouter(reg, nil, "per-sender", "", log)

	msg := domain.InboundMessage{
		ID:        "msg-3",
		ChannelID: "irc",
		From:      "alice",
		Body:      "Hi",
		Timestamp: time.Now(),
	}

	router.HandleInbound(context.Background(), msg)
	assert.Empty(t, ch.sent) // No response sent
}

func TestRouter_HandleInbound_ChannelNotFound(t *testing.T) {
	log := testLogger()
	reg := channel.NewRegistry(log) // empty registry
	runner := newTestRunner(log)
	router := NewRouter(reg, runner, "per-sender", "", log)

	msg := domain.InboundMessage{
		ID:        "msg-4",
		ChannelID: "nonexistent",
		From:      "alice",
		ChatID:    "alice",
		ChatType:  domain.ChatTypeDM,
		Body:      "Hi",
		Timestamp: time.Now(),
	}

	// Should not panic, just log error
	router.HandleInbound(context.Background(), msg)
}

func TestRouter_Wire(t *testing.T) {
	log := testLogger()
	ch := &mockChannel{id: "irc"}

	reg := channel.NewRegistry(log)
	reg.Register(ch)

	runner := newTestRunner(log)
	router := NewRouter(reg, runner, "per-sender", "", log)
	router.Wire()

	// Verify the handler was set
	assert.NotNil(t, ch.handler)
}

func TestRouter_SendTo(t *testing.T) {
	log := testLogger()
	ch := &mockChannel{id: "irc"}

	reg := channel.NewRegistry(log)
	reg.Register(ch)

	router := NewRouter(reg, nil, "per-sender", "", log)

	err := router.SendTo(context.Background(), "irc", "#test", "hello")
	require.NoError(t, err)
	require.Len(t, ch.sent, 1)
	assert.Equal(t, "#test", ch.sent[0].To)
	assert.Equal(t, "hello", ch.sent[0].Body)
}

func TestRouter_SendTo_NotFound(t *testing.T) {
	log := testLogger()
	reg := channel.NewRegistry(log)
	router := NewRouter(reg, nil, "per-sender", "", log)

	err := router.SendTo(context.Background(), "nonexistent", "#test", "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSessionKey_PerSender(t *testing.T) {
	msg := domain.InboundMessage{
		ChannelID: "irc",
		From:      "alice",
		ChatID:    "#general",
	}

	key := ResolveSessionKey(msg, "per-sender")
	assert.Equal(t, "irc", key.ChannelID)
	assert.Equal(t, "#general", key.ChatID)
	assert.Equal(t, "alice", key.SenderID)
}

func TestSessionKey_Global(t *testing.T) {
	msg := domain.InboundMessage{
		ChannelID: "irc",
		From:      "alice",
		ChatID:    "#general",
	}

	key := ResolveSessionKey(msg, "global")
	assert.Equal(t, "irc", key.ChannelID)
	assert.Equal(t, "#general", key.ChatID)
	assert.Empty(t, key.SenderID) // No sender in global mode
}

func TestSessionKey_DefaultScope(t *testing.T) {
	msg := domain.InboundMessage{
		ChannelID: "irc",
		From:      "bob",
		ChatID:    "#test",
	}

	key := ResolveSessionKey(msg, "")
	assert.Equal(t, "bob", key.SenderID) // Defaults to per-sender
}

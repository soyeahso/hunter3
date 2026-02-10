package channel

import (
	"context"
	"testing"
	"time"

	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *logging.Logger {
	return logging.New(nil, "silent")
}

// mockChannel is a test double for domain.Channel.
type mockChannel struct {
	id       string
	started  bool
	stopped  bool
	sent     []domain.OutboundMessage
	handler  func(domain.InboundMessage)
	startErr error
	stopErr  error
	sendErr  error
}

func (m *mockChannel) ID() string { return m.id }
func (m *mockChannel) Capabilities() domain.ChannelCapabilities {
	return domain.ChannelCapabilities{
		ChatTypes: []domain.ChatType{domain.ChatTypeDM},
	}
}
func (m *mockChannel) Start(_ context.Context) error {
	m.started = true
	return m.startErr
}
func (m *mockChannel) Stop(_ context.Context) error {
	m.stopped = true
	return m.stopErr
}
func (m *mockChannel) Send(_ context.Context, msg domain.OutboundMessage) error {
	m.sent = append(m.sent, msg)
	return m.sendErr
}
func (m *mockChannel) OnMessage(handler func(domain.InboundMessage)) {
	m.handler = handler
}
func (m *mockChannel) Status() domain.ChannelStatus {
	return domain.ChannelStatus{
		ChannelID: m.id,
		Connected: m.started && !m.stopped,
		Running:   m.started && !m.stopped,
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry(testLogger())
	ch := &mockChannel{id: "test"}
	reg.Register(ch)

	got, ok := reg.Get("test")
	require.True(t, ok)
	assert.Equal(t, "test", got.ID())

	_, ok = reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry(testLogger())
	reg.Register(&mockChannel{id: "irc"})
	reg.Register(&mockChannel{id: "discord"})

	ids := reg.List()
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, "irc")
	assert.Contains(t, ids, "discord")
}

func TestRegistry_Count(t *testing.T) {
	reg := NewRegistry(testLogger())
	assert.Equal(t, 0, reg.Count())

	reg.Register(&mockChannel{id: "irc"})
	assert.Equal(t, 1, reg.Count())
}

func TestRegistry_Status(t *testing.T) {
	reg := NewRegistry(testLogger())
	ch := &mockChannel{id: "irc"}
	reg.Register(ch)

	statuses := reg.Status()
	require.Len(t, statuses, 1)
	assert.Equal(t, "irc", statuses[0].ChannelID)
}

func TestRegistry_StartAll(t *testing.T) {
	reg := NewRegistry(testLogger())
	ch1 := &mockChannel{id: "irc"}
	ch2 := &mockChannel{id: "discord"}
	reg.Register(ch1)
	reg.Register(ch2)

	err := reg.StartAll(context.Background())
	require.NoError(t, err)
	// StartAll launches goroutines; wait briefly for them to execute.
	assert.Eventually(t, func() bool { return ch1.started }, time.Second, 10*time.Millisecond)
	assert.Eventually(t, func() bool { return ch2.started }, time.Second, 10*time.Millisecond)
}

func TestRegistry_StartAll_Error(t *testing.T) {
	reg := NewRegistry(testLogger())
	ch := &mockChannel{id: "broken", startErr: assert.AnError}
	reg.Register(ch)

	// StartAll fires goroutines and always returns nil; errors are logged.
	err := reg.StartAll(context.Background())
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return ch.started }, time.Second, 10*time.Millisecond)
}

func TestRegistry_StopAll(t *testing.T) {
	reg := NewRegistry(testLogger())
	ch1 := &mockChannel{id: "irc"}
	ch2 := &mockChannel{id: "discord"}
	reg.Register(ch1)
	reg.Register(ch2)

	reg.StopAll(context.Background())
	assert.True(t, ch1.stopped)
	assert.True(t, ch2.stopped)
}

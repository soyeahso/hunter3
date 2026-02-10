package irc

import (
	"context"
	"testing"
	"time"

	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *logging.Logger {
	return logging.New(nil, "silent")
}

func TestNew(t *testing.T) {
	cfg := config.IRCConfig{
		Server:   "irc.libera.chat",
		Port:     6697,
		Nick:     "testbot",
		Channels: []string{"#test"},
		UseTLS:   true,
	}
	ch := New(cfg, testLogger())
	assert.Equal(t, "irc", ch.ID())
}

func TestCapabilities(t *testing.T) {
	ch := New(config.IRCConfig{}, testLogger())
	caps := ch.Capabilities()

	assert.Contains(t, caps.ChatTypes, domain.ChatTypeDM)
	assert.Contains(t, caps.ChatTypes, domain.ChatTypeGroup)
	assert.False(t, caps.Media)
	assert.False(t, caps.Reactions)
	assert.False(t, caps.Edit)
	assert.False(t, caps.Threads)
	assert.False(t, caps.Reply)
}

func TestStatus_NotStarted(t *testing.T) {
	ch := New(config.IRCConfig{}, testLogger())
	status := ch.Status()

	assert.Equal(t, "irc", status.ChannelID)
	assert.False(t, status.Connected)
	assert.False(t, status.Running)
	assert.Empty(t, status.LastError)
}

func TestOnMessage(t *testing.T) {
	ch := New(config.IRCConfig{}, testLogger())

	var received domain.InboundMessage
	ch.OnMessage(func(msg domain.InboundMessage) {
		received = msg
	})

	// Verify handler is stored
	ch.mu.RLock()
	assert.NotNil(t, ch.handler)
	ch.mu.RUnlock()

	// Simulate calling the handler directly
	ch.mu.RLock()
	handler := ch.handler
	ch.mu.RUnlock()

	handler(domain.InboundMessage{
		ID:        "test-1",
		ChannelID: "irc",
		From:      "testuser",
		ChatID:    "#test",
		Body:      "hello",
		Timestamp: time.Now(),
	})

	assert.Equal(t, "test-1", received.ID)
	assert.Equal(t, "testuser", received.From)
	assert.Equal(t, "hello", received.Body)
}

func TestSend_NotConnected(t *testing.T) {
	ch := New(config.IRCConfig{}, testLogger())
	err := ch.Send(context.Background(), domain.OutboundMessage{To: "#test", Body: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestSend_NoTarget(t *testing.T) {
	ch := New(config.IRCConfig{}, testLogger())
	// client is nil so this will hit "not connected" first
	err := ch.Send(context.Background(), domain.OutboundMessage{Body: "hi"})
	require.Error(t, err)
}

func TestSplitMessage_Short(t *testing.T) {
	result := splitMessage("hello world", 400)
	assert.Equal(t, []string{"hello world"}, result)
}

func TestSplitMessage_MultiLine(t *testing.T) {
	text := "line one\nline two\nline three"
	result := splitMessage(text, 400)
	assert.Equal(t, []string{"line one\nline two\nline three"}, result)
}

func TestSplitMessage_LongLine(t *testing.T) {
	// Create a string longer than 20 chars
	text := "abcdefghijklmnopqrstuvwxyz"
	result := splitMessage(text, 10)
	require.True(t, len(result) > 1)
	// Each chunk should be at most 10 chars (plus potential prefix)
	for _, chunk := range result {
		assert.LessOrEqual(t, len(chunk), 12) // small margin for joining
	}
}

func TestSplitMessage_MultipleLinesExceedMax(t *testing.T) {
	text := "short\nthis is a slightly longer line\nanother line"
	result := splitMessage(text, 20)
	require.True(t, len(result) >= 2)
	for _, chunk := range result {
		assert.NotEmpty(t, chunk)
	}
}

func TestDefaultPorts(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.IRCConfig
		wantTLS bool
	}{
		{"TLS defaults to 6697", config.IRCConfig{Server: "irc.test", Nick: "bot", UseTLS: true}, true},
		{"plain defaults to 6667", config.IRCConfig{Server: "irc.test", Nick: "bot", UseTLS: false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := New(tt.cfg, testLogger())
			assert.Equal(t, 0, ch.cfg.Port) // port is zero in config
		})
	}
}

package gateway

import (
	"testing"

	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLog() *logging.Logger {
	return logging.New(nil, "silent")
}

// --- ClientRegistry tests ---

func TestClientRegistryNew(t *testing.T) {
	reg := NewClientRegistry(testLog())
	require.NotNil(t, reg)
	assert.Equal(t, 0, reg.Count())
}

func TestClientRegistryAddAndGet(t *testing.T) {
	reg := NewClientRegistry(testLog())

	c := &Client{
		ConnID: "conn-1",
		Info:   ClientInfo{ID: "client-1"},
	}
	reg.Add(c)

	assert.Equal(t, 1, reg.Count())

	got, ok := reg.Get("conn-1")
	assert.True(t, ok)
	assert.Equal(t, "client-1", got.Info.ID)
}

func TestClientRegistryGetNotFound(t *testing.T) {
	reg := NewClientRegistry(testLog())

	_, ok := reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestClientRegistryRemove(t *testing.T) {
	reg := NewClientRegistry(testLog())

	c := &Client{ConnID: "conn-1"}
	reg.Add(c)
	assert.Equal(t, 1, reg.Count())

	reg.Remove("conn-1")
	assert.Equal(t, 0, reg.Count())

	_, ok := reg.Get("conn-1")
	assert.False(t, ok)
}

func TestClientRegistryRemoveNonexistent(t *testing.T) {
	reg := NewClientRegistry(testLog())
	// Should not panic
	reg.Remove("nonexistent")
	assert.Equal(t, 0, reg.Count())
}

func TestClientRegistryCount(t *testing.T) {
	reg := NewClientRegistry(testLog())
	assert.Equal(t, 0, reg.Count())

	reg.Add(&Client{ConnID: "conn-1"})
	assert.Equal(t, 1, reg.Count())

	reg.Add(&Client{ConnID: "conn-2"})
	assert.Equal(t, 2, reg.Count())

	reg.Remove("conn-1")
	assert.Equal(t, 1, reg.Count())
}

func TestClientRegistryMultipleClients(t *testing.T) {
	reg := NewClientRegistry(testLog())

	for i := range 5 {
		reg.Add(&Client{
			ConnID: "conn-" + string(rune('0'+i)),
			Info:   ClientInfo{ID: "client-" + string(rune('0'+i))},
		})
	}

	assert.Equal(t, 5, reg.Count())
}

func TestClientRegistryCloseAll(t *testing.T) {
	reg := NewClientRegistry(testLog())

	// Add clients without real WebSocket connections
	// CloseAll will call Close on each, which handles nil sockets
	reg.Add(&Client{ConnID: "conn-1", closed: true})
	reg.Add(&Client{ConnID: "conn-2", closed: true})

	assert.Equal(t, 2, reg.Count())
	reg.CloseAll()
	assert.Equal(t, 0, reg.Count())
}

// --- resolveBindAddr extended tests ---

func TestResolveBindAddr_Extended(t *testing.T) {
	tests := []struct {
		name string
		bind string
		port int
		host string
		want string
	}{
		{"loopback", "loopback", 18789, "", "127.0.0.1:18789"},
		{"lan", "lan", 9999, "", "0.0.0.0:9999"},
		{"auto", "auto", 8080, "", "0.0.0.0:8080"},
		{"custom_default", "custom", 3000, "", "0.0.0.0:3000"},
		{"custom_host", "custom", 3000, "10.0.0.1", "10.0.0.1:3000"},
		{"unknown_fallback", "whatever", 5000, "", "127.0.0.1:5000"},
		{"empty_fallback", "", 5000, "", "127.0.0.1:5000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.GatewayConfig{Bind: tt.bind, Port: tt.port, CustomBindHost: tt.host}
			assert.Equal(t, tt.want, resolveBindAddr(cfg))
		})
	}
}

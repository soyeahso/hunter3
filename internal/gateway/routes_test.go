package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- isAllowedConfigPath tests ---

func TestIsAllowedConfigPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		// Allowed paths
		{"gateway.port", true},
		{"gateway.mode", true},
		{"gateway.bind", true},
		{"gateway.customBindHost", true},
		{"gateway.controlUi", true},
		{"logging", true},
		{"logging.level", true},
		{"session", true},
		{"session.scope", true},
		{"memory", true},
		{"channels.defaults", true},
		{"channels.defaults.opOnly", true},
		// Blocked paths (not in allowlist)
		{"gateway.auth", false},
		{"gateway.auth.mode", false},
		{"gateway.auth.token", false},
		{"gateway.auth.password", false},
		{"gateway.tls", false},
		{"gateway.tls.enabled", false},
		{"gateway.tls.certPath", false},
		{"gateway.tls.keyPath", false},
		{"gateway.llm", false},
		{"channels.irc", false},
		{"channels.irc.password", false},
		{"agent", false},
		{"models.providers", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, isAllowedConfigPath(tt.path))
		})
	}
}

// --- parseConfigPathForRPC tests ---

func TestParseConfigPathForRPC(t *testing.T) {
	tests := []struct {
		input   string
		want    []string
		wantErr bool
	}{
		{"gateway.port", []string{"gateway", "port"}, false},
		{"gateway.auth.mode", []string{"gateway", "auth", "mode"}, false},
		{"logging", []string{"logging"}, false},
		{"a.b.c.d", []string{"a", "b", "c", "d"}, false},
		{"", nil, true},
		{"gateway..port", nil, true},   // empty segment
		{".gateway.port", nil, true},   // leading dot
		{"gateway.port.", nil, true},    // trailing dot
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseConfigPathForRPC(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// --- getValueAtPathRPC tests ---

func TestGetValueAtPathRPC(t *testing.T) {
	root := map[string]any{
		"gateway": map[string]any{
			"port": 18789,
			"mode": "local",
			"auth": map[string]any{
				"mode": "token",
			},
		},
		"logging": map[string]any{
			"level": "info",
		},
	}

	tests := []struct {
		path []string
		want any
		ok   bool
	}{
		{[]string{"gateway", "port"}, 18789, true},
		{[]string{"gateway", "mode"}, "local", true},
		{[]string{"gateway", "auth", "mode"}, "token", true},
		{[]string{"logging", "level"}, "info", true},
		{[]string{"gateway"}, map[string]any{"port": 18789, "mode": "local", "auth": map[string]any{"mode": "token"}}, true},
		{[]string{"nonexistent"}, nil, false},
		{[]string{"gateway", "nonexistent"}, nil, false},
		{[]string{"gateway", "port", "sub"}, nil, false}, // port is int, not map
	}

	for _, tt := range tests {
		val, ok := getValueAtPathRPC(root, tt.path)
		assert.Equal(t, tt.ok, ok)
		if tt.ok {
			assert.Equal(t, tt.want, val)
		}
	}
}

// --- setValueAtPathRPC tests ---

func TestSetValueAtPathRPC(t *testing.T) {
	root := map[string]any{
		"gateway": map[string]any{
			"port": 18789,
		},
	}

	setValueAtPathRPC(root, []string{"gateway", "port"}, 9999)
	val, ok := getValueAtPathRPC(root, []string{"gateway", "port"})
	assert.True(t, ok)
	assert.Equal(t, 9999, val)
}

func TestSetValueAtPathRPC_CreatesIntermediateMaps(t *testing.T) {
	root := map[string]any{}

	setValueAtPathRPC(root, []string{"a", "b", "c"}, "deep")
	val, ok := getValueAtPathRPC(root, []string{"a", "b", "c"})
	assert.True(t, ok)
	assert.Equal(t, "deep", val)
}

func TestSetValueAtPathRPC_OverwritesNonMap(t *testing.T) {
	root := map[string]any{
		"gateway": "string-value",
	}

	setValueAtPathRPC(root, []string{"gateway", "port"}, 8080)
	val, ok := getValueAtPathRPC(root, []string{"gateway", "port"})
	assert.True(t, ok)
	assert.Equal(t, 8080, val)
}

func TestSetValueAtPathRPC_SingleSegment(t *testing.T) {
	root := map[string]any{}

	setValueAtPathRPC(root, []string{"version"}, "1.0.0")
	assert.Equal(t, "1.0.0", root["version"])
}

// --- Server Methods tests ---

func TestServerMethods(t *testing.T) {
	_, ts := testServer(t)
	_ = ts

	// testServer creates a server with registerRPCHandlers
	// The default handlers are: health, config.get, config.set, channels.status, session.list, chat.send
	// We already test this indirectly via WebSocket tests, so just verify it doesn't panic
}

// --- Config RPC sensitive path tests ---

func TestConfigGetSensitivePath(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	req, _ := NewRequest("req-10", "config.get", configGetParams{Key: "gateway.auth.token"})
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.False(t, *resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "forbidden", resp.Error.Code)
}

func TestConfigSetSensitivePath(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	req, _ := NewRequest("req-11", "config.set", configSetParams{Key: "gateway.auth.token", Value: "hacked"})
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.False(t, *resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "forbidden", resp.Error.Code)
}

func TestConfigGetTLSPath(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	req, _ := NewRequest("req-12", "config.get", configGetParams{Key: "gateway.tls.keyPath"})
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.False(t, *resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "forbidden", resp.Error.Code)
}

func TestConfigGetEmptyKey(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	req, _ := NewRequest("req-13", "config.get", configGetParams{Key: ""})
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.False(t, *resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "invalid_params", resp.Error.Code)
}

func TestConfigSetEmptyKey(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	req, _ := NewRequest("req-14", "config.set", configSetParams{Key: "", Value: "x"})
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.False(t, *resp.OK)
	assert.Equal(t, "invalid_params", resp.Error.Code)
}

func TestConfigGetNotFound(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	// Use an allowed prefix so the request reaches the lookup stage
	req, _ := NewRequest("req-15", "config.get", configGetParams{Key: "logging.nonexistent"})
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.False(t, *resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "not_found", resp.Error.Code)
}

func TestChannelsStatus(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	req, _ := NewRequest("req-16", "channels.status", nil)
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.True(t, *resp.OK)
}

func TestSessionList(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	req, _ := NewRequest("req-17", "session.list", nil)
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.True(t, *resp.OK)
}

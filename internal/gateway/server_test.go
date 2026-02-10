package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/soyeahso/hunter3/internal/agent"
	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/llm"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	cfg := config.Defaults()
	cfg.Gateway.Auth.Mode = "token"
	cfg.Gateway.Auth.Token = "test-token-123"

	log := logging.New(nil, "silent")
	raw := map[string]any{
		"gateway": map[string]any{
			"port": 18789,
			"mode": "local",
		},
	}

	srv := New(cfg, log, WithConfigRaw(raw))

	mux := http.NewServeMux()
	srv.registerHTTPRoutes(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return srv, ts
}

func TestHealthEndpoint(t *testing.T) {
	_, ts := testServer(t)

	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health HealthResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&health))
	assert.Equal(t, "ok", health.Status)
	// Public endpoint only returns status; no version, clients, or uptime
	assert.Empty(t, health.Version)
}

func TestNotFoundEndpoint(t *testing.T) {
	_, ts := testServer(t)

	resp, err := http.Get(ts.URL + "/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestWebSocketHandshakeSuccess(t *testing.T) {
	srv, ts := testServer(t)
	_ = srv

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()
	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	// Read challenge event
	var challenge Frame
	err = conn.ReadJSON(&challenge)
	require.NoError(t, err)
	assert.Equal(t, FrameTypeEvent, challenge.Type)
	assert.Equal(t, "connect.challenge", challenge.Event)

	// Send connect request
	connectReq, err := NewRequest("req-1", "connect", ConnectParams{
		MinProtocol: 1,
		MaxProtocol: 1,
		Client: ClientInfo{
			ID:       "test-client",
			Version:  "1.0.0",
			Platform: "linux",
			Mode:     "app",
		},
		Auth: &ConnectAuth{Token: "test-token-123"},
	})
	require.NoError(t, err)
	require.NoError(t, conn.WriteJSON(connectReq))

	// Read hello-ok response
	var helloResp Frame
	err = conn.ReadJSON(&helloResp)
	require.NoError(t, err)
	assert.Equal(t, FrameTypeResponse, helloResp.Type)
	assert.Equal(t, "req-1", helloResp.ID)
	require.NotNil(t, helloResp.OK)
	assert.True(t, *helloResp.OK)

	// Parse hello payload
	var hello HelloOK
	require.NoError(t, json.Unmarshal(helloResp.Payload, &hello))
	assert.Equal(t, ProtocolVersion, hello.Protocol)
	assert.NotEmpty(t, hello.Server.ConnID)
	assert.NotEmpty(t, hello.Features.Methods)
	assert.Greater(t, hello.Policy.MaxPayload, 0)
}

func TestWebSocketHandshakeWrongToken(t *testing.T) {
	_, ts := testServer(t)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Read challenge
	var challenge Frame
	require.NoError(t, conn.ReadJSON(&challenge))

	// Send connect with wrong token
	connectReq, _ := NewRequest("req-1", "connect", ConnectParams{
		MinProtocol: 1,
		MaxProtocol: 1,
		Client: ClientInfo{
			ID:       "test-client",
			Version:  "1.0.0",
			Platform: "linux",
			Mode:     "app",
		},
		Auth: &ConnectAuth{Token: "wrong-token"},
	})
	require.NoError(t, conn.WriteJSON(connectReq))

	// Should get error response
	var errResp Frame
	err = conn.ReadJSON(&errResp)
	require.NoError(t, err)
	assert.Equal(t, FrameTypeResponse, errResp.Type)
	require.NotNil(t, errResp.OK)
	assert.False(t, *errResp.OK)
	require.NotNil(t, errResp.Error)
	assert.Equal(t, "unauthorized", errResp.Error.Code)
}

func TestWebSocketRPCHealth(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	// Send health RPC request
	req, _ := NewRequest("req-2", "health", nil)
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	assert.Equal(t, FrameTypeResponse, resp.Type)
	assert.Equal(t, "req-2", resp.ID)
	require.NotNil(t, resp.OK)
	assert.True(t, *resp.OK)

	var health HealthResponse
	require.NoError(t, json.Unmarshal(resp.Payload, &health))
	assert.Equal(t, "ok", health.Status)
}

func TestWebSocketRPCConfigGet(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	req, _ := NewRequest("req-3", "config.get", configGetParams{Key: "gateway.port"})
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.True(t, *resp.OK)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Payload, &result))
	assert.Equal(t, "gateway.port", result["key"])
	assert.Equal(t, float64(18789), result["value"])
}

func TestWebSocketRPCConfigSet(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	req, _ := NewRequest("req-4", "config.set", configSetParams{Key: "gateway.mode", Value: "remote"})
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.True(t, *resp.OK)

	// Verify with get
	req2, _ := NewRequest("req-5", "config.get", configGetParams{Key: "gateway.mode"})
	require.NoError(t, conn.WriteJSON(req2))

	var resp2 Frame
	require.NoError(t, conn.ReadJSON(&resp2))
	require.NotNil(t, resp2.OK)
	assert.True(t, *resp2.OK)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp2.Payload, &result))
	assert.Equal(t, "remote", result["value"])
}

func TestWebSocketRPCUnknownMethod(t *testing.T) {
	conn := authenticatedConn(t)
	defer conn.Close()

	req, _ := NewRequest("req-6", "nonexistent.method", nil)
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.False(t, *resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "method_not_found", resp.Error.Code)
}

func TestResolveAuth(t *testing.T) {
	auth := ResolveAuth(config.GatewayAuth{
		Mode:  "token",
		Token: "my-token",
	})
	assert.Equal(t, "token", auth.Mode)
	assert.Equal(t, "my-token", auth.Token)
}

func TestResolveAuthDefaultsToPassword(t *testing.T) {
	auth := ResolveAuth(config.GatewayAuth{
		Password: "my-pass",
	})
	assert.Equal(t, "password", auth.Mode)
	assert.Equal(t, "my-pass", auth.Password)
}

func TestResolveAuthEnvOverride(t *testing.T) {
	t.Setenv("HUNTER3_GATEWAY_TOKEN", "env-token")
	auth := ResolveAuth(config.GatewayAuth{Mode: "token"})
	assert.Equal(t, "env-token", auth.Token)
}

func TestAuthorizeTokenSuccess(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "token", Token: "secret"},
		&ConnectAuth{Token: "secret"},
	)
	assert.True(t, result.OK)
	assert.Equal(t, "token", result.Method)
}

func TestAuthorizeTokenFail(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "token", Token: "secret"},
		&ConnectAuth{Token: "wrong"},
	)
	assert.False(t, result.OK)
	assert.Equal(t, "token_mismatch", result.Reason)
}

func TestAuthorizePasswordSuccess(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "password", Password: "pass123"},
		&ConnectAuth{Password: "pass123"},
	)
	assert.True(t, result.OK)
	assert.Equal(t, "password", result.Method)
}

func TestAuthorizeNoCredentials(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "token", Token: "secret"},
		nil,
	)
	assert.False(t, result.OK)
}

func TestResolveBindAddr(t *testing.T) {
	tests := []struct {
		bind string
		port int
		want string
	}{
		{"loopback", 18789, "127.0.0.1:18789"},
		{"lan", 9999, "0.0.0.0:9999"},
		{"auto", 8080, "0.0.0.0:8080"},
		{"custom", 3000, "0.0.0.0:3000"},
		{"unknown", 5000, "127.0.0.1:5000"},
	}

	for _, tt := range tests {
		t.Run(tt.bind, func(t *testing.T) {
			addr := resolveBindAddr(config.GatewayConfig{Bind: tt.bind, Port: tt.port})
			assert.Equal(t, tt.want, addr)
		})
	}
}

func TestServerStart(t *testing.T) {
	cfg := config.Defaults()
	cfg.Gateway.Port = 0 // let OS pick a port
	cfg.Gateway.Auth.Mode = "token"
	cfg.Gateway.Auth.Token = "test-token"

	log := logging.New(nil, "silent")
	srv := New(cfg, log)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop it
	cancel()

	err := <-errCh
	assert.NoError(t, err)
}

func testServerWithRunner(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	cfg := config.Defaults()
	cfg.Gateway.Auth.Mode = "token"
	cfg.Gateway.Auth.Token = "test-token-123"

	log := logging.New(nil, "silent")
	raw := map[string]any{
		"gateway": map[string]any{"port": 18789, "mode": "local"},
	}

	mock := &llm.MockClient{
		ProviderName: "mock",
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			return &llm.CompletionResponse{
				Content: "I am a mock agent. You said: " + req.Messages[len(req.Messages)-1].Content,
				Model:   "mock-model",
				Usage:   llm.Usage{InputTokens: 10, OutputTokens: 20},
				CostUSD: 0.001,
			}, nil
		},
	}

	reg := llm.NewRegistry(log)
	reg.Register("mock", mock)
	reg.SetFallback("mock")

	runner := agent.NewRunner(
		agent.RunnerConfig{AgentID: "test", AgentName: "TestBot", Model: "mock"},
		reg,
		agent.NewMemorySessionStore(),
		agent.NewToolRegistry(),
		log,
	)

	srv := New(cfg, log, WithConfigRaw(raw), WithRunner(runner))

	mux := http.NewServeMux()
	srv.registerHTTPRoutes(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return srv, ts
}

func authenticatedConnWithRunner(t *testing.T) *websocket.Conn {
	t.Helper()
	_, ts := testServerWithRunner(t)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	var challenge Frame
	require.NoError(t, conn.ReadJSON(&challenge))

	connectReq, _ := NewRequest("auth-req", "connect", ConnectParams{
		MinProtocol: 1, MaxProtocol: 1,
		Client: ClientInfo{ID: "test-client", Version: "1.0.0", Platform: "linux", Mode: "app"},
		Auth:   &ConnectAuth{Token: "test-token-123"},
	})
	require.NoError(t, conn.WriteJSON(connectReq))

	var helloResp Frame
	require.NoError(t, conn.ReadJSON(&helloResp))
	require.NotNil(t, helloResp.OK)
	require.True(t, *helloResp.OK)

	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestChatSendRPC(t *testing.T) {
	conn := authenticatedConnWithRunner(t)
	defer conn.Close()

	req, _ := NewRequest("chat-1", "chat.send", chatSendParams{
		Message: "Hello bot!",
	})
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	assert.Equal(t, "chat-1", resp.ID)
	require.NotNil(t, resp.OK)
	assert.True(t, *resp.OK)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Payload, &result))
	assert.Contains(t, result["response"], "Hello bot!")
	assert.NotEmpty(t, result["sessionId"])
	assert.Equal(t, "mock-model", result["model"])
}

func TestChatSendNoRunner(t *testing.T) {
	conn := authenticatedConn(t) // uses testServer (no runner)
	defer conn.Close()

	req, _ := NewRequest("chat-2", "chat.send", chatSendParams{
		Message: "Hello",
	})
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.False(t, *resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "unavailable", resp.Error.Code)
}

func TestChatSendEmptyMessage(t *testing.T) {
	conn := authenticatedConnWithRunner(t)
	defer conn.Close()

	req, _ := NewRequest("chat-3", "chat.send", chatSendParams{Message: ""})
	require.NoError(t, conn.WriteJSON(req))

	var resp Frame
	require.NoError(t, conn.ReadJSON(&resp))
	require.NotNil(t, resp.OK)
	assert.False(t, *resp.OK)
	assert.Equal(t, "invalid_params", resp.Error.Code)
}

// authenticatedConn returns a WebSocket connection that has completed the handshake.
func authenticatedConn(t *testing.T) *websocket.Conn {
	t.Helper()
	_, ts := testServer(t)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Read challenge
	var challenge Frame
	require.NoError(t, conn.ReadJSON(&challenge))

	// Send connect
	connectReq, _ := NewRequest("auth-req", "connect", ConnectParams{
		MinProtocol: 1,
		MaxProtocol: 1,
		Client: ClientInfo{
			ID:       "test-client",
			Version:  "1.0.0",
			Platform: "linux",
			Mode:     "app",
		},
		Auth: &ConnectAuth{Token: "test-token-123"},
	})
	require.NoError(t, conn.WriteJSON(connectReq))

	// Read hello-ok
	var helloResp Frame
	require.NoError(t, conn.ReadJSON(&helloResp))
	require.NotNil(t, helloResp.OK)
	require.True(t, *helloResp.OK, "handshake should succeed")

	t.Cleanup(func() { conn.Close() })
	return conn
}

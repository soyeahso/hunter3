package gateway

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/soyeahso/hunter3/internal/agent"
	"github.com/soyeahso/hunter3/internal/channel"
	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/hooks"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/soyeahso/hunter3/internal/version"
)

var (
	ErrClientClosed    = errors.New("client connection closed")
	ErrEmptyConfigPath = errors.New("empty config path")
)

// Server is the Hunter3 gateway HTTP + WebSocket server.
type Server struct {
	cfg       config.Config
	auth      ResolvedAuth
	log       *logging.Logger
	clients   *ClientRegistry
	handlers  map[string]RequestHandler
	version   string
	eventSeq  atomic.Int64

	mu        sync.RWMutex
	configRaw map[string]any

	// Agent runner (optional — nil if no LLM provider is available)
	runner    *agent.Runner

	// Channel registry (optional — nil if no channels configured)
	channels  *channel.Registry

	// Hook manager (optional — nil if not configured)
	hooks     *hooks.Manager

	startedAt   time.Time
	httpServer  *http.Server
	upgrader    websocket.Upgrader
	authLimiter *authRateLimiter
}

// authRateLimiter tracks failed auth attempts per IP to prevent brute-force attacks.
type authRateLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

const (
	authRateWindow   = 5 * time.Minute
	authRateMaxFails = 10
	authRateMaxIPs   = 10000 // max tracked IPs to prevent memory exhaustion
)

func newAuthRateLimiter() *authRateLimiter {
	rl := &authRateLimiter{failures: make(map[string][]time.Time)}
	go rl.periodicCleanup()
	return rl
}

// periodicCleanup removes stale entries every minute.
func (l *authRateLimiter) periodicCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		cutoff := time.Now().Add(-authRateWindow)
		for ip, times := range l.failures {
			filtered := times[:0]
			for _, t := range times {
				if t.After(cutoff) {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) == 0 {
				delete(l.failures, ip)
			} else {
				l.failures[ip] = filtered
			}
		}
		l.mu.Unlock()
	}
}

func (l *authRateLimiter) allow(remoteAddr string) bool {
	host, _, _ := net.SplitHostPort(remoteAddr)
	if host == "" {
		host = remoteAddr
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-authRateWindow)
	recent := l.failures[host]
	filtered := recent[:0]
	for _, t := range recent {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		delete(l.failures, host)
		return true
	}
	l.failures[host] = filtered
	return len(filtered) < authRateMaxFails
}

func (l *authRateLimiter) recordFailure(remoteAddr string) {
	host, _, _ := net.SplitHostPort(remoteAddr)
	if host == "" {
		host = remoteAddr
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Enforce max entries cap to prevent memory exhaustion from DDoS
	if _, exists := l.failures[host]; !exists && len(l.failures) >= authRateMaxIPs {
		var oldestIP string
		var oldestTime time.Time
		for ip, times := range l.failures {
			if len(times) > 0 && (oldestIP == "" || times[0].Before(oldestTime)) {
				oldestIP = ip
				oldestTime = times[0]
			}
		}
		if oldestIP != "" {
			delete(l.failures, oldestIP)
		}
	}

	l.failures[host] = append(l.failures[host], time.Now())
}

// ServerOption configures the gateway server.
type ServerOption func(*Server)

// WithConfigRaw sets the raw config map for RPC access.
func WithConfigRaw(raw map[string]any) ServerOption {
	return func(s *Server) {
		s.configRaw = raw
	}
}

// WithChannels sets the channel registry for channel status reporting.
func WithChannels(ch *channel.Registry) ServerOption {
	return func(s *Server) {
		s.channels = ch
	}
}

// WithHooks sets the hook manager for lifecycle events.
func WithHooks(hm *hooks.Manager) ServerOption {
	return func(s *Server) {
		s.hooks = hm
	}
}

// WithRunner sets the agent runner for chat.send handling.
func WithRunner(r *agent.Runner) ServerOption {
	return func(s *Server) {
		s.runner = r
	}
}

// New creates a new gateway server.
func New(cfg config.Config, log *logging.Logger, opts ...ServerOption) *Server {
	allowedOrigins := cfg.Gateway.ControlUI.AllowedOrigins
	s := &Server{
		cfg:         cfg,
		auth:        ResolveAuth(cfg.Gateway.Auth),
		log:         log.Sub("gateway"),
		clients:     NewClientRegistry(log.Sub("clients")),
		handlers:    make(map[string]RequestHandler),
		version:     version.Version,
		configRaw:   make(map[string]any),
		authLimiter: newAuthRateLimiter(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     checkWebSocketOrigin(allowedOrigins),
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	s.registerRPCHandlers()
	return s
}

// checkWebSocketOrigin returns a function that validates WebSocket Origin headers.
// If no origins are configured, only same-origin (no Origin header) or non-browser
// clients are allowed. If origins are configured, the Origin must match one of them.
func checkWebSocketOrigin(allowed []string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Same-origin or non-browser clients
		}
		for _, a := range allowed {
			if a == "*" || a == origin {
				return true
			}
		}
		return false
	}
}

// Handle registers an RPC method handler.
func (s *Server) Handle(method string, handler RequestHandler) {
	s.handlers[method] = handler
}

// Methods returns the list of registered RPC method names.
func (s *Server) Methods() []string {
	methods := make([]string, 0, len(s.handlers))
	for m := range s.handlers {
		methods = append(methods, m)
	}
	return methods
}

// resolveBindAddr computes the listen address from config.
func resolveBindAddr(cfg config.GatewayConfig) string {
	switch cfg.Bind {
	case "loopback":
		return fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	case "lan", "auto":
		return fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	case "custom":
		host := cfg.CustomBindHost
		if host == "" {
			host = "0.0.0.0"
		}
		return fmt.Sprintf("%s:%d", host, cfg.Port)
	default:
		return fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	}
}

// Start begins listening for HTTP and WebSocket connections.
// It blocks until the context is cancelled or an error occurs.
func (s *Server) Start(ctx context.Context) error {
	addr := resolveBindAddr(s.cfg.Gateway)

	mux := http.NewServeMux()
	s.registerHTTPRoutes(mux)

	handler := withMiddleware(mux, s.log, s.cfg.Gateway.ControlUI.AllowedOrigins)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		BaseContext:  func(l net.Listener) context.Context { return ctx },
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Enable TLS if configured
	if s.cfg.Gateway.TLS.Enabled {
		cert, err := tls.LoadX509KeyPair(s.cfg.Gateway.TLS.CertPath, s.cfg.Gateway.TLS.KeyPath)
		if err != nil {
			ln.Close()
			return fmt.Errorf("loading TLS certificate: %w", err)
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		ln = tls.NewListener(ln, tlsCfg)
		s.log.Info().Msg("TLS enabled")
	} else if s.cfg.Gateway.Bind != "loopback" {
		s.log.Warn().Msg("TLS is not enabled — credentials will be transmitted in cleartext")
	}

	s.startedAt = time.Now()

	s.log.Info().
		Str("addr", ln.Addr().String()).
		Str("bind", s.cfg.Gateway.Bind).
		Str("auth", s.auth.Mode).
		Int("methods", len(s.handlers)).
		Msg("gateway server starting")

	if s.hooks != nil {
		s.hooks.Emit(ctx, hooks.EventGatewayStart, map[string]any{
			"addr": ln.Addr().String(),
		})
	}

	s.log.Info().
		Str("addr", ln.Addr().String()).
		Msg("gateway server ready")

	// Shutdown when context is cancelled
	go func() {
		<-ctx.Done()
		s.log.Info().Msg("shutting down gateway server")
		if s.hooks != nil {
			s.hooks.Emit(context.Background(), hooks.EventGatewayStop, nil)
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.clients.CloseAll()
		s.httpServer.Shutdown(shutdownCtx)
	}()

	if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Addr returns the server's listen address, or empty string if not started.
func (s *Server) Addr() string {
	if s.httpServer != nil {
		return s.httpServer.Addr
	}
	return ""
}

// handleWebSocket upgrades HTTP to WebSocket and runs the connection loop.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Rate-limit connection attempts per IP
	if !s.authLimiter.allow(r.RemoteAddr) {
		s.log.Warn().Str("remote", r.RemoteAddr).Msg("rate limited — too many failed auth attempts")
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Error().Err(err).Msg("websocket upgrade failed")
		return
	}

	// Enforce the advertised max payload size
	conn.SetReadLimit(4 * 1024 * 1024) // 4MB

	s.log.Debug().Str("remote", r.RemoteAddr).Msg("new websocket connection")

	// Run handshake with timeout
	client, err := s.handshake(conn)
	if err != nil {
		s.log.Warn().Err(err).Msg("handshake failed")
		s.authLimiter.recordFailure(conn.RemoteAddr().String())
		conn.Close()
		return
	}

	s.clients.Add(client)
	defer func() {
		s.clients.Remove(client.ConnID)
		client.Close()
	}()

	// Message read loop
	s.readLoop(client)
}

// handshake performs the WebSocket authentication handshake.
// Flow: server sends challenge → client sends connect → server validates → sends hello-ok.
func (s *Server) handshake(conn *websocket.Conn) (*Client, error) {
	// Set a read deadline for the handshake
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// Send challenge
	nonce := uuid.New().String()
	challenge, err := NewEvent("connect.challenge", map[string]any{
		"nonce": nonce,
		"ts":    time.Now().UnixMilli(),
	}, 0)
	if err != nil {
		return nil, fmt.Errorf("creating challenge: %w", err)
	}
	if err := conn.WriteJSON(challenge); err != nil {
		return nil, fmt.Errorf("sending challenge: %w", err)
	}

	// Read connect request
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("reading connect: %w", err)
	}

	var frame Frame
	if err := json.Unmarshal(msg, &frame); err != nil {
		return nil, fmt.Errorf("parsing connect frame: %w", err)
	}

	if frame.Type != FrameTypeRequest || frame.Method != "connect" {
		sendErrorAndClose(conn, frame.ID, "protocol_error", "expected connect request")
		return nil, fmt.Errorf("expected connect request, got type=%s method=%s", frame.Type, frame.Method)
	}

	var params ConnectParams
	if err := json.Unmarshal(frame.Params, &params); err != nil {
		sendErrorAndClose(conn, frame.ID, "invalid_params", "invalid connect params")
		return nil, fmt.Errorf("parsing connect params: %w", err)
	}

	// Authenticate
	authResult := Authorize(s.auth, params.Auth)
	if !authResult.OK {
		sendErrorAndClose(conn, frame.ID, "unauthorized", authResult.Reason)
		return nil, fmt.Errorf("auth failed: %s", authResult.Reason)
	}

	// Clear the read deadline for post-handshake
	conn.SetReadDeadline(time.Time{})

	client := NewClient(conn, params.Client, authResult, s.log.Sub("ws"))

	// Send hello-ok
	hello := HelloOK{
		Protocol: ProtocolVersion,
		Server: ServerInfo{
			Version: s.version,
			Commit:  version.Commit,
			ConnID:  client.ConnID,
		},
		Features: Features{
			Methods: s.Methods(),
			Events:  []string{"connect.challenge", "chat.delta", "chat.event", "channels.status"},
		},
		Policy: ServerPolicy{
			MaxPayload:       4 * 1024 * 1024, // 4MB
			MaxBufferedBytes: 16 * 1024 * 1024, // 16MB
			TickIntervalMs:   30000,
		},
	}

	resp, err := NewResponse(frame.ID, hello)
	if err != nil {
		return nil, fmt.Errorf("creating hello response: %w", err)
	}
	if err := conn.WriteJSON(resp); err != nil {
		return nil, fmt.Errorf("sending hello: %w", err)
	}

	s.log.Info().
		Str("connId", client.ConnID).
		Str("clientId", params.Client.ID).
		Str("clientVersion", params.Client.Version).
		Str("authMethod", authResult.Method).
		Msg("client authenticated")

	return client, nil
}

// readLoop processes incoming frames from an authenticated client.
func (s *Server) readLoop(client *Client) {
	for {
		frame, err := client.ReadFrame()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				s.log.Debug().Str("connId", client.ConnID).Msg("client closed connection")
			} else {
				s.log.Warn().Err(err).Str("connId", client.ConnID).Msg("read error")
			}
			return
		}

		if frame.Type != FrameTypeRequest {
			s.log.Debug().Str("type", frame.Type).Msg("ignoring non-request frame")
			continue
		}

		s.dispatch(client, frame)
	}
}

// dispatch routes a request frame to the appropriate handler.
func (s *Server) dispatch(client *Client, frame Frame) {
	handler, ok := s.handlers[frame.Method]
	if !ok {
		client.RespondError(frame.ID, ErrorShape{
			Code:    "method_not_found",
			Message: "unknown method: " + frame.Method,
		})
		return
	}

	rc := &RequestContext{
		Client: client,
		Frame:  frame,
		Server: s,
	}

	// Run handler (could be made async with goroutines if needed)
	handler(rc)
}

// sendErrorAndClose sends an error response and closes the connection.
func sendErrorAndClose(conn *websocket.Conn, reqID, code, message string) {
	errFrame := NewErrorResponse(reqID, ErrorShape{
		Code:    code,
		Message: message,
	})
	conn.WriteJSON(errFrame)
	conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, message))
}

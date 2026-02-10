package gateway

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/llm"
)

// safeConfigPrefixes lists config path prefixes that can be read and
// written via RPC. All other paths are denied by default (allowlist).
var safeConfigPrefixes = []string{
	"gateway.port",
	"gateway.mode",
	"gateway.bind",
	"gateway.customBindHost",
	"gateway.controlUi",
	"logging",
	"session",
	"memory",
	"channels.defaults",
}

func isAllowedConfigPath(key string) bool {
	for _, prefix := range safeConfigPrefixes {
		if key == prefix || strings.HasPrefix(key, prefix+".") {
			return true
		}
	}
	return false
}

// llmCallTimeout is the maximum duration for an LLM completion call.
const llmCallTimeout = 5 * time.Minute

// registerHTTPRoutes sets up all HTTP routes on the server mux.
func (s *Server) registerHTTPRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ws", s.handleWebSocket)

	// Catch-all for unknown routes
	mux.HandleFunc("/", handleNotFound)
}

// registerRPCHandlers sets up all JSON-RPC method handlers.
func (s *Server) registerRPCHandlers() {
	s.Handle("health", s.rpcHealth)
	s.Handle("config.get", s.rpcConfigGet)
	s.Handle("config.set", s.rpcConfigSet)
	s.Handle("channels.status", s.rpcChannelsStatus)
	s.Handle("session.list", s.rpcSessionList)
	s.Handle("chat.send", s.rpcChatSend)
}

// Built-in RPC handlers

func (s *Server) rpcHealth(rc *RequestContext) {
	rc.Respond(HealthResponse{
		Status:  "ok",
		Version: s.version,
		Clients: s.clients.Count(),
	})
}

type configGetParams struct {
	Key string `json:"key"`
}

func (s *Server) rpcConfigGet(rc *RequestContext) {
	var p configGetParams
	if err := rc.Params(&p); err != nil {
		rc.RespondError("invalid_params", err.Error())
		return
	}
	if p.Key == "" {
		rc.RespondError("invalid_params", "key is required")
		return
	}
	if !isAllowedConfigPath(p.Key) {
		rc.RespondError("forbidden", "access denied for config path: "+p.Key)
		return
	}

	s.mu.RLock()
	raw := s.configRaw
	s.mu.RUnlock()

	path, err := parseConfigPathForRPC(p.Key)
	if err != nil {
		rc.RespondError("invalid_params", err.Error())
		return
	}

	val, ok := getValueAtPathRPC(raw, path)
	if !ok {
		rc.RespondError("not_found", "key not found: "+p.Key)
		return
	}
	rc.Respond(map[string]any{"key": p.Key, "value": val})
}

type configSetParams struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func (s *Server) rpcConfigSet(rc *RequestContext) {
	var p configSetParams
	if err := rc.Params(&p); err != nil {
		rc.RespondError("invalid_params", err.Error())
		return
	}
	if p.Key == "" {
		rc.RespondError("invalid_params", "key is required")
		return
	}
	if !isAllowedConfigPath(p.Key) {
		rc.RespondError("forbidden", "cannot modify config path: "+p.Key)
		return
	}

	path, err := parseConfigPathForRPC(p.Key)
	if err != nil {
		rc.RespondError("invalid_params", err.Error())
		return
	}

	s.mu.Lock()
	setValueAtPathRPC(s.configRaw, path, p.Value)
	s.mu.Unlock()

	rc.Respond(map[string]any{"key": p.Key, "value": p.Value})
}

func (s *Server) rpcChannelsStatus(rc *RequestContext) {
	if s.channels != nil {
		rc.Respond(map[string]any{"channels": s.channels.Status()})
		return
	}
	rc.Respond(map[string]any{"channels": []any{}})
}

func (s *Server) rpcSessionList(rc *RequestContext) {
	rc.Respond(map[string]any{"sessions": []any{}})
}

type chatSendParams struct {
	Message   string `json:"message"`
	SessionID string `json:"sessionId,omitempty"`
	ChannelID string `json:"channelId,omitempty"`
	ChatID    string `json:"chatId,omitempty"`
	Stream    bool   `json:"stream,omitempty"`
}

func (s *Server) rpcChatSend(rc *RequestContext) {
	if s.runner == nil {
		rc.RespondError("unavailable", "no LLM provider configured")
		return
	}

	var p chatSendParams
	if err := rc.Params(&p); err != nil {
		rc.RespondError("invalid_params", err.Error())
		return
	}
	if p.Message == "" {
		rc.RespondError("invalid_params", "message is required")
		return
	}

	msg := domain.InboundMessage{
		ID:        rc.Frame.ID,
		ChannelID: p.ChannelID,
		ChatID:    p.ChatID,
		From:      rc.Client.ConnID,
		FromName:  rc.Client.Info.DisplayName,
		ChatType:  domain.ChatTypeDM,
		Body:      p.Message,
		Timestamp: time.Now(),
	}
	if msg.ChannelID == "" {
		msg.ChannelID = "gateway"
	}
	if msg.ChatID == "" {
		msg.ChatID = rc.Client.ConnID
	}

	if p.Stream {
		s.handleStreamChat(rc, msg)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), llmCallTimeout)
	defer cancel()

	result, err := s.runner.Run(ctx, msg)
	if err != nil {
		rc.RespondError("agent_error", err.Error())
		return
	}

	rc.Respond(map[string]any{
		"response":  result.Response,
		"sessionId": result.SessionID,
		"model":     result.Model,
		"usage":     result.Usage,
		"costUsd":   result.CostUSD,
		"durationMs": result.Duration.Milliseconds(),
	})
}

func (s *Server) handleStreamChat(rc *RequestContext, msg domain.InboundMessage) {
	seq := s.eventSeq.Add(1)

	ctx, cancel := context.WithTimeout(context.Background(), llmCallTimeout)
	defer cancel()

	result, err := s.runner.RunStream(
		ctx,
		msg,
		func(evt llm.StreamEvent) {
			switch evt.Type {
			case "delta":
				rc.Client.SendEvent("chat.delta", map[string]any{
					"requestId": rc.Frame.ID,
					"content":   evt.Content,
				}, seq)
				seq = s.eventSeq.Add(1)
			}
		},
	)

	if err != nil {
		rc.RespondError("agent_error", err.Error())
		return
	}

	rc.Respond(map[string]any{
		"response":  result.Response,
		"sessionId": result.SessionID,
		"model":     result.Model,
		"usage":     result.Usage,
		"costUsd":   result.CostUSD,
		"durationMs": result.Duration.Milliseconds(),
	})
}

// Helpers that mirror config.ParseConfigPath / GetValueAtPath without importing config
// to avoid circular dependencies — they operate on raw maps only.

func parseConfigPathForRPC(raw string) ([]string, error) {
	// Delegate to config package logic inline (simple split).
	if raw == "" {
		return nil, ErrEmptyConfigPath
	}
	var parts []string
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i == len(raw) || raw[i] == '.' {
			if i == start {
				return nil, ErrEmptyConfigPath
			}
			parts = append(parts, raw[start:i])
			start = i + 1
		}
	}
	return parts, nil
}

func getValueAtPathRPC(root map[string]any, path []string) (any, bool) {
	current := any(root)
	for _, key := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func setValueAtPathRPC(root map[string]any, path []string, value any) {
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key]
		if !ok {
			next = map[string]any{}
			current[key] = next
		}
		m, ok := next.(map[string]any)
		if !ok {
			m = map[string]any{}
			current[key] = m
		}
		current = m
	}
	current[path[len(path)-1]] = value
}

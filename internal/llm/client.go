// Package llm defines the LLM client interface and pluggable provider system.
//
// Instead of direct HTTP API clients, this package wraps CLI tools (like Claude Code's
// `claude` CLI or `gemini-cli`) as providers. This approach:
//   - Reuses existing auth, caching, and rate-limit logic in each CLI
//   - Stays current with API changes automatically via CLI updates
//   - Makes adding new providers trivial (just wrap another CLI)
package llm

import (
	"context"
	"time"
)

// Role constants for messages.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// Message is a single turn in a conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolDefinition describes a tool the LLM can invoke.
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema string `json:"inputSchema"` // JSON Schema string
}

// CompletionRequest is the input to a Complete or Stream call.
type CompletionRequest struct {
	Model       string           `json:"model,omitempty"`
	System      string           `json:"system,omitempty"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	MaxTokens   int              `json:"maxTokens,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

// CompletionResponse is the result of a non-streaming completion.
type CompletionResponse struct {
	Content    string      `json:"content"`
	StopReason string      `json:"stopReason,omitempty"`
	ToolCalls  []ToolCall  `json:"toolCalls,omitempty"`
	Usage      Usage       `json:"usage"`
	Model      string      `json:"model,omitempty"`
	SessionID  string      `json:"sessionId,omitempty"`
	Duration   time.Duration `json:"duration,omitempty"`
	CostUSD    float64     `json:"costUsd,omitempty"`
}

// ToolCall is an LLM request to invoke a tool.
type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"` // JSON string
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	CacheRead    int `json:"cacheReadInputTokens,omitempty"`
	CacheWrite   int `json:"cacheCreationInputTokens,omitempty"`
}

// StreamEvent is a chunk from a streaming completion.
type StreamEvent struct {
	Type    string `json:"type"`              // "delta", "done", "error"
	Content string `json:"content,omitempty"` // text delta
	Error   string `json:"error,omitempty"`   // error message (type="error")

	// Final fields (type="done")
	Response *CompletionResponse `json:"response,omitempty"`
}

// Client is the interface all LLM providers must implement.
type Client interface {
	// Complete sends a request and returns the full response.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Stream sends a request and returns a channel of streaming events.
	Stream(ctx context.Context, req CompletionRequest) (<-chan StreamEvent, error)

	// Name returns the provider name (e.g., "claude", "gemini").
	Name() string
}

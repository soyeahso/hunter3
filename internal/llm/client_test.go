package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func silentLog() *logging.Logger {
	return logging.New(nil, "silent")
}

// --- Registry tests ---

func TestRegistryRegisterAndResolve(t *testing.T) {
	reg := NewRegistry(silentLog())

	mock := &MockClient{ProviderName: "test-provider"}
	reg.Register("test-provider", mock)

	client, err := reg.Resolve("test-provider")
	require.NoError(t, err)
	assert.Equal(t, "test-provider", client.Name())
}

func TestRegistryAlias(t *testing.T) {
	reg := NewRegistry(silentLog())

	mock := &MockClient{ProviderName: "claude"}
	reg.Register("claude", mock)
	reg.Alias("sonnet", "claude")
	reg.Alias("opus", "claude")

	client, err := reg.Resolve("sonnet")
	require.NoError(t, err)
	assert.Equal(t, "claude", client.Name())

	client, err = reg.Resolve("opus")
	require.NoError(t, err)
	assert.Equal(t, "claude", client.Name())
}

func TestRegistryFallback(t *testing.T) {
	reg := NewRegistry(silentLog())

	mock := &MockClient{ProviderName: "default-llm"}
	reg.Register("default-llm", mock)
	reg.SetFallback("default-llm")

	// Unknown model should resolve to fallback
	client, err := reg.Resolve("unknown-model-xyz")
	require.NoError(t, err)
	assert.Equal(t, "default-llm", client.Name())
}

func TestRegistryResolveNotFound(t *testing.T) {
	reg := NewRegistry(silentLog())

	_, err := reg.Resolve("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no LLM provider")
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry(silentLog())
	reg.Register("a", &MockClient{ProviderName: "a"})
	reg.Register("b", &MockClient{ProviderName: "b"})

	names := reg.List()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "a")
	assert.Contains(t, names, "b")
}

func TestNewRegistryFromConfig(t *testing.T) {
	// This tests that config-based registration works (providers won't be found
	// in test environment since CLIs aren't installed, but the registry is created)
	cfg := config.ModelsConfig{
		Providers: map[string]config.ModelProviderEntry{
			"gemini": {API: "google-generative-ai"},
		},
	}
	reg := NewRegistryFromConfig(cfg, "", "", "", "", "", silentLog())
	assert.NotNil(t, reg)
}

// --- MockClient tests ---

func TestMockClientComplete(t *testing.T) {
	mock := &MockClient{
		ProviderName: "test",
		CompleteFunc: func(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
			return &CompletionResponse{
				Content: "The answer is 42",
				Usage:   Usage{InputTokens: 10, OutputTokens: 5},
			}, nil
		},
	}

	resp, err := mock.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "What is the answer?"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "The answer is 42", resp.Content)
	assert.Equal(t, 10, resp.Usage.InputTokens)
}

func TestMockClientStream(t *testing.T) {
	mock := &MockClient{ProviderName: "test"}

	ch, err := mock.Stream(context.Background(), CompletionRequest{})
	require.NoError(t, err)

	var events []StreamEvent
	for evt := range ch {
		events = append(events, evt)
	}

	assert.Len(t, events, 2)
	assert.Equal(t, "delta", events[0].Type)
	assert.Equal(t, "done", events[1].Type)
}

func TestMockClientCompleteError(t *testing.T) {
	mock := &MockClient{
		ProviderName: "test",
		CompleteFunc: func(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
			return nil, &ProviderError{Provider: "test", Message: "rate limited", Code: 429}
		},
	}

	_, err := mock.Complete(context.Background(), CompletionRequest{})
	assert.Error(t, err)

	var provErr *ProviderError
	assert.ErrorAs(t, err, &provErr)
	assert.Equal(t, 429, provErr.Code)
}

// --- Protocol parsing tests ---

func TestParseClaudeResponse(t *testing.T) {
	raw := `{
		"type": "result",
		"subtype": "success",
		"is_error": false,
		"result": "2 + 2 = 4",
		"session_id": "test-session",
		"total_cost_usd": 0.001,
		"usage": {
			"input_tokens": 10,
			"output_tokens": 5,
			"cache_read_input_tokens": 100,
			"cache_creation_input_tokens": 50
		}
	}`

	resp, err := parseClaudeResponse([]byte(raw))
	require.NoError(t, err)
	assert.Equal(t, "2 + 2 = 4", resp.Content)
	assert.Equal(t, "test-session", resp.SessionID)
	assert.Equal(t, 0.001, resp.CostUSD)
	assert.Equal(t, 10, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
	assert.Equal(t, 100, resp.Usage.CacheRead)
	assert.Equal(t, 50, resp.Usage.CacheWrite)
}

func TestParseClaudeResponseError(t *testing.T) {
	raw := `{"type":"result","subtype":"error","is_error":true,"result":"API key expired"}`

	_, err := parseClaudeResponse([]byte(raw))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key expired")
}

func TestParseClaudeStreamLine(t *testing.T) {
	// System init — should be skipped
	init := `{"type":"system","subtype":"init","session_id":"abc"}`
	evt, err := parseClaudeStreamLine([]byte(init))
	require.NoError(t, err)
	assert.Nil(t, evt)

	// Assistant message
	assistant := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello!"}]}}`
	evt, err = parseClaudeStreamLine([]byte(assistant))
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "delta", evt.Type)
	assert.Equal(t, "Hello!", evt.Content)

	// Result
	result := `{"type":"result","result":"Full response","session_id":"abc","total_cost_usd":0.01,"usage":{"input_tokens":10,"output_tokens":20}}`
	evt, err = parseClaudeStreamLine([]byte(result))
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "done", evt.Type)
	assert.Equal(t, "Full response", evt.Response.Content)
	assert.Equal(t, 20, evt.Response.Usage.OutputTokens)
}

func TestParseExternalResponse(t *testing.T) {
	ecfg := ExternalCLIConfig{Name: "test", ResultField: "result"}

	// Standard JSON
	raw := `{"result": "hello world"}`
	resp, err := parseExternalResponse(ecfg, []byte(raw))
	require.NoError(t, err)
	assert.Equal(t, "hello world", resp.Content)

	// Content field
	raw2 := `{"content": "from content field"}`
	resp, err = parseExternalResponse(ecfg, []byte(raw2))
	require.NoError(t, err)
	assert.Equal(t, "from content field", resp.Content)

	// Plain text fallback
	plain := `This is just plain text`
	resp, err = parseExternalResponse(ecfg, []byte(plain))
	require.NoError(t, err)
	assert.Equal(t, "This is just plain text", resp.Content)
}

func TestParseExternalStreamLine(t *testing.T) {
	ecfg := ExternalCLIConfig{Name: "test", StreamTextField: "content"}

	// Text delta
	line := `{"content":"hello "}`
	evt, err := parseExternalStreamLine(ecfg, []byte(line))
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "delta", evt.Type)
	assert.Equal(t, "hello ", evt.Content)

	// Done event
	done := `{"type":"done","result":"full text"}`
	evt, err = parseExternalStreamLine(ecfg, []byte(done))
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "done", evt.Type)

	// Plain text
	plain := `Just raw text`
	evt, err = parseExternalStreamLine(ecfg, []byte(plain))
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "delta", evt.Type)
	assert.Equal(t, "Just raw text", evt.Content)
}

func TestBuildClaudeArgs(t *testing.T) {
	args := buildClaudeArgs(CompletionRequest{
		Model:  "sonnet",
		System: "You are helpful.",
		Stream: false,
	})

	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "--dangerously-skip-permissions")
	assert.Contains(t, args, "json")
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "sonnet")
	assert.Contains(t, args, "--system-prompt")
	assert.Contains(t, args, "You are helpful.")
}

func TestBuildClaudeArgsStream(t *testing.T) {
	args := buildClaudeArgs(CompletionRequest{
		Model:  "haiku",
		Stream: true,
	})

	assert.Contains(t, args, "stream-json")
	assert.Contains(t, args, "--verbose")
}

func TestBuildExternalArgs(t *testing.T) {
	ecfg := ExternalCLIConfig{
		BaseArgs:   []string{"run"},
		ModelFlag:  "--model",
		SystemFlag: "--system",
		JSONFlag:   []string{"--format", "json"},
		StreamFlag: []string{"--stream"},
	}

	args := buildExternalArgs(ecfg, CompletionRequest{
		Model:  "llama3",
		System: "Be concise.",
	})

	assert.Equal(t, "run", args[0])
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "llama3")
	assert.Contains(t, args, "--system")
	assert.Contains(t, args, "Be concise.")
	assert.Contains(t, args, "--format")
}

func TestProviderError(t *testing.T) {
	err := &ProviderError{Provider: "claude", Message: "rate limited", Code: 429}
	assert.Equal(t, "claude: 429 rate limited", err.Error())

	err2 := &ProviderError{Provider: "gemini", Message: "unknown error"}
	assert.Equal(t, "gemini: unknown error", err2.Error())
}

func TestCLIExists(t *testing.T) {
	// "ls" should exist on any unix system
	assert.True(t, CLIExists("ls"))
	assert.False(t, CLIExists("nonexistent-binary-xyz-12345"))
}

func TestParseCLIJSONField(t *testing.T) {
	data := []byte(`{"result":"hello","model":"test"}`)
	assert.Equal(t, "hello", parseCLIJSONField(data, "result"))
	assert.Equal(t, "test", parseCLIJSONField(data, "model"))
	assert.Equal(t, "", parseCLIJSONField(data, "nonexistent"))
}

// --- Completeness: verify JSON round-trip of core types ---

func TestCompletionRequestJSON(t *testing.T) {
	temp := 0.7
	req := CompletionRequest{
		Model:       "sonnet",
		System:      "You are helpful.",
		Messages:    []Message{{Role: RoleUser, Content: "hi"}},
		MaxTokens:   1024,
		Temperature: &temp,
	}
	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded CompletionRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, req.Model, decoded.Model)
	assert.Equal(t, req.Messages[0].Content, decoded.Messages[0].Content)
}

func TestStreamEventTypes(t *testing.T) {
	delta := StreamEvent{Type: "delta", Content: "hello"}
	assert.Equal(t, "delta", delta.Type)

	errEvt := StreamEvent{Type: "error", Error: "something broke"}
	assert.Equal(t, "error", errEvt.Type)

	done := StreamEvent{
		Type:     "done",
		Response: &CompletionResponse{Content: "full text"},
	}
	assert.Equal(t, "done", done.Type)
	assert.Equal(t, "full text", done.Response.Content)
}

func TestUsageJSON(t *testing.T) {
	u := Usage{InputTokens: 100, OutputTokens: 50, CacheRead: 200, CacheWrite: 10}
	data, err := json.Marshal(u)
	require.NoError(t, err)

	var decoded Usage
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, u, decoded)
}

func TestMockClientDefaultComplete(t *testing.T) {
	mock := &MockClient{ProviderName: "default"}
	resp, err := mock.Complete(context.Background(), CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "mock response", resp.Content)
}

// --- ExternalCLIConfig defaults ---

func TestExternalCLIConfigDefaults(t *testing.T) {
	ecfg := ExternalCLIConfig{
		Command: "test-cli",
		Name:    "test",
	}
	client := NewExternalCLIClient(ecfg, silentLog())
	assert.Equal(t, "test", client.Name())
}

func TestProviderErrorFormat(t *testing.T) {
	tests := []struct {
		err  ProviderError
		want string
	}{
		{ProviderError{Provider: "a", Message: "fail", Code: 500}, "a: 500 fail"},
		{ProviderError{Provider: "b", Message: "oops"}, "b: oops"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.err.Error(), fmt.Sprintf("%+v", tt.err))
	}
}

package agent

import (
	"context"
	"testing"
	"time"

	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/llm"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunStream_Basic(t *testing.T) {
	log := logging.New(nil, "debug")
	registry := llm.NewRegistry(log)

	// Register a mock streaming client
	mockClient := &mockStreamClient{
		events: []llm.StreamEvent{
			{Type: "delta", Content: "Hello"},
			{Type: "delta", Content: " world"},
			{Type: "done", Response: &llm.CompletionResponse{
				Content: "Hello world",
				Model:   "test-model",
				Usage:   llm.Usage{InputTokens: 10, OutputTokens: 5},
			}},
		},
	}
	registry.Register("test-model", mockClient)

	sessions := NewMemorySessionStore()
	tools := NewToolRegistry()

	runner := NewRunner(
		RunnerConfig{
			AgentID:   "test-agent",
			AgentName: "Test Agent",
			Model:     "test-model",
			MaxTokens: 1000,
		},
		registry,
		sessions,
		tools,
		log,
	)

	msg := domain.InboundMessage{
		ChannelID: "test",
		ChatID:    "chat1",
		From:      "user1",
		FromName:  "Test User",
		Body:      "Hello!",
		ChatType:  domain.ChatTypeDM,
		Timestamp: time.Now(),
	}

	var receivedEvents []llm.StreamEvent
	callback := func(evt llm.StreamEvent) {
		receivedEvents = append(receivedEvents, evt)
	}

	result, err := runner.RunStream(context.Background(), msg, callback)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Hello world", result.Response)
	assert.Equal(t, "test-model", result.Model)
	// Deltas are buffered and sent as a single cleaned chunk on the final iteration.
	require.GreaterOrEqual(t, len(receivedEvents), 1)
	var combined string
	for _, evt := range receivedEvents {
		if evt.Type == "delta" {
			combined += evt.Content
		}
	}
	assert.Equal(t, "Hello world", combined)
}

func TestRunStream_WithToolCalls(t *testing.T) {
	log := logging.New(nil, "debug")
	registry := llm.NewRegistry(log)

	// Mock client that simulates tool calls
	mockClient := &mockStreamClient{
		responses: [][]llm.StreamEvent{
			// First iteration: request tool call
			{
				{Type: "delta", Content: "Let me check that.\n\n"},
				{Type: "delta", Content: "```tool_call\n{\"tool\": \"test_tool\", \"input\": {}}\n```"},
				{Type: "done", Response: &llm.CompletionResponse{
					Content: "Let me check that.\n\n```tool_call\n{\"tool\": \"test_tool\", \"input\": {}}\n```",
					Model:   "test-model",
					Usage:   llm.Usage{InputTokens: 10, OutputTokens: 20},
				}},
			},
			// Second iteration: final response
			{
				{Type: "delta", Content: "The result is 42."},
				{Type: "done", Response: &llm.CompletionResponse{
					Content: "The result is 42.",
					Model:   "test-model",
					Usage:   llm.Usage{InputTokens: 30, OutputTokens: 10},
				}},
			},
		},
		responseIndex: 0,
	}
	registry.Register("test-model", mockClient)

	sessions := NewMemorySessionStore()
	tools := NewToolRegistry()

	// Register a test tool
	testTool := &mockTool{
		name:        "test_tool",
		description: "A test tool",
		schema:      `{"type": "object"}`,
		handler: func(ctx context.Context, input string) (string, error) {
			return "Tool result: 42", nil
		},
	}
	tools.Register(testTool)

	runner := NewRunner(
		RunnerConfig{
			AgentID:   "test-agent",
			AgentName: "Test Agent",
			Model:     "test-model",
			MaxTokens: 1000,
		},
		registry,
		sessions,
		tools,
		log,
	)

	msg := domain.InboundMessage{
		ChannelID: "test",
		ChatID:    "chat1",
		From:      "user1",
		FromName:  "Test User",
		Body:      "What is the answer?",
		ChatType:  domain.ChatTypeDM,
		Timestamp: time.Now(),
	}

	var toolStartSeen bool
	var toolResultSeen bool
	callback := func(evt llm.StreamEvent) {
		if evt.Type == "tool_start" {
			toolStartSeen = true
		}
		if evt.Type == "tool_result" {
			toolResultSeen = true
		}
	}

	result, err := runner.RunStream(context.Background(), msg, callback)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "The result is 42.", result.Response)
	assert.True(t, toolStartSeen, "Should receive tool_start event")
	assert.True(t, toolResultSeen, "Should receive tool_result event")
}

// mockStreamClient is a test helper that implements llm.Client with streaming
type mockStreamClient struct {
	events        []llm.StreamEvent
	responses     [][]llm.StreamEvent
	responseIndex int
}

func (m *mockStreamClient) Name() string {
	return "mock-stream"
}

func (m *mockStreamClient) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	// Not used in stream tests
	return &llm.CompletionResponse{
		Content: "mock response",
		Model:   "test-model",
	}, nil
}

func (m *mockStreamClient) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent, 10)

	go func() {
		defer close(ch)

		var eventsToSend []llm.StreamEvent
		if m.responses != nil && m.responseIndex < len(m.responses) {
			eventsToSend = m.responses[m.responseIndex]
			m.responseIndex++
		} else {
			eventsToSend = m.events
		}

		for _, evt := range eventsToSend {
			select {
			case ch <- evt:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// mockTool is a test helper that implements the Tool interface
type mockTool struct {
	name        string
	description string
	schema      string
	handler     func(ctx context.Context, input string) (string, error)
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) InputSchema() string {
	return m.schema
}

func (m *mockTool) Execute(ctx context.Context, input string) (string, error) {
	return m.handler(ctx, input)
}

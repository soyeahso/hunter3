package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/llm"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func silentLog() *logging.Logger {
	return logging.New(nil, "silent")
}

func testRegistry(mock llm.Client) *llm.Registry {
	reg := llm.NewRegistry(silentLog())
	reg.Register("mock", mock)
	reg.SetFallback("mock")
	return reg
}

func testMessage() domain.InboundMessage {
	return domain.InboundMessage{
		ID:        "msg-1",
		ChannelID: "irc",
		From:      "testuser",
		FromName:  "Test User",
		ChatID:    "#general",
		ChatType:  domain.ChatTypeDM,
		Body:      "Hello, how are you?",
		Timestamp: time.Now(),
	}
}

// --- Runner tests ---

func TestRunnerComplete(t *testing.T) {
	mock := &llm.MockClient{
		ProviderName: "mock",
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			// Verify the request has context
			assert.NotEmpty(t, req.System)
			assert.NotEmpty(t, req.Messages)
			last := req.Messages[len(req.Messages)-1]
			assert.Equal(t, "user", last.Role)
			assert.Equal(t, "Hello, how are you?", last.Content)

			return &llm.CompletionResponse{
				Content: "I'm doing well, thank you!",
				Model:   "mock-model",
				Usage:   llm.Usage{InputTokens: 20, OutputTokens: 10},
				CostUSD: 0.001,
			}, nil
		},
	}

	runner := NewRunner(
		RunnerConfig{
			AgentID:   "test-agent",
			AgentName: "Test Bot",
			Model:     "mock",
		},
		testRegistry(mock),
		NewMemorySessionStore(),
		NewToolRegistry(),
		silentLog(),
	)

	result, err := runner.Run(context.Background(), testMessage())
	require.NoError(t, err)
	assert.Equal(t, "I'm doing well, thank you!", result.Response)
	assert.NotEmpty(t, result.SessionID)
	assert.Equal(t, 20, result.Usage.InputTokens)
	assert.Equal(t, 0.001, result.CostUSD)
}

func TestRunnerStream(t *testing.T) {
	mock := &llm.MockClient{
		ProviderName: "mock",
		StreamFunc: func(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamEvent, error) {
			ch := make(chan llm.StreamEvent, 3)
			ch <- llm.StreamEvent{Type: "delta", Content: "Hello "}
			ch <- llm.StreamEvent{Type: "delta", Content: "world!"}
			ch <- llm.StreamEvent{
				Type: "done",
				Response: &llm.CompletionResponse{
					Content: "Hello world!",
					Usage:   llm.Usage{InputTokens: 10, OutputTokens: 5},
				},
			}
			close(ch)
			return ch, nil
		},
	}

	runner := NewRunner(
		RunnerConfig{
			AgentID:   "test-agent",
			AgentName: "Test Bot",
			Model:     "mock",
		},
		testRegistry(mock),
		NewMemorySessionStore(),
		NewToolRegistry(),
		silentLog(),
	)

	var deltas []string
	result, err := runner.RunStream(context.Background(), testMessage(), func(evt llm.StreamEvent) {
		if evt.Type == "delta" {
			deltas = append(deltas, evt.Content)
		}
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello world!", result.Response)
	// Deltas are forwarded in real-time as they arrive from the LLM.
	assert.Equal(t, []string{"Hello ", "world!"}, deltas)
}

func TestRunnerSessionPersistence(t *testing.T) {
	callCount := 0
	mock := &llm.MockClient{
		ProviderName: "mock",
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			callCount++
			// On second call, should see history
			if callCount == 2 {
				assert.GreaterOrEqual(t, len(req.Messages), 3, "second call should include history")
			}
			return &llm.CompletionResponse{
				Content: fmt.Sprintf("Response %d", callCount),
			}, nil
		},
	}

	sessions := NewMemorySessionStore()
	runner := NewRunner(
		RunnerConfig{
			AgentID:   "test-agent",
			AgentName: "Test Bot",
			Model:     "mock",
		},
		testRegistry(mock),
		sessions,
		NewToolRegistry(),
		silentLog(),
	)

	msg := testMessage()

	// First call
	r1, err := runner.Run(context.Background(), msg)
	require.NoError(t, err)
	assert.Equal(t, "Response 1", r1.Response)

	// Second call same sender/channel — should reuse session
	msg.Body = "Follow up question"
	r2, err := runner.Run(context.Background(), msg)
	require.NoError(t, err)
	assert.Equal(t, "Response 2", r2.Response)
	assert.Equal(t, r1.SessionID, r2.SessionID, "should reuse session")
	assert.Equal(t, 2, callCount)
}

func TestRunnerLLMError(t *testing.T) {
	mock := &llm.MockClient{
		ProviderName: "mock",
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			return nil, &llm.ProviderError{Provider: "mock", Message: "service down", Code: 500}
		},
	}

	runner := NewRunner(
		RunnerConfig{AgentID: "test", AgentName: "Test", Model: "mock"},
		testRegistry(mock),
		NewMemorySessionStore(),
		NewToolRegistry(),
		silentLog(),
	)

	_, err := runner.Run(context.Background(), testMessage())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLM completion")
}

// --- SessionStore tests ---

func TestMemorySessionStoreGetOrCreate(t *testing.T) {
	store := NewMemorySessionStore()
	key := domain.SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "user1"}

	s1 := store.GetOrCreate(key, "agent-1")
	assert.NotEmpty(t, s1.ID)
	assert.Equal(t, "agent-1", s1.AgentID)

	// Same key returns same session
	s2 := store.GetOrCreate(key, "agent-1")
	assert.Equal(t, s1.ID, s2.ID)

	// Different key creates new session
	key2 := domain.SessionKey{ChannelID: "irc", ChatID: "#other", SenderID: "user1"}
	s3 := store.GetOrCreate(key2, "agent-1")
	assert.NotEqual(t, s1.ID, s3.ID)
}

func TestMemorySessionStoreAppendAndHistory(t *testing.T) {
	store := NewMemorySessionStore()
	key := domain.SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "user1"}
	session := store.GetOrCreate(key, "agent-1")

	store.Append(session.ID, domain.Message{Role: "user", Content: "hi", Timestamp: time.Now()})
	store.Append(session.ID, domain.Message{Role: "assistant", Content: "hello!", Timestamp: time.Now()})

	history := store.History(session.ID)
	assert.Len(t, history, 2)
	assert.Equal(t, "user", history[0].Role)
	assert.Equal(t, "hi", history[0].Content)
	assert.Equal(t, "assistant", history[1].Role)
}

func TestMemorySessionStoreList(t *testing.T) {
	store := NewMemorySessionStore()

	store.GetOrCreate(domain.SessionKey{ChannelID: "a", ChatID: "1"}, "agent")
	store.GetOrCreate(domain.SessionKey{ChannelID: "b", ChatID: "2"}, "agent")

	ids := store.List()
	assert.Len(t, ids, 2)
}

func TestMemorySessionStoreGet(t *testing.T) {
	store := NewMemorySessionStore()
	key := domain.SessionKey{ChannelID: "irc", ChatID: "#test"}
	session := store.GetOrCreate(key, "agent-1")

	got := store.Get(session.ID)
	assert.NotNil(t, got)
	assert.Equal(t, session.ID, got.ID)

	assert.Nil(t, store.Get("nonexistent"))
}

// --- SystemPrompt tests ---

func TestBuildSystemPrompt(t *testing.T) {
	prompt := BuildSystemPrompt(PromptConfig{
		AgentName:   "TestBot",
		AgentID:     "test-agent",
		ChannelID:   "irc",
		ChatType:    "dm",
		UserName:    "Alice",
		ExtraPrompt: "Always respond in haiku.",
	})

	assert.Contains(t, prompt, "TestBot")
	assert.Contains(t, prompt, "irc")
	assert.Contains(t, prompt, "Alice")
	assert.Contains(t, prompt, "haiku")
	assert.Contains(t, prompt, "Current date:")
}

func TestBuildSystemPromptMinimal(t *testing.T) {
	prompt := BuildSystemPrompt(PromptConfig{
		AgentName: "Bot",
	})
	assert.Contains(t, prompt, "Bot")
	assert.Contains(t, prompt, "Guidelines:")
}

// --- ToolRegistry tests ---

type echoTool struct{}

func (e *echoTool) Name() string        { return "echo" }
func (e *echoTool) Description() string  { return "Echoes input" }
func (e *echoTool) InputSchema() string  { return `{"type":"object","properties":{"text":{"type":"string"}}}` }
func (e *echoTool) Execute(ctx context.Context, input string) (string, error) {
	return input, nil
}

func TestToolRegistry(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&echoTool{})

	tool, ok := reg.Get("echo")
	assert.True(t, ok)
	assert.Equal(t, "echo", tool.Name())

	_, ok = reg.Get("nonexistent")
	assert.False(t, ok)

	defs := reg.Definitions()
	assert.Len(t, defs, 1)
	assert.Equal(t, "echo", defs[0].Name)
}

func TestToolExecute(t *testing.T) {
	tool := &echoTool{}
	out, err := tool.Execute(context.Background(), `{"text":"hello"}`)
	require.NoError(t, err)
	assert.Equal(t, `{"text":"hello"}`, out)
}

// --- Failover tests ---

func TestFailoverSuccess(t *testing.T) {
	mock := &llm.MockClient{
		ProviderName: "mock",
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			return &llm.CompletionResponse{Content: "ok"}, nil
		},
	}

	reg := testRegistry(mock)
	fc := NewFailoverClient(reg, "mock", nil, silentLog())

	resp, err := fc.Complete(context.Background(), llm.CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
}

func TestFailoverTriesFallback(t *testing.T) {
	callOrder := []string{}

	primary := &llm.MockClient{
		ProviderName: "primary",
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			callOrder = append(callOrder, "primary")
			return nil, &llm.ProviderError{Provider: "primary", Message: "overloaded", Code: 529}
		},
	}

	fallback := &llm.MockClient{
		ProviderName: "fallback",
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			callOrder = append(callOrder, "fallback")
			return &llm.CompletionResponse{Content: "fallback response"}, nil
		},
	}

	reg := llm.NewRegistry(silentLog())
	reg.Register("primary", primary)
	reg.Register("fallback", fallback)

	fc := NewFailoverClient(reg, "primary", []string{"fallback"}, silentLog())

	resp, err := fc.Complete(context.Background(), llm.CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "fallback response", resp.Content)
	assert.Equal(t, []string{"primary", "fallback"}, callOrder)
}

func TestFailoverNonRetryableStops(t *testing.T) {
	callCount := 0

	primary := &llm.MockClient{
		ProviderName: "primary",
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			callCount++
			return nil, fmt.Errorf("non-retryable error")
		},
	}

	fallback := &llm.MockClient{
		ProviderName: "fallback",
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			callCount++
			return &llm.CompletionResponse{Content: "should not reach"}, nil
		},
	}

	reg := llm.NewRegistry(silentLog())
	reg.Register("primary", primary)
	reg.Register("fallback", fallback)

	fc := NewFailoverClient(reg, "primary", []string{"fallback"}, silentLog())

	_, err := fc.Complete(context.Background(), llm.CompletionRequest{})
	assert.Error(t, err)
	assert.Equal(t, 1, callCount, "should not try fallback on non-retryable error")
}

func TestIsRetryable(t *testing.T) {
	assert.True(t, isRetryable(&llm.ProviderError{Code: 429}))
	assert.True(t, isRetryable(&llm.ProviderError{Code: 529}))
	assert.True(t, isRetryable(&llm.ProviderError{Code: 503}))
	assert.True(t, isRetryable(fmt.Errorf("server overloaded")))
	assert.True(t, isRetryable(fmt.Errorf("rate limit exceeded")))
	assert.False(t, isRetryable(fmt.Errorf("invalid input")))
	assert.False(t, isRetryable(nil))
}

// --- stripToolCalls tests ---

func TestStripNoChanges(t *testing.T) {
	t.Run("plain text", func(t *testing.T) {
		input := "Just a normal response with no tool calls."
		assert.Equal(t, input, stripToolCalls(input, nil))
	})
	t.Run("markdown preserved", func(t *testing.T) {
		input := "The weather is **sunny** and `warm`."
		assert.Equal(t, input, stripToolCalls(input, nil))
	})
	t.Run("code block with language hint stripped", func(t *testing.T) {
		input := "Example:\n\n```json\n{\"key\": \"value\"}\n```\n\nDone."
		assert.Equal(t, "Example:\n\nDone.", stripToolCalls(input, nil))
	})
}

func TestStripToolCallCodeBlock(t *testing.T) {
	input := "Here is my answer.\n\n```tool_call\n{\"tool\": \"echo\", \"input\": {\"text\": \"hi\"}}\n```\n\nDone."
	assert.Equal(t, "Here is my answer.\n\nDone.", stripToolCalls(input, nil))
}

func TestStripXMLFunctionCalls(t *testing.T) {
	t.Run("single block", func(t *testing.T) {
		input := "I'll check the weather.\n\n<function_calls>\n<invoke name=\"computer\">\n" +
			"<parameter name=\"action\">bash</parameter>\n</invoke>\n</function_calls>\n\nThe weather is sunny."
		assert.Equal(t, "I'll check the weather.\n\nThe weather is sunny.", stripToolCalls(input, nil))
	})
	t.Run("multiple blocks", func(t *testing.T) {
		input := "Checking.\n\n<function_calls>\n<invoke name=\"a\"/>\n</function_calls>\n\n" +
			"Middle.\n\n<function_calls>\n<invoke name=\"b\"/>\n</function_calls>\n\nFinal."
		assert.Equal(t, "Checking.\n\nMiddle.\n\nFinal.", stripToolCalls(input, nil))
	})
	t.Run("mixed with tool_call", func(t *testing.T) {
		input := "Intro.\n\n```tool_call\n{\"tool\": \"echo\"}\n```\n\n<function_calls>\n<invoke name=\"x\"/>\n</function_calls>\n\nEnd."
		assert.Equal(t, "Intro.\n\nEnd.", stripToolCalls(input, nil))
	})
}

func TestStripParameterTags(t *testing.T) {
	t.Run("standalone block-level", func(t *testing.T) {
		input := "Weather for 02904.\n\n" +
			"<parameter name=\"coordinate\">[750, 550]</parameter>\n\n" +
			"<parameter name=\"screenshot\">true</parameter>\n\n" +
			"Clear, 30F"
		got := stripToolCalls(input, nil)
		assert.NotContains(t, got, "<parameter")
		assert.Contains(t, got, "Weather for 02904.")
		assert.Contains(t, got, "Clear, 30F")
	})
	t.Run("inline", func(t *testing.T) {
		input := `Hello <parameter name="action">click</parameter> world.`
		assert.Equal(t, "Hello world.", stripToolCalls(input, nil))
	})
}

func TestStripOrphanedXMLTags(t *testing.T) {
	input := "Result: <screenshot>true</screenshot> done."
	assert.Equal(t, "Result: true done.", stripToolCalls(input, nil))
}

func TestStripCodeBlock(t *testing.T) {
	t.Run("bash block-level", func(t *testing.T) {
		input := "Let me check.\n\n```bash\ncurl -s \"wttr.in/?format=3\"\n```\n\nIt's sunny."
		assert.Equal(t, "Let me check.\n\nIt's sunny.", stripToolCalls(input, nil))
	})
	t.Run("adjacent to text preserves paragraph break", func(t *testing.T) {
		input := "Let me check.```bash\ncurl http://example.com\n```Here are the results."
		assert.Equal(t, "Let me check.\n\nHere are the results.", stripToolCalls(input, nil))
	})
	t.Run("no newline after language hint", func(t *testing.T) {
		input := "Let me check.```bashcurl -s \"wttr.in/?format=3\"```Here are the results."
		got := stripToolCalls(input, nil)
		assert.Equal(t, "Let me check.\n\nHere are the results.", got)
		assert.NotContains(t, got, "curl")
	})
	t.Run("plain block no language hint", func(t *testing.T) {
		input := "Wind: Southwest at 8 mph```\ncurl -s \"wttr.in/\"\n```It's a warm evening!"
		got := stripToolCalls(input, nil)
		assert.Equal(t, "Wind: Southwest at 8 mph\n\nIt's a warm evening!", got)
		assert.NotContains(t, got, "curl")
	})
	t.Run("any language hint stripped", func(t *testing.T) {
		for _, lang := range []string{"bash", "sh", "json", "python", "text", "console", ""} {
			t.Run("```"+lang, func(t *testing.T) {
				input := "Before.\n\n```" + lang + "\nsome content\n```\n\nAfter."
				assert.Equal(t, "Before.\n\nAfter.", stripToolCalls(input, nil))
			})
		}
	})
	t.Run("weather data followed by bold section", func(t *testing.T) {
		input := "Lake Placid, FL: +73°F 69% 9mph" +
			"```text\ncurl -s \"wttr.in/Lake+Placid,FL\"\n```" +
			"**Today's forecast:**\n\nBeautiful sunny day!"
		got := stripToolCalls(input, nil)
		assert.Equal(t, "Lake Placid, FL: +73°F 69% 9mph\n\n**Today's forecast:**\n\nBeautiful sunny day!", got)
		assert.NotContains(t, got, "curl")
	})
	t.Run("multiline command", func(t *testing.T) {
		input := "Checking.\n\n```bash\ncurl -s \"wttr.in/?format=3\" \\\n  | head -1\n```\n\nDone."
		got := stripToolCalls(input, nil)
		assert.Equal(t, "Checking.\n\nDone.", got)
		assert.NotContains(t, got, "curl")
	})
}

func TestStripSentenceRunon(t *testing.T) {
	t.Run("weather list into sentence", func(t *testing.T) {
		input := "- **Wind:** 4 mph from the eastPretty pleasant weather!"
		got := stripToolCalls(input, nil)
		assert.Equal(t, "- **Wind:** 4 mph from the east\n\nPretty pleasant weather!", got)
	})
	t.Run("unit into sentence", func(t *testing.T) {
		input := "Humidity: 69% 9mphIt's a warm evening with clear skies!"
		got := stripToolCalls(input, nil)
		assert.Equal(t, "Humidity: 69% 9mph\n\nIt's a warm evening with clear skies!", got)
	})
	t.Run("does not affect normal text", func(t *testing.T) {
		input := "The weather in Lake Placid is sunny."
		assert.Equal(t, input, stripToolCalls(input, nil))
	})
	t.Run("does not affect markdown bold", func(t *testing.T) {
		input := "**Current conditions:** Clear, 73°F"
		assert.Equal(t, input, stripToolCalls(input, nil))
	})
}

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/llm"
	"github.com/soyeahso/hunter3/internal/logging"
)

// maxToolIterations limits how many tool call rounds the agent can perform.
const maxToolIterations = 5

// RunnerConfig configures the agent runner.
type RunnerConfig struct {
	AgentID     string
	AgentName   string
	Model       string
	Fallbacks   []string
	MaxTokens   int
	Temperature *float64
	ExtraPrompt string
}

// RunResult is the outcome of processing a message.
type RunResult struct {
	Response  string        `json:"response"`
	SessionID string        `json:"sessionId"`
	Model     string        `json:"model,omitempty"`
	Usage     llm.Usage     `json:"usage"`
	CostUSD   float64       `json:"costUsd,omitempty"`
	Duration  time.Duration `json:"duration"`
}

// StreamCallback is called for each streaming event during RunStream execution.
// Event types:
//   - "delta": Incremental text output (Content field contains the text)
//   - "tool_start": Tool execution is beginning (Content describes the tool)
//   - "tool_result": Tool completed successfully (Content describes completion)
//   - "tool_error": Tool execution failed (Content describes the error)
//   - "done": Final response is ready (Response field contains full result)
//   - "error": Stream error occurred (Error field contains error message)
type StreamCallback func(event llm.StreamEvent)

// Runner is the agent orchestration loop.
// It takes inbound messages, builds context, calls the LLM, and returns responses.
type Runner struct {
	cfg      RunnerConfig
	client   *FailoverClient
	sessions SessionStore
	tools    *ToolRegistry
	log      *logging.Logger
}

// NewRunner creates an agent runner.
func NewRunner(
	cfg RunnerConfig,
	registry *llm.Registry,
	sessions SessionStore,
	tools *ToolRegistry,
	log *logging.Logger,
) *Runner {
	fc := NewFailoverClient(registry, cfg.Model, cfg.Fallbacks, log)
	return &Runner{
		cfg:      cfg,
		client:   fc,
		sessions: sessions,
		tools:    tools,
		log:      log.Sub("agent." + cfg.AgentID),
	}
}

// Run processes an inbound message and returns the agent's response.
func (r *Runner) Run(ctx context.Context, msg domain.InboundMessage) (*RunResult, error) {
	start := time.Now()

	// Get or create session
	key := domain.SessionKey{
		ChannelID: msg.ChannelID,
		ChatID:    msg.ChatID,
		SenderID:  msg.From,
	}
	session := r.sessions.GetOrCreate(key, r.cfg.AgentID)

	r.log.Info().
		Str("sessionId", session.ID).
		Str("from", msg.From).
		Str("channel", msg.ChannelID).
		Int("historyLen", len(session.Messages)).
		Msg("processing message")

	// Record user message
	r.sessions.Append(session.ID, domain.Message{
		Role:      "user",
		Content:   msg.Body,
		Timestamp: msg.Timestamp,
	})

	// Build system prompt
	system := BuildSystemPrompt(PromptConfig{
		AgentName:   r.cfg.AgentName,
		AgentID:     r.cfg.AgentID,
		Model:       r.cfg.Model,
		Tools:       r.tools.Definitions(),
		ChannelID:   msg.ChannelID,
		ChatType:    string(msg.ChatType),
		UserName:    msg.FromName,
		ExtraPrompt: r.cfg.ExtraPrompt,
	})

	// Tool execution loop
	var finalResp *llm.CompletionResponse
	for i := 0; i < maxToolIterations; i++ {
		history := r.sessions.History(session.ID)

		req := llm.CompletionRequest{
			Model:       r.cfg.Model,
			System:      system,
			Messages:    history,
			MaxTokens:   r.cfg.MaxTokens,
			Temperature: r.cfg.Temperature,
		}

		resp, err := r.client.Complete(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("LLM completion: %w", err)
		}

		finalResp = resp

		// Check for tool calls in the response
		calls := parseToolCalls(resp.Content)
		if len(calls) == 0 {
			// No tool calls — final response
			break
		}

		r.log.Info().Int("toolCalls", len(calls)).Msg("executing tool calls")

		// Record the assistant's response (with tool calls)
		r.sessions.Append(session.ID, domain.Message{
			Role:      "assistant",
			Content:   resp.Content,
			Timestamp: time.Now(),
		})

		// Execute tools and build results
		results := r.executeToolCalls(ctx, calls)

		// Append tool results as a follow-up message
		r.sessions.Append(session.ID, domain.Message{
			Role:      "user",
			Content:   formatToolResults(results),
			Timestamp: time.Now(),
		})
		// Loop to let the LLM process tool results
	}

	if finalResp == nil {
		return nil, fmt.Errorf("no response from LLM")
	}

	// Strip tool_call blocks from final response for clean output
	cleanResponse := stripToolCalls(finalResp.Content, r.log)

	// Record assistant response
	r.sessions.Append(session.ID, domain.Message{
		Role:      "assistant",
		Content:   cleanResponse,
		Timestamp: time.Now(),
	})

	r.log.Info().
		Str("sessionId", session.ID).
		Str("model", finalResp.Model).
		Int("inputTokens", finalResp.Usage.InputTokens).
		Int("outputTokens", finalResp.Usage.OutputTokens).
		Dur("duration", time.Since(start)).
		Msg("response generated")

	return &RunResult{
		Response:  cleanResponse,
		SessionID: session.ID,
		Model:     finalResp.Model,
		Usage:     finalResp.Usage,
		CostUSD:   finalResp.CostUSD,
		Duration:  time.Since(start),
	}, nil
}

// RunStream processes a message with streaming output.
// This version supports tool execution loops with streaming.
func (r *Runner) RunStream(ctx context.Context, msg domain.InboundMessage, cb StreamCallback) (*RunResult, error) {
	start := time.Now()

	// Get or create session
	key := domain.SessionKey{
		ChannelID: msg.ChannelID,
		ChatID:    msg.ChatID,
		SenderID:  msg.From,
	}
	session := r.sessions.GetOrCreate(key, r.cfg.AgentID)

	r.log.Info().
		Str("sessionId", session.ID).
		Str("from", msg.From).
		Str("channel", msg.ChannelID).
		Int("historyLen", len(session.Messages)).
		Msg("processing message with streaming")

	// Record user message
	r.sessions.Append(session.ID, domain.Message{
		Role:      "user",
		Content:   msg.Body,
		Timestamp: msg.Timestamp,
	})

	// Build system prompt
	system := BuildSystemPrompt(PromptConfig{
		AgentName:   r.cfg.AgentName,
		AgentID:     r.cfg.AgentID,
		Model:       r.cfg.Model,
		Tools:       r.tools.Definitions(),
		ChannelID:   msg.ChannelID,
		ChatType:    string(msg.ChatType),
		UserName:    msg.FromName,
		ExtraPrompt: r.cfg.ExtraPrompt,
	})

	// Tool execution loop with streaming
	var finalResp *llm.CompletionResponse
	var fullContent string

	for i := 0; i < maxToolIterations; i++ {
		history := r.sessions.History(session.ID)

		req := llm.CompletionRequest{
			Model:       r.cfg.Model,
			System:      system,
			Messages:    history,
			MaxTokens:   r.cfg.MaxTokens,
			Temperature: r.cfg.Temperature,
			Stream:      true,
		}

		ch, err := r.client.Stream(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("LLM stream: %w", err)
		}

		// Accumulate content from stream while forwarding deltas in real-time.
		fullContent = ""
		var streamResp *llm.CompletionResponse

		for evt := range ch {
			switch evt.Type {
			case "delta":
				fullContent += evt.Content
				if cb != nil {
					cb(evt)
				}
			case "done":
				if evt.Response != nil {
					streamResp = evt.Response
				}
			case "error":
				return nil, fmt.Errorf("stream error: %s", evt.Error)
			}
		}

		// Use accumulated content if response content is empty
		if streamResp != nil {
			if streamResp.Content == "" {
				streamResp.Content = fullContent
			}
			finalResp = streamResp
		} else {
			// Fallback: create response from accumulated content
			finalResp = &llm.CompletionResponse{
				Content: fullContent,
				Model:   r.cfg.Model,
			}
		}

		// Check for tool calls in the response
		calls := parseToolCalls(finalResp.Content)
		if len(calls) == 0 {
			break
		}

		r.log.Info().Int("toolCalls", len(calls)).Msg("executing tool calls")

		// Send a "tool_start" event to notify streaming consumers
		if cb != nil {
			cb(llm.StreamEvent{
				Type:    "tool_start",
				Content: fmt.Sprintf("Executing %d tool(s)...", len(calls)),
			})
		}

		// Record the assistant's response (with tool calls)
		r.sessions.Append(session.ID, domain.Message{
			Role:      "assistant",
			Content:   finalResp.Content,
			Timestamp: time.Now(),
		})

		// Execute tools and build results
		results := r.executeToolCalls(ctx, calls)

		// Send tool execution results through callback
		if cb != nil {
			for _, tr := range results {
				if tr.Err != nil {
					cb(llm.StreamEvent{
						Type:    "tool_error",
						Content: fmt.Sprintf("Tool %s failed: %v", tr.Tool, tr.Err),
					})
				} else {
					cb(llm.StreamEvent{
						Type:    "tool_result",
						Content: fmt.Sprintf("Tool %s completed", tr.Tool),
					})
				}
			}
		}

		// Append tool results as a follow-up message
		r.sessions.Append(session.ID, domain.Message{
			Role:      "user",
			Content:   formatToolResults(results),
			Timestamp: time.Now(),
		})
		// Loop to let the LLM process tool results
	}

	if finalResp == nil {
		return nil, fmt.Errorf("no response from LLM")
	}

	// Strip tool_call blocks from final response for clean output
	cleanResponse := stripToolCalls(finalResp.Content, r.log)

	// Record assistant response
	r.sessions.Append(session.ID, domain.Message{
		Role:      "assistant",
		Content:   cleanResponse,
		Timestamp: time.Now(),
	})

	r.log.Info().
		Str("sessionId", session.ID).
		Str("model", finalResp.Model).
		Int("inputTokens", finalResp.Usage.InputTokens).
		Int("outputTokens", finalResp.Usage.OutputTokens).
		Dur("duration", time.Since(start)).
		Msg("streaming response generated")

	return &RunResult{
		Response:  cleanResponse,
		SessionID: session.ID,
		Model:     finalResp.Model,
		Usage:     finalResp.Usage,
		CostUSD:   finalResp.CostUSD,
		Duration:  time.Since(start),
	}, nil
}

// toolCall is a parsed tool invocation from the LLM response.
type toolCall struct {
	Tool  string          `json:"tool"`
	Input json.RawMessage `json:"input"`
}

// toolResult holds the output from executing a tool.
type toolResult struct {
	Tool   string
	Output string
	Err    error
}

// toolCallRe matches ```tool_call\n{...}\n``` blocks in LLM output.
var toolCallRe = regexp.MustCompile("(?s)```tool_call\\s*\n(\\{.*?\\})\n\\s*```")

// xmlFuncCallRe matches <function_calls>...</function_calls> XML blocks in LLM output.
var xmlFuncCallRe = regexp.MustCompile(`(?s)<function_calls>.*?</function_calls>`)

// xmlBlockLevelRe matches self-contained XML blocks that LLMs emit for tool use
// (block-level, replaced with paragraph break).
var xmlBlockLevelRe = regexp.MustCompile(`(?s)(?:` +
	`<invoke\b[^>]*>.*?</invoke>` +
	`|<tool_call\b[^>]*>.*?</tool_call>` +
	`|<tool_use\b[^>]*>.*?</tool_use>` +
	`)`)

// xmlInlineTagRe matches parameter tags that can appear inline within text.
var xmlInlineTagRe = regexp.MustCompile(`(?s)<parameter\b[^>]*>.*?</parameter>`)

// xmlTagRe matches any remaining XML-like opening/closing/self-closing tags.
// Applied after block-level stripping to catch orphaned tags like
// <location>, <screenshot>, etc. that leak from tool-use output.
var xmlTagRe = regexp.MustCompile(`</?[a-zA-Z][a-zA-Z0-9_]*(?:\s[^>]*)?>`)

// codeBlockRe matches any fenced code block (with or without a language hint).
// In a chat bot context, all code blocks are execution artifacts that should
// not appear in user-facing output.
var codeBlockRe = regexp.MustCompile("(?s)```\\w*\\s*.*?```")

// codeFenceRe matches fenced code block opening/closing markers on their own line.
// Only the markers are stripped — content between fences is preserved.
var codeFenceRe = regexp.MustCompile(`(?m)^\s*` + "```" + `\w*\s*$`)

// whitespaceLineRe matches lines containing only horizontal whitespace.
var whitespaceLineRe = regexp.MustCompile(`(?m)^[ \t]+$`)

// blankLineCollapseRe collapses 3+ consecutive newlines to a single blank line.
var blankLineCollapseRe = regexp.MustCompile(`\n{3,}`)

// parseToolCalls extracts tool_call blocks from LLM response text.
func parseToolCalls(text string) []toolCall {
	matches := toolCallRe.FindAllStringSubmatch(text, -1)
	var calls []toolCall
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		var tc toolCall
		if err := json.Unmarshal([]byte(match[1]), &tc); err != nil {
			continue
		}
		if tc.Tool != "" {
			calls = append(calls, tc)
		}
	}
	return calls
}

// executeToolCalls runs each tool and returns results.
func (r *Runner) executeToolCalls(ctx context.Context, calls []toolCall) []toolResult {
	var results []toolResult
	for _, tc := range calls {
		tool, ok := r.tools.Get(tc.Tool)
		if !ok {
			results = append(results, toolResult{
				Tool: tc.Tool,
				Err:  fmt.Errorf("unknown tool: %s", tc.Tool),
			})
			continue
		}

		r.log.Debug().Str("tool", tc.Tool).Msg("executing tool")
		output, err := tool.Execute(ctx, string(tc.Input))
		results = append(results, toolResult{
			Tool:   tc.Tool,
			Output: output,
			Err:    err,
		})
	}
	return results
}

// formatToolResults renders tool execution results for the LLM.
func formatToolResults(results []toolResult) string {
	var b strings.Builder
	b.WriteString("Tool execution results:\n\n")
	for _, r := range results {
		fmt.Fprintf(&b, "### %s\n", r.Tool)
		if r.Err != nil {
			fmt.Fprintf(&b, "Error: %s\n", r.Err)
		} else {
			b.WriteString(r.Output)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// stripToolCalls removes tool_call code blocks and XML function_calls blocks
// from the response, leaving surrounding text. Stripped XML blocks are logged
// to the console so they remain visible for debugging.
func stripToolCalls(text string, log *logging.Logger) string {
	// Block-level elements are replaced with a paragraph break so
	// surrounding text stays visually separated (e.g. a list item
	// followed by a closing sentence). Inline tags use a space.

	// Strip ```tool_call``` code blocks (block-level)
	cleaned := toolCallRe.ReplaceAllString(text, "\n\n")

	// Strip all fenced code blocks (block-level)
	//cleaned = codeBlockRe.ReplaceAllLiteralString(cleaned, "\n\n")

	// Strip <function_calls>...</function_calls> XML blocks (block-level)
	xmlMatches := xmlFuncCallRe.FindAllString(cleaned, -1)
	if len(xmlMatches) > 0 && log != nil {
		for _, m := range xmlMatches {
			log.Info().Str("xml", m).Msg("stripped XML function_calls from LLM response")
		}
	}
	cleaned = xmlFuncCallRe.ReplaceAllString(cleaned, "\n\n")

	// Strip known block-level XML tags
	cleaned = xmlBlockLevelRe.ReplaceAllString(cleaned, "\n\n")

	// Strip inline parameter tags
	cleaned = xmlInlineTagRe.ReplaceAllString(cleaned, " ")

	// Strip any remaining orphaned XML tags (inline)
	//cleaned = xmlTagRe.ReplaceAllString(cleaned, " ")

	// Strip code fence markers (``` and ```language) but keep content between them.
	// Channels like IRC don't render markdown, so fence markers appear as glitches.
	cleaned = codeFenceRe.ReplaceAllString(cleaned, "")

	// Clean up whitespace artifacts left by replacements:
	// Lines that are now only spaces/tabs become empty lines.
	cleaned = whitespaceLineRe.ReplaceAllString(cleaned, "")
	// Collapse 3+ consecutive newlines into one blank line.
	cleaned = blankLineCollapseRe.ReplaceAllString(cleaned, "\n\n")

	return strings.TrimSpace(cleaned)
}

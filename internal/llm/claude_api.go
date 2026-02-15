package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ClaudeAPIClient is a direct HTTP client for Claude API.
type ClaudeAPIClient struct {
	apiKey string
	model  string
	client *http.Client
}

// NewClaudeAPIClient creates a new Claude API client.
func NewClaudeAPIClient(apiKey, model string) *ClaudeAPIClient {
	return &ClaudeAPIClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// Complete sends a non-streaming completion request to Claude API.
func (c *ClaudeAPIClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()

	// Build request body
	body := c.buildRequestBody(req, false)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result claudeAPIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return c.responseToCompletion(&result, time.Since(start)), nil
}

// Stream sends a streaming completion request to Claude API.
func (c *ClaudeAPIClient) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamEvent, error) {
	eventChan := make(chan StreamEvent)

	body := c.buildRequestBody(req, true)
	payload, err := json.Marshal(body)
	if err != nil {
		close(eventChan)
		return eventChan, fmt.Errorf("failed to marshal request: %w", err)
	}

	go c.streamRequest(ctx, eventChan, payload)
	return eventChan, nil
}

// Name returns the provider name.
func (c *ClaudeAPIClient) Name() string {
	return "claude"
}

// Helper methods

func (c *ClaudeAPIClient) buildRequestBody(req CompletionRequest, stream bool) map[string]interface{} {
	body := map[string]interface{}{
		"model":     c.model,
		"messages":  c.messagesToClaude(req.Messages),
		"max_tokens": req.MaxTokens,
		"stream":    stream,
	}

	if req.System != "" {
		body["system"] = req.System
	}

	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]interface{}, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"input_schema": parseJSONSchema(t.InputSchema),
			}
		}
		body["tools"] = tools
	}

	return body
}

func (c *ClaudeAPIClient) messagesToClaude(msgs []Message) []map[string]string {
	result := make([]map[string]string, len(msgs))
	for i, m := range msgs {
		result[i] = map[string]string{
			"role":    m.Role,
			"content": m.Content,
		}
	}
	return result
}

func (c *ClaudeAPIClient) streamRequest(ctx context.Context, eventChan chan StreamEvent, payload []byte) {
	defer close(eventChan)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", strings.NewReader(string(payload)))
	if err != nil {
		eventChan <- StreamEvent{Type: "error", Error: fmt.Sprintf("request creation failed: %v", err)}
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		eventChan <- StreamEvent{Type: "error", Error: fmt.Sprintf("request failed: %v", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		eventChan <- StreamEvent{Type: "error", Error: fmt.Sprintf("API error (%d): %s", resp.StatusCode, string(body))}
		return
	}

	scanner := newServerSentEventScanner(resp.Body)
	var fullContent strings.Builder
	var usage Usage

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		dataStr := strings.TrimPrefix(line, "data: ")
		if dataStr == "[DONE]" {
			break
		}

		var event claudeStreamEvent
		if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				fullContent.WriteString(event.Delta.Text)
				eventChan <- StreamEvent{
					Type:    "delta",
					Content: event.Delta.Text,
				}
			}
		case "message_delta":
			if event.Delta.StopReason != "" {
				// Message finished
			}
		case "message_start":
			if event.Message.Usage.InputTokens > 0 {
				usage.InputTokens = event.Message.Usage.InputTokens
			}
		case "message_stop":
			if event.Message != nil && event.Message.Usage.OutputTokens > 0 {
				usage.OutputTokens = event.Message.Usage.OutputTokens
			}
		}
	}

	// Send completion event
	eventChan <- StreamEvent{
		Type: "done",
		Response: &CompletionResponse{
			Content: fullContent.String(),
			Usage:   usage,
			Model:   c.model,
		},
	}
}

func (c *ClaudeAPIClient) responseToCompletion(resp *claudeAPIResponse, duration time.Duration) *CompletionResponse {
	var content strings.Builder
	var toolCalls []ToolCall

	for _, block := range resp.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		} else if block.Type == "tool_use" {
			toolCalls = append(toolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	return &CompletionResponse{
		Content:   content.String(),
		StopReason: resp.StopReason,
		ToolCalls: toolCalls,
		Usage: Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		},
		Model:    resp.Model,
		Duration: duration,
	}
}

// API Response structures

type claudeAPIResponse struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Role     string                 `json:"role"`
	Content  []claudeContentBlock   `json:"content"`
	Model    string                 `json:"model"`
	StopReason string                `json:"stop_reason"`
	Usage    claudeUsage            `json:"usage"`
}

type claudeContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input string `json:"input,omitempty"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type claudeStreamEvent struct {
	Type    string `json:"type"`
	Delta   claudeStreamDelta `json:"delta,omitempty"`
	Message *claudeAPIResponse `json:"message,omitempty"`
}

type claudeStreamDelta struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
}

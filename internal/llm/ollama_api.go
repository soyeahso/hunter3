package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaAPIClient is a direct HTTP client for Ollama API.
type OllamaAPIClient struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaAPIClient creates a new Ollama API client.
// baseURL should be like "http://localhost:11434"
func NewOllamaAPIClient(baseURL, model string) *OllamaAPIClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	// Remove trailing slash if present
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &OllamaAPIClient{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// Complete sends a non-streaming completion request to Ollama API.
func (o *OllamaAPIClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()

	prompt := o.buildPrompt(req)
	body := map[string]interface{}{
		"model":  o.model,
		"prompt": prompt,
		"stream": false,
	}

	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/generate", o.baseURL), strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
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

	var result ollamaAPIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &CompletionResponse{
		Content: result.Response,
		Model:   o.model,
		Duration: time.Since(start),
	}, nil
}

// Stream sends a streaming completion request to Ollama API.
func (o *OllamaAPIClient) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamEvent, error) {
	eventChan := make(chan StreamEvent)

	prompt := o.buildPrompt(req)
	body := map[string]interface{}{
		"model":  o.model,
		"prompt": prompt,
		"stream": true,
	}

	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}

	payload, err := json.Marshal(body)
	if err != nil {
		close(eventChan)
		return eventChan, fmt.Errorf("failed to marshal request: %w", err)
	}

	go o.streamRequest(ctx, eventChan, payload)
	return eventChan, nil
}

// Name returns the provider name.
func (o *OllamaAPIClient) Name() string {
	return "ollama"
}

// Helper methods

func (o *OllamaAPIClient) buildPrompt(req CompletionRequest) string {
	var prompt strings.Builder

	if req.System != "" {
		prompt.WriteString("System: ")
		prompt.WriteString(req.System)
		prompt.WriteString("\n\n")
	}

	for _, msg := range req.Messages {
		if msg.Role != "user" {
			prompt.WriteString(fmt.Sprintf("%s: ", msg.Role))
		}
		prompt.WriteString(msg.Content)
		prompt.WriteString("\n\n")
	}

	return prompt.String()
}

func (o *OllamaAPIClient) streamRequest(ctx context.Context, eventChan chan StreamEvent, payload []byte) {
	defer close(eventChan)

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/generate", o.baseURL), strings.NewReader(string(payload)))
	if err != nil {
		eventChan <- StreamEvent{Type: "error", Error: fmt.Sprintf("request creation failed: %v", err)}
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
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

	scanner := bufio.NewScanner(resp.Body)
	var fullContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event ollamaStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.Response != "" {
			fullContent.WriteString(event.Response)
			eventChan <- StreamEvent{
				Type:    "delta",
				Content: event.Response,
			}
		}
	}

	// Send completion event
	eventChan <- StreamEvent{
		Type: "done",
		Response: &CompletionResponse{
			Content: fullContent.String(),
			Model:   o.model,
		},
	}
}

// API Response structures

type ollamaAPIResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
}

type ollamaStreamEvent struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
}

package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GeminiAPIClient is a direct HTTP client for Google Gemini API.
type GeminiAPIClient struct {
	apiKey string
	model  string
	client *http.Client
}

// NewGeminiAPIClient creates a new Gemini API client.
func NewGeminiAPIClient(apiKey, model string) *GeminiAPIClient {
	return &GeminiAPIClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// Complete sends a non-streaming completion request to Gemini API.
func (g *GeminiAPIClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()

	body := g.buildRequestBody(req)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.model, url.QueryEscape(g.apiKey))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
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

	var result geminiAPIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return g.responseToCompletion(&result, time.Since(start)), nil
}

// Stream sends a streaming completion request to Gemini API.
func (g *GeminiAPIClient) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamEvent, error) {
	eventChan := make(chan StreamEvent)

	body := g.buildRequestBody(req)
	payload, err := json.Marshal(body)
	if err != nil {
		close(eventChan)
		return eventChan, fmt.Errorf("failed to marshal request: %w", err)
	}

	go g.streamRequest(ctx, eventChan, payload)
	return eventChan, nil
}

// Name returns the provider name.
func (g *GeminiAPIClient) Name() string {
	return "gemini"
}

// Helper methods

func (g *GeminiAPIClient) buildRequestBody(req CompletionRequest) map[string]interface{} {
	contents := []map[string]interface{}{
		{
			"role": "user",
			"parts": []map[string]string{
				{"text": g.buildPrompt(req)},
			},
		},
	}

	body := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": req.MaxTokens,
		},
	}

	if req.Temperature != nil {
		body["generationConfig"].(map[string]interface{})["temperature"] = *req.Temperature
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]interface{}, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]interface{}{
				"functionDeclarations": []map[string]interface{}{
					{
						"name":        t.Name,
						"description": t.Description,
						"parameters": parseJSONSchema(t.InputSchema),
					},
				},
			}
		}
		body["tools"] = tools
	}

	return body
}

func (g *GeminiAPIClient) buildPrompt(req CompletionRequest) string {
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

func (g *GeminiAPIClient) streamRequest(ctx context.Context, eventChan chan StreamEvent, payload []byte) {
	defer close(eventChan)

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?key=%s",
		g.model, url.QueryEscape(g.apiKey))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(payload)))
	if err != nil {
		eventChan <- StreamEvent{Type: "error", Error: fmt.Sprintf("request creation failed: %v", err)}
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
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

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event geminiStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		for _, candidate := range event.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					fullContent.WriteString(part.Text)
					eventChan <- StreamEvent{
						Type:    "delta",
						Content: part.Text,
					}
				}
			}
		}
	}

	// Send completion event
	eventChan <- StreamEvent{
		Type: "done",
		Response: &CompletionResponse{
			Content: fullContent.String(),
			Model:   g.model,
		},
	}
}

func (g *GeminiAPIClient) responseToCompletion(resp *geminiAPIResponse, duration time.Duration) *CompletionResponse {
	var content strings.Builder
	var toolCalls []ToolCall

	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				content.WriteString(part.Text)
			}
			if part.FunctionCall != nil {
				input, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, ToolCall{
					Name:  part.FunctionCall.Name,
					Input: string(input),
				})
			}
		}
	}

	stopReason := ""
	if len(resp.Candidates) > 0 && resp.Candidates[0].FinishReason != "" {
		stopReason = resp.Candidates[0].FinishReason
	}

	return &CompletionResponse{
		Content:    content.String(),
		StopReason: stopReason,
		ToolCalls:  toolCalls,
		Model:      g.model,
		Duration:   duration,
	}
}

// API Response structures

type geminiAPIResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

type geminiCandidate struct {
	Content struct {
		Parts []geminiPart `json:"parts"`
		Role  string       `json:"role"`
	} `json:"content"`
	FinishReason string `json:"finishReason"`
}

type geminiPart struct {
	Text         string `json:"text,omitempty"`
	FunctionCall *struct {
		Name string                 `json:"name"`
		Args map[string]interface{} `json:"args"`
	} `json:"functionCall,omitempty"`
}

type geminiStreamEvent struct {
	Candidates []geminiCandidate `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

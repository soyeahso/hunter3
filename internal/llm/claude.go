package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/soyeahso/hunter3/internal/logging"
)

// claudeJSONResult is the JSON output from `claude -p --output-format json`.
type claudeJSONResult struct {
	Type       string  `json:"type"`
	Subtype    string  `json:"subtype"`
	IsError    bool    `json:"is_error"`
	Result     string  `json:"result"`
	StopReason *string `json:"stop_reason"`
	SessionID  string  `json:"session_id"`
	DurationMs int     `json:"duration_ms"`
	CostUSD    float64 `json:"total_cost_usd"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		CacheRead    int `json:"cache_read_input_tokens"`
		CacheWrite   int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

// claudeStreamMessage is a line from `claude -p --output-format stream-json --verbose`.
type claudeStreamMessage struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`

	// For type="assistant"
	Message *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content,omitempty"`
	} `json:"message,omitempty"`

	// For type="result"
	Result     string  `json:"result,omitempty"`
	IsError    bool    `json:"is_error,omitempty"`
	SessionID  string  `json:"session_id,omitempty"`
	DurationMs int     `json:"duration_ms,omitempty"`
	CostUSD    float64 `json:"total_cost_usd,omitempty"`
	Usage      *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		CacheRead    int `json:"cache_read_input_tokens"`
		CacheWrite   int `json:"cache_creation_input_tokens"`
	} `json:"usage,omitempty"`
}

// NewClaudeClient creates a Client that wraps the `claude` CLI.
func NewClaudeClient(log *logging.Logger) *CLIClient {
	return NewCLIClient(CLIConfig{
		Command:         "claude",
		ProviderName:    "claude",
		BuildArgs:       buildClaudeArgs,
		ParseResponse:   parseClaudeResponse,
		ParseStreamLine: parseClaudeStreamLine,
		PromptViaStdin:  true,
	}, log)
}

// NewClaudeClientWithCommand creates a Claude-protocol client using a custom binary
// (e.g., a different path or wrapper).
func NewClaudeClientWithCommand(command string, log *logging.Logger) *CLIClient {
	return NewCLIClient(CLIConfig{
		Command:         command,
		ProviderName:    "claude",
		BuildArgs:       buildClaudeArgs,
		ParseResponse:   parseClaudeResponse,
		ParseStreamLine: parseClaudeStreamLine,
		PromptViaStdin:  true,
	}, log)
}

func buildClaudeArgs(req CompletionRequest) []string {
	// Use stream-json for streaming requests, regular json for non-streaming
	outputFormat := "json"
	if req.Stream {
		outputFormat = "stream-json"
	}
	// SECURITY: --dangerously-skip-permissions is required for non-interactive
	// (piped stdin) mode. Tool execution is disabled below via --tools "".
	// This combination allows safe headless completion without filesystem or
	// shell access. Do not remove --tools "" without also removing this flag.
	args := []string{"-p", "--dangerously-skip-permissions", "--output-format", outputFormat}

	// Only use --verbose for streaming; it can inject non-JSON text into stdout
	// for the plain json output format, breaking the parser.
	if req.Stream {
		args = append(args, "--verbose")
	}

	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	if req.System != "" {
		args = append(args, "--system-prompt", req.System)
	}

	if req.MaxTokens > 0 {
		// Claude CLI doesn't have a direct max-tokens flag; use budget as proxy
		// (the CLI manages token limits internally)
	}

	// Disable built-in tools for pure completion (no file editing, etc.)
	args = append(args, "--tools", "")

	// The user message is piped via stdin (handled by CLIClient)
	return args
}

func parseClaudeResponse(data []byte) (*CompletionResponse, error) {
	lastResult, err := decodeClaudeJSON(data)
	if lastResult == nil {
		// Fallback: try line-by-line parsing in case the output
		// contains non-JSON lines (e.g. from --verbose diagnostics).
		lastResult, _ = decodeClaudeJSONLines(data)
	}

	if lastResult == nil {
		// Provide detailed error with context
		errMsg := fmt.Sprintf("no valid JSON object in claude output (%d bytes)", len(data))
		if err != nil {
			errMsg += fmt.Sprintf(": %v", err)
		}
		// Include a preview of the raw data for debugging (first 500 chars)
		preview := string(data)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		errMsg += fmt.Sprintf(" | raw prefix: %s", preview)
		return nil, fmt.Errorf("%s", errMsg)
	}

	if lastResult.IsError {
		return nil, &ProviderError{
			Provider: "claude",
			Message:  lastResult.Result,
		}
	}

	return &CompletionResponse{
		Content:   lastResult.Result,
		Model:     "", // Claude CLI doesn't echo model in JSON result at top level
		SessionID: lastResult.SessionID,
		CostUSD:   lastResult.CostUSD,
		Usage: Usage{
			InputTokens:  lastResult.Usage.InputTokens,
			OutputTokens: lastResult.Usage.OutputTokens,
			CacheRead:    lastResult.Usage.CacheRead,
			CacheWrite:   lastResult.Usage.CacheWrite,
		},
	}, nil
}

// decodeClaudeJSON tries to parse concatenated JSON objects from the raw output
// using json.Decoder. Returns the best "result" object found, or the last valid
// object if no result type is present.
func decodeClaudeJSON(data []byte) (*claudeJSONResult, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var bestResult *claudeJSONResult
	var lastObj *claudeJSONResult
	var lastErr error

	for dec.More() {
		var raw claudeJSONResult
		if err := dec.Decode(&raw); err != nil {
			lastErr = err
			break
		}
		lastObj = &raw
		if raw.Type == "result" || raw.Result != "" {
			bestResult = &raw
		}
	}

	if bestResult != nil {
		return bestResult, nil
	}
	return lastObj, lastErr
}

// decodeClaudeJSONLines tries line-by-line JSON parsing as a fallback.
// This handles output where non-JSON text (verbose diagnostics, progress
// indicators) is interspersed with JSON lines.
func decodeClaudeJSONLines(data []byte) (*claudeJSONResult, error) {
	var bestResult *claudeJSONResult
	var lastObj *claudeJSONResult

	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var raw claudeJSONResult
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}
		lastObj = &raw
		if raw.Type == "result" || raw.Result != "" {
			bestResult = &raw
		}
	}

	if bestResult != nil {
		return bestResult, nil
	}
	return lastObj, nil
}

func parseClaudeStreamLine(data []byte) (*StreamEvent, error) {
	var msg claudeStreamMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	switch msg.Type {
	case "system":
		// Init message — skip
		return nil, nil

	case "assistant":
		if msg.Message == nil || len(msg.Message.Content) == 0 {
			return nil, nil
		}
		var parts []string
		for _, c := range msg.Message.Content {
			if c.Type == "text" {
				parts = append(parts, c.Text)
			}
		}
		if len(parts) == 0 {
			return nil, nil
		}
		return &StreamEvent{
			Type:    "delta",
			Content: strings.Join(parts, ""),
		}, nil

	case "result":
		resp := &CompletionResponse{
			Content:   msg.Result,
			SessionID: msg.SessionID,
			CostUSD:   msg.CostUSD,
		}
		if msg.Usage != nil {
			resp.Usage = Usage{
				InputTokens:  msg.Usage.InputTokens,
				OutputTokens: msg.Usage.OutputTokens,
				CacheRead:    msg.Usage.CacheRead,
				CacheWrite:   msg.Usage.CacheWrite,
			}
		}
		if msg.IsError {
			return &StreamEvent{
				Type:  "error",
				Error: msg.Result,
			}, nil
		}
		return &StreamEvent{
			Type:     "done",
			Response: resp,
		}, nil

	default:
		return nil, nil
	}
}

package llm

import (
	"encoding/json"
	"fmt"

	"github.com/soyeahso/hunter3/internal/logging"
)

// ExternalCLIConfig configures a generic external CLI provider.
// The external CLI must support:
//   - Reading the prompt from stdin
//   - Producing JSON output with at least a "result" or "content" field
//   - Optionally producing newline-delimited JSON for streaming
type ExternalCLIConfig struct {
	// Command is the binary name (e.g., "gemini", "ollama").
	Command string

	// Name is the display name for this provider.
	Name string

	// BaseArgs are always-present arguments (e.g., ["run", "llama3"] for ollama).
	BaseArgs []string

	// ModelFlag is the flag to pass the model name (e.g., "--model"). Empty to skip.
	ModelFlag string

	// SystemFlag is the flag to pass the system prompt. Empty to skip.
	SystemFlag string

	// JSONFlag are the flags to request JSON output (e.g., ["--format", "json"]).
	JSONFlag []string

	// StreamFlag are the flags to request streaming output. Empty means no streaming.
	StreamFlag []string

	// ResultField is the JSON field name containing the result text (default: "result").
	ResultField string

	// StreamTextField is the JSON field in each stream line containing text (default: "content").
	StreamTextField string
}

// externalJSONResult is a generic JSON response from an external CLI.
type externalJSONResult struct {
	Result  string `json:"result"`
	Content string `json:"content"`
	Output  string `json:"output"`
	Text    string `json:"text"`
	Error   string `json:"error"`
}

// NewExternalCLIClient creates a Client from an ExternalCLIConfig.
func NewExternalCLIClient(ecfg ExternalCLIConfig, log *logging.Logger) *CLIClient {
	if ecfg.ResultField == "" {
		ecfg.ResultField = "result"
	}
	if ecfg.StreamTextField == "" {
		ecfg.StreamTextField = "content"
	}

	return NewCLIClient(CLIConfig{
		Command:      ecfg.Command,
		ProviderName: ecfg.Name,
		BuildArgs: func(req CompletionRequest) []string {
			return buildExternalArgs(ecfg, req)
		},
		ParseResponse: func(data []byte) (*CompletionResponse, error) {
			return parseExternalResponse(ecfg, data)
		},
		ParseStreamLine: func(data []byte) (*StreamEvent, error) {
			return parseExternalStreamLine(ecfg, data)
		},
	}, log)
}

func buildExternalArgs(ecfg ExternalCLIConfig, req CompletionRequest) []string {
	args := make([]string, len(ecfg.BaseArgs))
	copy(args, ecfg.BaseArgs)

	if req.Model != "" && ecfg.ModelFlag != "" {
		args = append(args, ecfg.ModelFlag, req.Model)
	}

	if req.System != "" && ecfg.SystemFlag != "" {
		args = append(args, ecfg.SystemFlag, req.System)
	}

	if req.Stream && len(ecfg.StreamFlag) > 0 {
		args = append(args, ecfg.StreamFlag...)
	} else if len(ecfg.JSONFlag) > 0 {
		args = append(args, ecfg.JSONFlag...)
	}

	return args
}

func parseExternalResponse(ecfg ExternalCLIConfig, data []byte) (*CompletionResponse, error) {
	// Try structured JSON first
	var raw externalJSONResult
	if err := json.Unmarshal(data, &raw); err != nil {
		// If not JSON, treat the whole output as plain text
		return &CompletionResponse{Content: string(data)}, nil
	}

	if raw.Error != "" {
		return nil, &ProviderError{Provider: ecfg.Name, Message: raw.Error}
	}

	// Try multiple common field names
	content := raw.Result
	if content == "" {
		content = raw.Content
	}
	if content == "" {
		content = raw.Output
	}
	if content == "" {
		content = raw.Text
	}

	// If still empty, try the configured field name
	if content == "" {
		content = parseCLIJSONField(data, ecfg.ResultField)
	}

	if content == "" {
		return nil, fmt.Errorf("no text found in %s output", ecfg.Name)
	}

	return &CompletionResponse{Content: content}, nil
}

func parseExternalStreamLine(ecfg ExternalCLIConfig, data []byte) (*StreamEvent, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		// Not JSON — treat as raw text delta
		return &StreamEvent{Type: "delta", Content: string(data)}, nil
	}

	// Check for done/error signals
	if typeField, ok := raw["type"]; ok {
		var t string
		json.Unmarshal(typeField, &t)
		if t == "done" || t == "result" || t == "end" {
			content := parseCLIJSONField(data, ecfg.ResultField)
			return &StreamEvent{
				Type:     "done",
				Response: &CompletionResponse{Content: content},
			}, nil
		}
		if t == "error" {
			errMsg := parseCLIJSONField(data, "error")
			if errMsg == "" {
				errMsg = parseCLIJSONField(data, "message")
			}
			return &StreamEvent{Type: "error", Error: errMsg}, nil
		}
	}

	// Extract text delta from the configured field
	if textField, ok := raw[ecfg.StreamTextField]; ok {
		var text string
		if err := json.Unmarshal(textField, &text); err == nil && text != "" {
			return &StreamEvent{Type: "delta", Content: text}, nil
		}
	}

	return nil, nil
}

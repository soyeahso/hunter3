package llm

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/soyeahso/hunter3/internal/logging"
)

// NewCopilotClient creates a Client that wraps the GitHub Copilot CLI (`copilot`).
func NewCopilotClient(log *logging.Logger) *CLIClient {
	return NewCLIClient(CLIConfig{
		Command:         "copilot",
		ProviderName:    "copilot",
		BuildArgs:       buildCopilotArgs,
		ParseResponse:   parseCopilotResponse,
		ParseStreamLine: parseCopilotStreamLine,
	}, log)
}

// NewCopilotClientWithCommand creates a Copilot-protocol client using a custom binary.
func NewCopilotClientWithCommand(command string, log *logging.Logger) *CLIClient {
	return NewCLIClient(CLIConfig{
		Command:         command,
		ProviderName:    "copilot",
		BuildArgs:       buildCopilotArgs,
		ParseResponse:   parseCopilotResponse,
		ParseStreamLine: parseCopilotStreamLine,
	}, log)
}

func buildCopilotArgs(req CompletionRequest) []string {
	// Build the prompt text from the last user message.
	var prompt string
	if len(req.Messages) > 0 {
		last := req.Messages[len(req.Messages)-1]
		if last.Role == RoleUser {
			prompt = last.Content
		}
	}

	// -p / --prompt for non-interactive mode; --allow-all to skip permission prompts.
	args := []string{
		"-p", prompt,
		"--allow-all",
	}

	// Add MCP server config if the file exists.
	if mcpPath := copilotMCPConfigPath(); mcpPath != "" {
		args = append(args, "--additional-mcp-config", "@"+mcpPath)
	}

	if req.Model != "" && req.Model != "copilot" {
		args = append(args, "--model", req.Model)
	}

	// Copilot CLI doesn't have --output-format json yet (github/copilot-cli#52).
	// Output is plain text; we parse it as-is.

	return args
}

// parseCopilotResponse handles plain-text output from the copilot CLI.
// Since copilot doesn't support JSON output, we treat the entire stdout as the response.
func parseCopilotResponse(data []byte) (*CompletionResponse, error) {
	text := strings.TrimSpace(string(data))
	return &CompletionResponse{
		Content: text,
	}, nil
}

// parseCopilotStreamLine handles streaming output from copilot.
// Copilot streams plain text line-by-line; each non-empty line is a delta.
func parseCopilotStreamLine(data []byte) (*StreamEvent, error) {
	line := bytes.TrimSpace(data)
	if len(line) == 0 {
		return nil, nil
	}
	return &StreamEvent{
		Type:    "delta",
		Content: string(line) + "\n",
	}, nil
}

// copilotMCPConfigPath returns the path to the MCP server config file,
// or "" if no config file is found. It checks ~/.hunter3/mcp-servers.json
// first, then falls back to ~/.copilot/mcp-config.json.
func copilotMCPConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidates := []string{
		filepath.Join(home, ".hunter3", "mcp-servers.json"),
		filepath.Join(home, ".copilot", "mcp-config.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

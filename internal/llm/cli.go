package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/soyeahso/hunter3/internal/logging"
)

// CLIConfig configures a CLI-based LLM provider.
type CLIConfig struct {
	// Command is the CLI binary name (e.g., "claude", "gemini").
	Command string

	// BuildArgs turns a CompletionRequest into CLI arguments.
	// Each provider implements this to map the generic request to its CLI flags.
	BuildArgs func(req CompletionRequest) []string

	// ParseResponse parses the CLI's JSON output into a CompletionResponse.
	ParseResponse func(data []byte) (*CompletionResponse, error)

	// ParseStreamLine parses a single line of streaming JSON output.
	ParseStreamLine func(data []byte) (*StreamEvent, error)

	// ProviderName is the display name for this provider.
	ProviderName string

	// PromptViaStdin pipes the last user message to the CLI's stdin.
	// Set to true for CLIs that read their prompt from stdin (e.g., claude -p).
	// Set to false for CLIs that take the prompt as a flag argument (e.g., copilot -p "msg").
	PromptViaStdin bool
}

// CLIClient wraps any CLI tool as an LLM provider.
type CLIClient struct {
	cfg CLIConfig
	log *logging.Logger
}

// NewCLIClient creates a new CLI-based LLM client.
func NewCLIClient(cfg CLIConfig, log *logging.Logger) *CLIClient {
	return &CLIClient{cfg: cfg, log: log.Sub("llm." + cfg.ProviderName)}
}

// Name returns the provider name.
func (c *CLIClient) Name() string { return c.cfg.ProviderName }

// Complete runs the CLI synchronously and returns the full response.
func (c *CLIClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	req.Stream = false
	args := c.cfg.BuildArgs(req)

	c.log.Debug().
		Str("cmd", c.cfg.Command).
		Strs("args", args).
		Msg("running completion")

	start := time.Now()

	cmd := exec.CommandContext(ctx, c.cfg.Command, args...)
	if c.cfg.PromptViaStdin {
		if req.Messages != nil && len(req.Messages) > 0 {
			last := req.Messages[len(req.Messages)-1]
			if last.Role == RoleUser {
				cmd.Stdin = strings.NewReader(last.Content)
			}
		}
	}

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s exited %d: %s", c.cfg.Command, exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("%s: %w", c.cfg.Command, err)
	}

	resp, err := c.cfg.ParseResponse(out)
	if err != nil {
		return nil, fmt.Errorf("parsing %s output: %w", c.cfg.Command, err)
	}

	resp.Duration = time.Since(start)

	c.log.Debug().
		Str("model", resp.Model).
		Int("inputTokens", resp.Usage.InputTokens).
		Int("outputTokens", resp.Usage.OutputTokens).
		Dur("duration", resp.Duration).
		Msg("completion done")

	return resp, nil
}

// Stream runs the CLI with streaming output and returns events on a channel.
func (c *CLIClient) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamEvent, error) {
	req.Stream = true
	args := c.cfg.BuildArgs(req)

	cmd := exec.CommandContext(ctx, c.cfg.Command, args...)
	if c.cfg.PromptViaStdin {
		if req.Messages != nil && len(req.Messages) > 0 {
			last := req.Messages[len(req.Messages)-1]
			if last.Role == RoleUser {
				cmd.Stdin = strings.NewReader(last.Content)
			}
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	// Capture stderr so we can report CLI errors that would otherwise be lost.
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", c.cfg.Command, err)
	}

	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)

		c.streamOutput(stdout, ch)

		// Wait for the process to exit and check for errors.
		if err := cmd.Wait(); err != nil {
			stderr := strings.TrimSpace(stderrBuf.String())
			if stderr == "" {
				stderr = err.Error()
			}
			c.log.Error().
				Str("cmd", c.cfg.Command).
				Str("stderr", stderr).
				Err(err).
				Msg("CLI process failed")
			ch <- StreamEvent{
				Type:  "error",
				Error: fmt.Sprintf("%s: %s", c.cfg.Command, stderr),
			}
		}
	}()

	return ch, nil
}

// streamOutput reads lines from the CLI's stdout and parses them into stream events.
func (c *CLIClient) streamOutput(r io.Reader, ch chan<- StreamEvent) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		evt, err := c.cfg.ParseStreamLine(line)
		if err != nil {
			c.log.Debug().Err(err).Msg("skipping unparseable stream line")
			continue
		}
		if evt != nil {
			ch <- *evt
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamEvent{Type: "error", Error: err.Error()}
	}
}

// CLIExists checks whether a CLI command is available in PATH.
func CLIExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// parseCLIJSONField is a helper to extract a string field from JSON bytes.
func parseCLIJSONField(data []byte, field string) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	val, ok := raw[field]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(val, &s); err != nil {
		return ""
	}
	return s
}

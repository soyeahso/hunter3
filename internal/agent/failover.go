package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/soyeahso/hunter3/internal/llm"
	"github.com/soyeahso/hunter3/internal/logging"
)

// FailoverClient wraps an LLM registry to try fallback providers on failure.
type FailoverClient struct {
	registry *llm.Registry
	primary  string
	fallbacks []string
	log      *logging.Logger
}

// NewFailoverClient creates a client that tries the primary model first,
// then falls back through the list on retryable errors (401, 429, 5xx).
func NewFailoverClient(registry *llm.Registry, primary string, fallbacks []string, log *logging.Logger) *FailoverClient {
	return &FailoverClient{
		registry:  registry,
		primary:   primary,
		fallbacks: fallbacks,
		log:       log.Sub("failover"),
	}
}

// Complete tries the primary provider, falling back on retryable errors.
func (f *FailoverClient) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	models := append([]string{f.primary}, f.fallbacks...)

	var lastErr error
	for _, model := range models {
		client, err := f.registry.Resolve(model)
		if err != nil {
			f.log.Debug().Str("model", model).Err(err).Msg("no provider for model, skipping")
			lastErr = err
			continue
		}

		req.Model = model
		resp, err := client.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		if isRetryable(err) {
			f.log.Warn().
				Str("model", model).
				Err(err).
				Msg("retryable error, trying next provider")
			continue
		}

		// Non-retryable error — don't try more providers
		return nil, err
	}

	return nil, lastErr
}

// Stream tries the primary provider for streaming, with failover.
func (f *FailoverClient) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamEvent, error) {
	models := append([]string{f.primary}, f.fallbacks...)

	var lastErr error
	for _, model := range models {
		client, err := f.registry.Resolve(model)
		if err != nil {
			lastErr = err
			continue
		}

		req.Model = model
		ch, err := client.Stream(ctx, req)
		if err == nil {
			return ch, nil
		}

		lastErr = err

		if isRetryable(err) {
			f.log.Warn().
				Str("model", model).
				Err(err).
				Msg("retryable stream error, trying next provider")
			continue
		}

		return nil, err
	}

	return nil, lastErr
}

// isRetryable checks if the error suggests trying another provider.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	var provErr *llm.ProviderError
	if errors.As(err, &provErr) {
		switch provErr.Code {
		case 401, 403, 429, 500, 502, 503, 529:
			return true
		}
	}

	msg := err.Error()
	return strings.Contains(msg, "overloaded") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "capacity") ||
		strings.Contains(msg, "timeout")
}

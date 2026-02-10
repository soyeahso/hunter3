package routing

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/logging"
)

// StreamFlusherConfig controls when buffered deltas are flushed to the channel.
type StreamFlusherConfig struct {
	// MaxBufferBytes triggers a flush when the buffer reaches this size.
	// Default: 300 bytes.
	MaxBufferBytes int

	// IdleTimeout triggers a flush when no new delta arrives within this duration.
	// Default: 2 seconds.
	IdleTimeout time.Duration
}

// StreamFlusher accumulates streaming LLM deltas and flushes them to a channel
// at natural text boundaries (sentences, paragraphs, size limit, idle timeout).
type StreamFlusher struct {
	cfg    StreamFlusherConfig
	ch     domain.Channel
	ctx    context.Context
	chanID string
	target string
	log    *logging.Logger

	mu      sync.Mutex
	buf     strings.Builder
	timer   *time.Timer
	flushed bool
}

// NewStreamFlusher creates a flusher that sends incremental chunks to the given channel.
func NewStreamFlusher(
	ctx context.Context,
	cfg StreamFlusherConfig,
	ch domain.Channel,
	chanID, target string,
	log *logging.Logger,
) *StreamFlusher {
	if cfg.MaxBufferBytes <= 0 {
		cfg.MaxBufferBytes = 300
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 2 * time.Second
	}
	return &StreamFlusher{
		cfg:    cfg,
		ch:     ch,
		ctx:    ctx,
		chanID: chanID,
		target: target,
		log:    log,
	}
}

// OnDelta appends a text delta to the buffer and flushes if a boundary is reached.
func (f *StreamFlusher) OnDelta(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.buf.WriteString(text)

	// Reset idle timer
	if f.timer != nil {
		f.timer.Stop()
	}
	f.timer = time.AfterFunc(f.cfg.IdleTimeout, func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.flushLocked()
	})

	f.checkFlushLocked()
}

// Flush sends any remaining buffered content. Call after the stream ends.
func (f *StreamFlusher) Flush() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.timer != nil {
		f.timer.Stop()
	}
	f.flushLocked()
}

// Flushed returns true if at least one chunk was sent.
func (f *StreamFlusher) Flushed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.flushed
}

// checkFlushLocked examines the buffer for natural flush boundaries.
func (f *StreamFlusher) checkFlushLocked() {
	content := f.buf.String()

	// Size threshold
	if len(content) >= f.cfg.MaxBufferBytes {
		f.flushLocked()
		return
	}

	// Paragraph boundary: double newline
	if idx := strings.LastIndex(content, "\n\n"); idx >= 0 {
		f.flushAtLocked(idx + 2)
		return
	}

	// Sentence boundary
	if pos := lastSentenceEnd(content); pos > 0 {
		f.flushAtLocked(pos)
		return
	}
}

// flushAtLocked sends the first pos bytes of the buffer and keeps the rest.
func (f *StreamFlusher) flushAtLocked(pos int) {
	content := f.buf.String()
	if pos > len(content) {
		pos = len(content)
	}
	toSend := strings.TrimSpace(content[:pos])
	if toSend == "" {
		return
	}

	f.sendLocked(toSend)

	remainder := content[pos:]
	f.buf.Reset()
	f.buf.WriteString(remainder)
}

// flushLocked sends the entire buffer.
func (f *StreamFlusher) flushLocked() {
	content := strings.TrimSpace(f.buf.String())
	if content == "" {
		return
	}
	f.sendLocked(content)
	f.buf.Reset()
}

// sendLocked delivers one chunk via the channel.
func (f *StreamFlusher) sendLocked(body string) {
	msg := domain.OutboundMessage{
		ChannelID: f.chanID,
		To:        f.target,
		Body:      body,
	}
	if err := f.ch.Send(f.ctx, msg); err != nil {
		f.log.Error().Err(err).
			Str("channel", f.chanID).
			Str("to", f.target).
			Msg("failed to send stream chunk")
	}
	f.flushed = true
}

// lastSentenceEnd returns the byte position just past the last sentence-ending
// punctuation (. ! ?) that is followed by a space or newline. Returns -1 if no
// suitable boundary is found or the buffer is too small (< 40 bytes).
func lastSentenceEnd(s string) int {
	best := -1
	for i := 0; i < len(s)-1; i++ {
		if (s[i] == '.' || s[i] == '!' || s[i] == '?') &&
			(s[i+1] == ' ' || s[i+1] == '\n') {
			best = i + 1
		}
	}
	if best > 40 {
		return best
	}
	return -1
}

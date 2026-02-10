package routing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamFlusher_SentenceFlush(t *testing.T) {
	ch := &mockChannel{id: "irc"}
	log := testLogger()

	f := NewStreamFlusher(
		context.Background(),
		StreamFlusherConfig{MaxBufferBytes: 300, IdleTimeout: 5 * time.Second},
		ch,
		"irc", "#test",
		log,
	)

	// Feed text that forms a sentence boundary (> 40 bytes min)
	f.OnDelta("This is the first sentence of a response. ")
	f.OnDelta("And this is the second one.")

	// First sentence should have been flushed (52 chars > 40 byte min)
	require.GreaterOrEqual(t, len(ch.sent), 1)
	assert.Contains(t, ch.sent[0].Body, "first sentence")

	f.Flush()
	// After Flush(), all content should be sent
	var all strings.Builder
	for _, m := range ch.sent {
		all.WriteString(m.Body)
		all.WriteString(" ")
	}
	assert.Contains(t, all.String(), "second one")
}

func TestStreamFlusher_ParagraphFlush(t *testing.T) {
	ch := &mockChannel{id: "irc"}
	log := testLogger()

	f := NewStreamFlusher(
		context.Background(),
		StreamFlusherConfig{MaxBufferBytes: 500, IdleTimeout: 5 * time.Second},
		ch,
		"irc", "#test",
		log,
	)

	f.OnDelta("First paragraph.\n\nSecond paragraph.")

	require.GreaterOrEqual(t, len(ch.sent), 1)
	assert.Equal(t, "First paragraph.", ch.sent[0].Body)

	f.Flush()
	assert.Len(t, ch.sent, 2)
	assert.Equal(t, "Second paragraph.", ch.sent[1].Body)
}

func TestStreamFlusher_SizeThreshold(t *testing.T) {
	ch := &mockChannel{id: "irc"}
	log := testLogger()

	f := NewStreamFlusher(
		context.Background(),
		StreamFlusherConfig{MaxBufferBytes: 50, IdleTimeout: 5 * time.Second},
		ch,
		"irc", "#test",
		log,
	)

	// Single delta exceeding the threshold, no sentence boundaries
	f.OnDelta(strings.Repeat("abcde ", 15)) // 90 bytes

	require.GreaterOrEqual(t, len(ch.sent), 1)
	assert.True(t, f.Flushed())
}

func TestStreamFlusher_IdleTimeout(t *testing.T) {
	ch := &mockChannel{id: "irc"}
	log := testLogger()

	f := NewStreamFlusher(
		context.Background(),
		StreamFlusherConfig{MaxBufferBytes: 1000, IdleTimeout: 50 * time.Millisecond},
		ch,
		"irc", "#test",
		log,
	)

	f.OnDelta("short text")

	// Nothing flushed immediately (no boundary, under size)
	assert.Empty(t, ch.sent)

	// Wait for idle timeout
	time.Sleep(150 * time.Millisecond)

	require.Len(t, ch.sent, 1)
	assert.Equal(t, "short text", ch.sent[0].Body)
}

func TestStreamFlusher_FinalFlush(t *testing.T) {
	ch := &mockChannel{id: "irc"}
	log := testLogger()

	f := NewStreamFlusher(
		context.Background(),
		StreamFlusherConfig{MaxBufferBytes: 1000, IdleTimeout: 5 * time.Second},
		ch,
		"irc", "#test",
		log,
	)

	f.OnDelta("partial content")
	assert.Empty(t, ch.sent) // too small, no boundary

	f.Flush()
	require.Len(t, ch.sent, 1)
	assert.Equal(t, "partial content", ch.sent[0].Body)
	assert.True(t, f.Flushed())
}

func TestStreamFlusher_EmptyFlush(t *testing.T) {
	ch := &mockChannel{id: "irc"}
	log := testLogger()

	f := NewStreamFlusher(
		context.Background(),
		StreamFlusherConfig{},
		ch,
		"irc", "#test",
		log,
	)

	f.Flush()
	assert.Empty(t, ch.sent)
	assert.False(t, f.Flushed())
}

func TestStreamFlusher_MessageTarget(t *testing.T) {
	ch := &mockChannel{id: "irc"}
	log := testLogger()

	f := NewStreamFlusher(
		context.Background(),
		StreamFlusherConfig{MaxBufferBytes: 10, IdleTimeout: 5 * time.Second},
		ch,
		"irc", "#mychannel",
		log,
	)

	f.OnDelta(strings.Repeat("x", 20))
	f.Flush()

	require.GreaterOrEqual(t, len(ch.sent), 1)
	for _, m := range ch.sent {
		assert.Equal(t, "#mychannel", m.To)
		assert.Equal(t, "irc", m.ChannelID)
	}
}

func TestLastSentenceEnd(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"period space", "This is a sentence that is long enough to pass. Next", 47},
		{"exclamation", "This is exciting and over forty bytes long! Yes", 43},
		{"question", "Is this a question that is long enough really? Yes", 46},
		{"too short", "Hi. X", -1},
		{"no boundary", "no sentence ending here", -1},
		{"empty", "", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastSentenceEnd(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

package logging

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "info")
	require.NotNil(t, log)

	log.Info().Msg("test message")
	assert.Contains(t, buf.String(), "test message")
}

func TestNewDefaultWriter(t *testing.T) {
	// nil writer should default to stderr console writer
	log := New(nil, "info")
	require.NotNil(t, log)
}

func TestSub(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "debug")
	sub := log.Sub("mymodule")
	require.NotNil(t, sub)

	sub.Info().Msg("sub message")
	output := buf.String()
	assert.Contains(t, output, "sub message")
	assert.Contains(t, output, "mymodule")
}

func TestSubChain(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "debug")
	sub1 := log.Sub("level1")
	sub2 := sub1.Sub("level2")

	sub2.Info().Msg("deep message")
	output := buf.String()
	assert.Contains(t, output, "deep message")
	assert.Contains(t, output, "level2")
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "warn")

	log.Debug().Msg("debug msg")
	log.Info().Msg("info msg")
	assert.Empty(t, buf.String(), "debug and info should be filtered at warn level")

	log.Warn().Msg("warn msg")
	assert.Contains(t, buf.String(), "warn msg")

	buf.Reset()
	log.Error().Msg("error msg")
	assert.Contains(t, buf.String(), "error msg")
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  zerolog.Level
	}{
		{"trace", zerolog.TraceLevel},
		{"debug", zerolog.DebugLevel},
		{"info", zerolog.InfoLevel},
		{"warn", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"fatal", zerolog.FatalLevel},
		{"silent", zerolog.Disabled},
		{"", zerolog.InfoLevel},
		{"unknown", zerolog.InfoLevel},
		{"INFO", zerolog.InfoLevel}, // case-sensitive, defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parseLevel(tt.input))
		})
	}
}

func TestZerolog(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "info")
	zl := log.Zerolog()
	assert.NotZero(t, zl)

	zl.Info().Msg("direct zerolog")
	assert.Contains(t, buf.String(), "direct zerolog")
}

func TestSilentLevel(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "silent")

	log.Debug().Msg("should not appear")
	log.Info().Msg("should not appear")
	log.Warn().Msg("should not appear")
	log.Error().Msg("should not appear")

	assert.Empty(t, buf.String())
}

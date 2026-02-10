package logging

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Logger wraps zerolog to provide subsystem-scoped child loggers.
type Logger struct {
	zl zerolog.Logger
}

// New creates a root logger writing to the given writer at the specified level.
// If w is nil, defaults to pretty console output on stderr.
func New(w io.Writer, level string) *Logger {
	if w == nil {
		w = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	}
	zl := zerolog.New(w).With().Timestamp().Logger()
	zl = zl.Level(parseLevel(level))
	return &Logger{zl: zl}
}

// Sub returns a child logger tagged with a subsystem name.
func (l *Logger) Sub(subsystem string) *Logger {
	return &Logger{zl: l.zl.With().Str("subsystem", subsystem).Logger()}
}

// Debug logs at debug level.
func (l *Logger) Debug() *zerolog.Event { return l.zl.Debug() }

// Info logs at info level.
func (l *Logger) Info() *zerolog.Event { return l.zl.Info() }

// Warn logs at warn level.
func (l *Logger) Warn() *zerolog.Event { return l.zl.Warn() }

// Error logs at error level.
func (l *Logger) Error() *zerolog.Event { return l.zl.Error() }

// Fatal logs at fatal level and exits.
func (l *Logger) Fatal() *zerolog.Event { return l.zl.Fatal() }

// Zerolog returns the underlying zerolog.Logger for advanced use.
func (l *Logger) Zerolog() zerolog.Logger { return l.zl }

func parseLevel(s string) zerolog.Level {
	switch s {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "silent":
		return zerolog.Disabled
	default:
		return zerolog.InfoLevel
	}
}

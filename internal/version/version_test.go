package version

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInfoDefault(t *testing.T) {
	info := Info()
	assert.Contains(t, info, "hunter3")
	assert.Contains(t, info, Version)
	assert.Contains(t, info, runtime.GOOS)
	assert.Contains(t, info, runtime.GOARCH)
}

func TestInfoWithCustomValues(t *testing.T) {
	origVersion := Version
	origCommit := Commit
	origDate := Date
	t.Cleanup(func() {
		Version = origVersion
		Commit = origCommit
		Date = origDate
	})

	Version = "1.2.3"
	Commit = "abc1234567890"
	Date = "2026-01-15"

	info := Info()
	assert.Contains(t, info, "1.2.3")
	assert.Contains(t, info, "abc1234") // truncated to 7 chars
	assert.NotContains(t, info, "abc1234567890")
	assert.Contains(t, info, "2026-01-15")
}

func TestShort(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abcdefghij", "abcdefg"},
		{"abc1234", "abc1234"},
		{"abc", "abc"},
		{"", ""},
		{"1234567", "1234567"},
		{"12345678", "1234567"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, short(tt.input))
		})
	}
}

func TestDefaultValues(t *testing.T) {
	// Verify default build-time values
	assert.Equal(t, "dev", Version)
	assert.Equal(t, "unknown", Commit)
	assert.Equal(t, "unknown", Date)
}

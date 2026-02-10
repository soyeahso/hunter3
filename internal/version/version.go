package version

import (
	"fmt"
	"runtime"
)

// Set via ldflags at build time:
//
//	go build -ldflags "-X github.com/soyeahso/hunter3/internal/version.Version=1.0.0
//	  -X github.com/soyeahso/hunter3/internal/version.Commit=abc123
//	  -X github.com/soyeahso/hunter3/internal/version.Date=2026-01-01"
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Info returns a formatted version string.
func Info() string {
	return fmt.Sprintf("hunter3 %s (commit: %s, built: %s, %s/%s)",
		Version, short(Commit), Date, runtime.GOOS, runtime.GOARCH)
}

func short(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

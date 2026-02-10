package config

import (
	"os"
	"path/filepath"
	"strings"
)

const defaultBaseDir = ".hunter3"

// Paths holds resolved filesystem paths for Hunter3 data.
type Paths struct {
	Base        string // ~/.hunter3
	Config      string // ~/.hunter3/config.yaml
	Credentials string // ~/.hunter3/credentials
	Sessions    string // ~/.hunter3/sessions
	Agents      string // ~/.hunter3/agents
	Logs        string // ~/.hunter3/logs
	Plugins     string // ~/.hunter3/plugins
	Data        string // ~/.hunter3/data
}

// ResolvePaths computes all standard paths from the home directory.
// If HUNTER3_HOME is set, it overrides the default base directory.
func ResolvePaths() (Paths, error) {
	base := os.Getenv("HUNTER3_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, err
		}
		base = filepath.Join(home, defaultBaseDir)
	}

	return Paths{
		Base:        base,
		Config:      filepath.Join(base, "config.yaml"),
		Credentials: filepath.Join(base, "credentials"),
		Sessions:    filepath.Join(base, "sessions"),
		Agents:      filepath.Join(base, "agents"),
		Logs:        filepath.Join(base, "logs"),
		Plugins:     filepath.Join(base, "plugins"),
		Data:        filepath.Join(base, "data"),
	}, nil
}

// EnsureDirs creates all standard directories if they don't exist.
func (p Paths) EnsureDirs() error {
	dirs := []string{p.Base, p.Credentials, p.Sessions, p.Agents, p.Logs, p.Plugins, p.Data}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}
	return nil
}

// blockedKeys are keys that must never appear in config paths.
var blockedKeys = map[string]bool{
	"__proto__":   true,
	"prototype":   true,
	"constructor": true,
}

// ParseConfigPath splits a dot-separated config path into segments.
// Returns an error if any segment is blocked or empty.
func ParseConfigPath(raw string) ([]string, error) {
	if raw == "" {
		return nil, &ConfigError{Message: "empty config path"}
	}
	parts := strings.Split(raw, ".")
	for _, p := range parts {
		if p == "" {
			return nil, &ConfigError{Message: "config path contains empty segment"}
		}
		if blockedKeys[p] {
			return nil, &ConfigError{Message: "config path contains blocked key: " + p}
		}
	}
	return parts, nil
}

// GetValueAtPath traverses a nested map using the given path segments.
func GetValueAtPath(root map[string]any, path []string) (any, bool) {
	current := any(root)
	for _, key := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// SetValueAtPath sets a value in a nested map, creating intermediate maps as needed.
func SetValueAtPath(root map[string]any, path []string, value any) {
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key]
		if !ok {
			next = map[string]any{}
			current[key] = next
		}
		m, ok := next.(map[string]any)
		if !ok {
			m = map[string]any{}
			current[key] = m
		}
		current = m
	}
	current[path[len(path)-1]] = value
}

// UnsetValueAtPath removes a value at the given path. Returns true if removed.
func UnsetValueAtPath(root map[string]any, path []string) bool {
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key]
		if !ok {
			return false
		}
		m, ok := next.(map[string]any)
		if !ok {
			return false
		}
		current = m
	}
	last := path[len(path)-1]
	if _, ok := current[last]; !ok {
		return false
	}
	delete(current, last)
	return true
}

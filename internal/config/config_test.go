package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	assert.Equal(t, 18789, cfg.Gateway.Port)
	assert.Equal(t, "local", cfg.Gateway.Mode)
	assert.Equal(t, "loopback", cfg.Gateway.Bind)
	assert.Equal(t, "token", cfg.Gateway.Auth.Mode)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "per-sender", cfg.Session.Scope)
	assert.Equal(t, 30, cfg.Session.IdleMinutes)
	assert.Equal(t, "sqlite", cfg.Session.Store)
	assert.Equal(t, true, cfg.Memory.Enabled)
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	require.NoError(t, err)
	// Should return defaults
	assert.Equal(t, 18789, cfg.Gateway.Port)
	assert.Equal(t, "info", cfg.Logging.Level)
}

func TestLoadValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `
gateway:
  port: 9999
  mode: remote
  bind: lan
  auth:
    mode: password
    password: secret123
logging:
  level: debug
  consoleStyle: json
session:
  scope: global
  idleMinutes: 60
channels:
  irc:
    server: irc.libera.chat
    port: 6697
    nick: testbot
    channels:
      - "#general"
      - "#dev"
    useTLS: true
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 9999, cfg.Gateway.Port)
	assert.Equal(t, "remote", cfg.Gateway.Mode)
	assert.Equal(t, "lan", cfg.Gateway.Bind)
	assert.Equal(t, "password", cfg.Gateway.Auth.Mode)
	assert.Equal(t, "secret123", cfg.Gateway.Auth.Password)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.ConsoleStyle)
	assert.Equal(t, "global", cfg.Session.Scope)
	assert.Equal(t, 60, cfg.Session.IdleMinutes)

	require.NotNil(t, cfg.Channels.IRC)
	assert.Equal(t, "irc.libera.chat", cfg.Channels.IRC.Server)
	assert.Equal(t, 6697, cfg.Channels.IRC.Port)
	assert.Equal(t, "testbot", cfg.Channels.IRC.Nick)
	assert.Equal(t, []string{"#general", "#dev"}, cfg.Channels.IRC.Channels)
	assert.True(t, cfg.Channels.IRC.UseTLS)
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{{invalid yaml"), 0o600))

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config")
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("HUNTER3_GATEWAY_PORT", "12345")
	t.Setenv("HUNTER3_LOG_LEVEL", "TRACE")

	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)

	assert.Equal(t, 12345, cfg.Gateway.Port)
	assert.Equal(t, "trace", cfg.Logging.Level)
}

func TestValidateValid(t *testing.T) {
	cfg := Defaults()
	issues := Validate(&cfg)
	assert.Empty(t, issues)
}

func TestValidateInvalidPort(t *testing.T) {
	cfg := Defaults()
	cfg.Gateway.Port = 99999
	issues := Validate(&cfg)
	require.Len(t, issues, 1)
	assert.Equal(t, "gateway.port", issues[0].Path)
}

func TestValidateInvalidMode(t *testing.T) {
	cfg := Defaults()
	cfg.Gateway.Mode = "invalid"
	issues := Validate(&cfg)
	require.Len(t, issues, 1)
	assert.Equal(t, "gateway.mode", issues[0].Path)
}

func TestValidateIRCMissingServer(t *testing.T) {
	cfg := Defaults()
	cfg.Channels.IRC = &IRCConfig{
		Nick: "bot",
	}
	issues := Validate(&cfg)
	require.NotEmpty(t, issues)

	var paths []string
	for _, i := range issues {
		paths = append(paths, i.Path)
	}
	assert.Contains(t, paths, "channels.irc.server")
}

func TestParseConfigPath(t *testing.T) {
	tests := []struct {
		input   string
		want    []string
		wantErr bool
	}{
		{"gateway.port", []string{"gateway", "port"}, false},
		{"channels.irc.server", []string{"channels", "irc", "server"}, false},
		{"", nil, true},
		{"a..b", nil, true},
		{"__proto__.x", nil, true},
		{"x.constructor", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseConfigPath(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGetSetValueAtPath(t *testing.T) {
	root := map[string]any{
		"gateway": map[string]any{
			"port": 18789,
		},
	}

	// Get existing
	val, ok := GetValueAtPath(root, []string{"gateway", "port"})
	assert.True(t, ok)
	assert.Equal(t, 18789, val)

	// Get missing
	_, ok = GetValueAtPath(root, []string{"gateway", "missing"})
	assert.False(t, ok)

	// Set existing
	SetValueAtPath(root, []string{"gateway", "port"}, 9999)
	val, ok = GetValueAtPath(root, []string{"gateway", "port"})
	assert.True(t, ok)
	assert.Equal(t, 9999, val)

	// Set new nested
	SetValueAtPath(root, []string{"channels", "irc", "server"}, "irc.libera.chat")
	val, ok = GetValueAtPath(root, []string{"channels", "irc", "server"})
	assert.True(t, ok)
	assert.Equal(t, "irc.libera.chat", val)
}

func TestUnsetValueAtPath(t *testing.T) {
	root := map[string]any{
		"gateway": map[string]any{
			"port": 18789,
			"mode": "local",
		},
	}

	ok := UnsetValueAtPath(root, []string{"gateway", "port"})
	assert.True(t, ok)

	_, exists := GetValueAtPath(root, []string{"gateway", "port"})
	assert.False(t, exists)

	// Mode should still be there
	val, exists := GetValueAtPath(root, []string{"gateway", "mode"})
	assert.True(t, exists)
	assert.Equal(t, "local", val)

	// Unset missing key
	ok = UnsetValueAtPath(root, []string{"gateway", "nonexistent"})
	assert.False(t, ok)
}

func TestLoadRawAndSaveRaw(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	raw := map[string]any{
		"gateway": map[string]any{
			"port": 9999,
		},
	}

	require.NoError(t, SaveRaw(path, raw))

	loaded, err := LoadRaw(path)
	require.NoError(t, err)

	val, ok := GetValueAtPath(loaded, []string{"gateway", "port"})
	assert.True(t, ok)
	assert.Equal(t, 9999, val)
}

func TestResolvePaths(t *testing.T) {
	t.Setenv("HUNTER3_HOME", t.TempDir())
	paths, err := ResolvePaths()
	require.NoError(t, err)
	assert.NotEmpty(t, paths.Base)
	assert.Contains(t, paths.Config, "config.yaml")
	assert.Contains(t, paths.Sessions, "sessions")
}

func TestResolvePathsCustomHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HUNTER3_HOME", tmp)

	paths, err := ResolvePaths()
	require.NoError(t, err)
	assert.Equal(t, tmp, paths.Base)
	assert.Equal(t, filepath.Join(tmp, "config.yaml"), paths.Config)
}

func TestEnsureDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HUNTER3_HOME", tmp)

	paths, err := ResolvePaths()
	require.NoError(t, err)
	require.NoError(t, paths.EnsureDirs())

	// Verify dirs exist
	for _, d := range []string{paths.Credentials, paths.Sessions, paths.Agents, paths.Logs, paths.Plugins, paths.Data} {
		info, err := os.Stat(d)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	}
}

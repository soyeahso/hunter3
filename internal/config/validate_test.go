package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidate_ValidDefaults(t *testing.T) {
	cfg := Defaults()
	issues := Validate(&cfg)
	assert.Empty(t, issues)
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := Defaults()

	cfg.Gateway.Port = -1
	issues := Validate(&cfg)
	assert.NotEmpty(t, issues)
	assert.Contains(t, issues[0].Path, "gateway.port")

	cfg.Gateway.Port = 70000
	issues = Validate(&cfg)
	assert.NotEmpty(t, issues)
}

func TestValidate_ValidPort(t *testing.T) {
	cfg := Defaults()
	cfg.Gateway.Port = 0
	assert.Empty(t, Validate(&cfg))

	cfg.Gateway.Port = 65535
	assert.Empty(t, Validate(&cfg))

	cfg.Gateway.Port = 8080
	assert.Empty(t, Validate(&cfg))
}

func TestValidate_InvalidMode(t *testing.T) {
	cfg := Defaults()
	cfg.Gateway.Mode = "invalid"
	issues := Validate(&cfg)
	assert.NotEmpty(t, issues)
	assert.Contains(t, issues[0].Path, "gateway.mode")
}

func TestValidate_ValidModes(t *testing.T) {
	for _, mode := range []string{"local", "remote", ""} {
		cfg := Defaults()
		cfg.Gateway.Mode = mode
		assert.Empty(t, Validate(&cfg), "mode %q should be valid", mode)
	}
}

func TestValidate_InvalidBind(t *testing.T) {
	cfg := Defaults()
	cfg.Gateway.Bind = "invalid"
	issues := Validate(&cfg)
	assert.NotEmpty(t, issues)
	assert.Contains(t, issues[0].Path, "gateway.bind")
}

func TestValidate_ValidBinds(t *testing.T) {
	for _, bind := range []string{"auto", "lan", "loopback", "custom", "tailnet", ""} {
		cfg := Defaults()
		cfg.Gateway.Bind = bind
		assert.Empty(t, Validate(&cfg), "bind %q should be valid", bind)
	}
}

func TestValidate_InvalidAuthMode(t *testing.T) {
	cfg := Defaults()
	cfg.Gateway.Auth.Mode = "oauth"
	issues := Validate(&cfg)
	assert.NotEmpty(t, issues)
	assert.Contains(t, issues[0].Path, "gateway.auth.mode")
}

func TestValidate_ValidAuthModes(t *testing.T) {
	for _, mode := range []string{"token", "password", ""} {
		cfg := Defaults()
		cfg.Gateway.Auth.Mode = mode
		assert.Empty(t, Validate(&cfg), "auth mode %q should be valid", mode)
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := Defaults()
	cfg.Logging.Level = "verbose"
	issues := Validate(&cfg)
	assert.NotEmpty(t, issues)
	assert.Contains(t, issues[0].Path, "logging.level")
}

func TestValidate_ValidLogLevels(t *testing.T) {
	for _, level := range []string{"silent", "fatal", "error", "warn", "info", "debug", "trace", ""} {
		cfg := Defaults()
		cfg.Logging.Level = level
		cfg.Logging.ConsoleLevel = level
		assert.Empty(t, Validate(&cfg), "log level %q should be valid", level)
	}
}

func TestValidate_InvalidConsoleLevel(t *testing.T) {
	cfg := Defaults()
	cfg.Logging.ConsoleLevel = "verbose"
	issues := Validate(&cfg)
	assert.NotEmpty(t, issues)
	assert.Contains(t, issues[0].Path, "logging.consoleLevel")
}

func TestValidate_InvalidConsoleStyle(t *testing.T) {
	cfg := Defaults()
	cfg.Logging.ConsoleStyle = "fancy"
	issues := Validate(&cfg)
	assert.NotEmpty(t, issues)
	assert.Contains(t, issues[0].Path, "logging.consoleStyle")
}

func TestValidate_ValidConsoleStyles(t *testing.T) {
	for _, style := range []string{"pretty", "compact", "json", ""} {
		cfg := Defaults()
		cfg.Logging.ConsoleStyle = style
		assert.Empty(t, Validate(&cfg), "console style %q should be valid", style)
	}
}

func TestValidate_InvalidSessionScope(t *testing.T) {
	cfg := Defaults()
	cfg.Session.Scope = "channel"
	issues := Validate(&cfg)
	assert.NotEmpty(t, issues)
	assert.Contains(t, issues[0].Path, "session.scope")
}

func TestValidate_ValidSessionScopes(t *testing.T) {
	for _, scope := range []string{"per-sender", "global", ""} {
		cfg := Defaults()
		cfg.Session.Scope = scope
		assert.Empty(t, Validate(&cfg), "scope %q should be valid", scope)
	}
}

func TestValidate_IRCMissingServer(t *testing.T) {
	cfg := Defaults()
	cfg.Channels.IRC = &IRCConfig{Nick: "bot"}
	issues := Validate(&cfg)
	assert.NotEmpty(t, issues)

	found := false
	for _, issue := range issues {
		if issue.Path == "channels.irc.server" {
			found = true
			break
		}
	}
	assert.True(t, found, "should report missing IRC server")
}

func TestValidate_IRCMissingNick(t *testing.T) {
	cfg := Defaults()
	cfg.Channels.IRC = &IRCConfig{Server: "irc.example.com"}
	issues := Validate(&cfg)

	found := false
	for _, issue := range issues {
		if issue.Path == "channels.irc.nick" {
			found = true
			break
		}
	}
	assert.True(t, found, "should report missing IRC nick")
}

func TestValidate_IRCInvalidPort(t *testing.T) {
	cfg := Defaults()
	cfg.Channels.IRC = &IRCConfig{
		Server: "irc.example.com",
		Nick:   "bot",
		Port:   70000,
	}
	issues := Validate(&cfg)

	found := false
	for _, issue := range issues {
		if issue.Path == "channels.irc.port" {
			found = true
			break
		}
	}
	assert.True(t, found, "should report invalid IRC port")
}

func TestValidate_IRCValidConfig(t *testing.T) {
	cfg := Defaults()
	cfg.Channels.IRC = &IRCConfig{
		Server: "irc.example.com",
		Nick:   "bot",
		Port:   6667,
	}
	issues := Validate(&cfg)
	assert.Empty(t, issues)
}

func TestValidate_MultipleIssues(t *testing.T) {
	cfg := Defaults()
	cfg.Gateway.Port = -1
	cfg.Gateway.Mode = "invalid"
	cfg.Logging.Level = "verbose"

	issues := Validate(&cfg)
	assert.GreaterOrEqual(t, len(issues), 3)
}

func TestValidationIssueString(t *testing.T) {
	issue := ValidationIssue{
		Path:    "gateway.port",
		Message: "port must be 0-65535, got -1",
	}
	assert.Equal(t, "gateway.port: port must be 0-65535, got -1", issue.String())
}

func TestValidate_IRCSASLWithoutPassword(t *testing.T) {
	cfg := Defaults()
	cfg.Channels.IRC = &IRCConfig{
		Server: "irc.example.com",
		Nick:   "bot",
		SASL:   true,
	}
	issues := Validate(&cfg)

	found := false
	for _, issue := range issues {
		if issue.Path == "channels.irc.sasl" {
			found = true
			break
		}
	}
	assert.True(t, found, "should report SASL without password")
}

func TestValidate_IRCSASLWithPassword(t *testing.T) {
	cfg := Defaults()
	cfg.Channels.IRC = &IRCConfig{
		Server:   "irc.example.com",
		Nick:     "bot",
		SASL:     true,
		Password: "secret",
	}
	issues := Validate(&cfg)
	assert.Empty(t, issues)
}

func TestValidate_IRCServerPassword(t *testing.T) {
	cfg := Defaults()
	cfg.Channels.IRC = &IRCConfig{
		Server:   "irc.example.com",
		Nick:     "bot",
		Password: "secret",
	}
	issues := Validate(&cfg)
	assert.Empty(t, issues)
}

func TestValidate_NoIRCConfig(t *testing.T) {
	cfg := Defaults()
	cfg.Channels.IRC = nil
	issues := Validate(&cfg)
	assert.Empty(t, issues)
}

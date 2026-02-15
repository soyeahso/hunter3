package config

import (
	"fmt"
	"slices"
)

// ValidationIssue describes a problem with a config value.
type ValidationIssue struct {
	Path    string
	Message string
}

func (v ValidationIssue) String() string {
	return fmt.Sprintf("%s: %s", v.Path, v.Message)
}

// Validate checks a Config for issues. Returns nil if valid.
func Validate(cfg *Config) []ValidationIssue {
	var issues []ValidationIssue

	// Gateway validation
	if cfg.Gateway.Port < 0 || cfg.Gateway.Port > 65535 {
		issues = append(issues, ValidationIssue{
			Path:    "gateway.port",
			Message: fmt.Sprintf("port must be 0-65535, got %d", cfg.Gateway.Port),
		})
	}

	validModes := []string{"local", "remote"}
	if cfg.Gateway.Mode != "" && !slices.Contains(validModes, cfg.Gateway.Mode) {
		issues = append(issues, ValidationIssue{
			Path:    "gateway.mode",
			Message: fmt.Sprintf("must be one of %v, got %q", validModes, cfg.Gateway.Mode),
		})
	}

	validBinds := []string{"auto", "lan", "loopback", "custom", "tailnet"}
	if cfg.Gateway.Bind != "" && !slices.Contains(validBinds, cfg.Gateway.Bind) {
		issues = append(issues, ValidationIssue{
			Path:    "gateway.bind",
			Message: fmt.Sprintf("must be one of %v, got %q", validBinds, cfg.Gateway.Bind),
		})
	}

	validAuthModes := []string{"token", "password"}
	if cfg.Gateway.Auth.Mode != "" && !slices.Contains(validAuthModes, cfg.Gateway.Auth.Mode) {
		issues = append(issues, ValidationIssue{
			Path:    "gateway.auth.mode",
			Message: fmt.Sprintf("must be one of %v, got %q", validAuthModes, cfg.Gateway.Auth.Mode),
		})
	}

	// Logging validation
	validLogLevels := []string{"silent", "fatal", "error", "warn", "info", "debug", "trace"}
	if cfg.Logging.Level != "" && !slices.Contains(validLogLevels, cfg.Logging.Level) {
		issues = append(issues, ValidationIssue{
			Path:    "logging.level",
			Message: fmt.Sprintf("must be one of %v, got %q", validLogLevels, cfg.Logging.Level),
		})
	}
	if cfg.Logging.ConsoleLevel != "" && !slices.Contains(validLogLevels, cfg.Logging.ConsoleLevel) {
		issues = append(issues, ValidationIssue{
			Path:    "logging.consoleLevel",
			Message: fmt.Sprintf("must be one of %v, got %q", validLogLevels, cfg.Logging.ConsoleLevel),
		})
	}

	validConsoleStyles := []string{"pretty", "compact", "json"}
	if cfg.Logging.ConsoleStyle != "" && !slices.Contains(validConsoleStyles, cfg.Logging.ConsoleStyle) {
		issues = append(issues, ValidationIssue{
			Path:    "logging.consoleStyle",
			Message: fmt.Sprintf("must be one of %v, got %q", validConsoleStyles, cfg.Logging.ConsoleStyle),
		})
	}

	// Session validation
	validScopes := []string{"per-sender", "global"}
	if cfg.Session.Scope != "" && !slices.Contains(validScopes, cfg.Session.Scope) {
		issues = append(issues, ValidationIssue{
			Path:    "session.scope",
			Message: fmt.Sprintf("must be one of %v, got %q", validScopes, cfg.Session.Scope),
		})
	}

	// IRC validation (only if configured)
	if cfg.Channels.IRC != nil {
		irc := cfg.Channels.IRC
		if irc.Server == "" {
			issues = append(issues, ValidationIssue{
				Path:    "channels.irc.server",
				Message: "server is required",
			})
		}
		if irc.Nick == "" {
			issues = append(issues, ValidationIssue{
				Path:    "channels.irc.nick",
				Message: "nick is required",
			})
		}
		if irc.Port < 0 || irc.Port > 65535 {
			issues = append(issues, ValidationIssue{
				Path:    "channels.irc.port",
				Message: fmt.Sprintf("port must be 0-65535, got %d", irc.Port),
			})
		}
		if irc.SASL && irc.Password == "" {
			issues = append(issues, ValidationIssue{
				Path:    "channels.irc.sasl",
				Message: "SASL requires a password to be set",
			})
		}
	}

	// CLI/API validation
	validCLIModes := []string{"", "none", "claude", "copilot"}
	if cfg.CLI != "" && !slices.Contains(validCLIModes, cfg.CLI) {
		issues = append(issues, ValidationIssue{
			Path:    "cli",
			Message: fmt.Sprintf("must be one of %v, got %q", validCLIModes, cfg.CLI),
		})
	}

	// If cli: none, validate API config
	if cfg.CLI == "none" {
		if cfg.APIProvider == "" {
			issues = append(issues, ValidationIssue{
				Path:    "apiProvider",
				Message: "required when cli: none",
			})
		}

		validProviders := []string{"claude", "gemini", "ollama"}
		if cfg.APIProvider != "" && !slices.Contains(validProviders, cfg.APIProvider) {
			issues = append(issues, ValidationIssue{
				Path:    "apiProvider",
				Message: fmt.Sprintf("must be one of %v, got %q", validProviders, cfg.APIProvider),
			})
		}

		if cfg.APIProvider != "ollama" && cfg.APIKey == "" {
			issues = append(issues, ValidationIssue{
				Path:    "apiKey",
				Message: "required when cli: none (except for ollama)",
			})
		}

		if cfg.APIModel == "" {
			issues = append(issues, ValidationIssue{
				Path:    "apiModel",
				Message: "required when cli: none",
			})
		}
	}

	return issues
}

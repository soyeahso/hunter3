package config

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// envVarPattern matches ${VAR_NAME} patterns in strings.
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnvVars replaces ${VAR} patterns with environment variable values.
// Unset variables are left unchanged.
func expandEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1]
		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		return match
	})
}

// expandSensitiveFields processes environment variable references in
// credential fields so passwords and tokens can be stored as ${ENV_VAR}.
func expandSensitiveFields(cfg *Config) {
	cfg.Gateway.Auth.Token = expandEnvVars(cfg.Gateway.Auth.Token)
	cfg.Gateway.Auth.Password = expandEnvVars(cfg.Gateway.Auth.Password)
	if cfg.Channels.IRC != nil {
		cfg.Channels.IRC.Password = expandEnvVars(cfg.Channels.IRC.Password)
	}
	for name, provider := range cfg.Models.Providers {
		provider.APIKey = expandEnvVars(provider.APIKey)
		cfg.Models.Providers[name] = provider
	}
}

// Load reads the config file, applies environment overrides, and returns
// a merged Config. Missing files produce defaults only.
func Load(path string) (Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnvOverrides(&cfg)
			return cfg, nil
		}
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, &ConfigError{Message: "failed to parse config: " + err.Error()}
	}

	applyDefaults(&cfg)
	applyEnvOverrides(&cfg)
	expandSensitiveFields(&cfg)
	return cfg, nil
}

// LoadRaw reads the config file into a generic map for path-based access.
func LoadRaw(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, &ConfigError{Message: "failed to parse config: " + err.Error()}
	}
	return raw, nil
}

// SaveRaw writes a generic map back to a YAML config file.
func SaveRaw(path string, raw map[string]any) error {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// applyDefaults fills zero-value fields with sensible defaults.
func applyDefaults(cfg *Config) {
	if cfg.Gateway.Port == 0 {
		cfg.Gateway.Port = 18789
	}
	if cfg.Gateway.Mode == "" {
		cfg.Gateway.Mode = "local"
	}
	if cfg.Gateway.Bind == "" {
		cfg.Gateway.Bind = "loopback"
	}
	if cfg.Gateway.Auth.Mode == "" {
		cfg.Gateway.Auth.Mode = "token"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.ConsoleLevel == "" {
		cfg.Logging.ConsoleLevel = "info"
	}
	if cfg.Logging.ConsoleStyle == "" {
		cfg.Logging.ConsoleStyle = "pretty"
	}
	if cfg.Session.Scope == "" {
		cfg.Session.Scope = "per-sender"
	}
	if cfg.Session.IdleMinutes == 0 {
		cfg.Session.IdleMinutes = 30
	}
	if cfg.Session.Store == "" {
		cfg.Session.Store = "sqlite"
	}
}

// applyEnvOverrides reads HUNTER3_* environment variables and overrides config values.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("HUNTER3_GATEWAY_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Gateway.Port = port
		}
	}
	if v := os.Getenv("HUNTER3_GATEWAY_MODE"); v != "" {
		cfg.Gateway.Mode = v
	}
	if v := os.Getenv("HUNTER3_GATEWAY_BIND"); v != "" {
		cfg.Gateway.Bind = v
	}
	if v := os.Getenv("HUNTER3_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = strings.ToLower(v)
	}
}

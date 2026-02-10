package config

import "fmt"

// ConfigError represents a configuration error.
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config: %s", e.Message)
}

// Defaults returns a Config with sensible defaults applied.
func Defaults() Config {
	return Config{
		Gateway: GatewayConfig{
			Port: 18789,
			Mode: "local",
			Bind: "loopback",
			Auth: GatewayAuth{
				Mode: "token",
			},
		},
		Logging: LoggingConfig{
			Level:        "info",
			ConsoleLevel: "info",
			ConsoleStyle: "pretty",
		},
		Session: SessionConfig{
			Scope:       "per-sender",
			IdleMinutes: 30,
			Store:       "sqlite",
		},
		Memory: MemoryConfig{
			Enabled:    true,
			Store:      "sqlite",
			SearchMode: "fts",
		},
	}
}

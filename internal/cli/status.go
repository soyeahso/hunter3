package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/llm"
	"github.com/soyeahso/hunter3/internal/version"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Hunter3 status and configuration summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Hunter3 %s (commit %s)\n\n", version.Version, version.Commit)

			// Show paths
			fmt.Printf("Config:  %s\n", paths.Config)
			fmt.Printf("Data:    %s\n", paths.Data)
			fmt.Printf("Logs:    %s\n", paths.Logs)
			fmt.Println()

			// Load config
			cfg, err := config.Load(paths.Config)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("Config:  not found (using defaults)")
				} else {
					fmt.Printf("Config:  error loading: %v\n", err)
				}
				return nil
			}

			// Gateway config
			port := cfg.Gateway.Port
			if port == 0 {
				port = 18789
			}
			bind := cfg.Gateway.Bind
			if bind == "" {
				bind = "loopback"
			}
			fmt.Printf("Gateway: port=%d bind=%s auth=%s\n",
				port, bind, cfg.Gateway.Auth.Mode)

			// Session config
			store := cfg.Session.Store
			if store == "" {
				store = "memory"
			}
			scope := cfg.Session.Scope
			if scope == "" {
				scope = "per-sender"
			}
			fmt.Printf("Session: store=%s scope=%s\n", store, scope)

			// LLM providers
			registry := llm.NewRegistryFromConfig(cfg.Models, cfg.CLI, log)
			providers := registry.List()
			if len(providers) > 0 {
				fmt.Printf("LLM:     %s\n", strings.Join(providers, ", "))
			} else {
				fmt.Println("LLM:     (none detected)")
			}

			// Agents
			if len(cfg.Agents.List) > 0 {
				for _, a := range cfg.Agents.List {
					model := a.Model
					if model == "" {
						model = cfg.Agents.Defaults.Model
					}
					if model == "" && len(providers) > 0 {
						model = providers[0]
					}
					fmt.Printf("Agent:   id=%s name=%s model=%s\n", a.ID, a.Name, model)
				}
			} else {
				fmt.Println("Agent:   (default)")
			}

			// Channels
			if cfg.Channels.IRC != nil {
				irc := cfg.Channels.IRC
				fmt.Printf("IRC:     server=%s nick=%s channels=%s tls=%v\n",
					irc.Server, irc.Nick, strings.Join(irc.Channels, ","), irc.UseTLS)
			} else {
				fmt.Println("IRC:     (not configured)")
			}

			// Memory
			if cfg.Memory.Enabled {
				fmt.Printf("Memory:  store=%s search=%s\n", cfg.Memory.Store, cfg.Memory.SearchMode)
			}

			// Validation
			issues := config.Validate(&cfg)
			if len(issues) > 0 {
				fmt.Printf("\nValidation issues (%d):\n", len(issues))
				for _, issue := range issues {
					fmt.Printf("  - %s: %s\n", issue.Path, issue.Message)
				}
			}

			return nil
		},
	}

	return cmd
}

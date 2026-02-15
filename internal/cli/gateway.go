package cli

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"path/filepath"

	"github.com/soyeahso/hunter3/internal/agent"
	"github.com/soyeahso/hunter3/internal/channel"
	"github.com/soyeahso/hunter3/internal/channel/irc"
	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/gateway"
	"github.com/soyeahso/hunter3/internal/hooks"
	"github.com/soyeahso/hunter3/internal/llm"
	"github.com/soyeahso/hunter3/internal/plugin"
	"github.com/soyeahso/hunter3/internal/routing"
	"github.com/soyeahso/hunter3/internal/store"
	"github.com/spf13/cobra"
)

func newGatewayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Manage the Hunter3 gateway server",
	}

	cmd.AddCommand(newGatewayRunCmd())
	return cmd
}

func newGatewayRunCmd() *cobra.Command {
	var (
		port  int
		bind  string
		force bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the gateway server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(paths.Config)
			if err != nil {
				return err
			}

			if port != 0 {
				cfg.Gateway.Port = port
			}
			if bind != "" {
				cfg.Gateway.Bind = bind
			}

			issues := config.Validate(&cfg)
			if len(issues) > 0 {
				for _, issue := range issues {
					log.Error().Str("path", issue.Path).Msg(issue.Message)
				}
				return fmt.Errorf("config validation failed with %d issue(s)", len(issues))
			}

			// Load raw config for RPC access
			raw, err := config.LoadRaw(paths.Config)
			if err != nil {
				raw = make(map[string]any)
			}

			// Initialize hook manager
			hookMgr := hooks.NewManager(log)

			opts := []gateway.ServerOption{
				gateway.WithConfigRaw(raw),
				gateway.WithHooks(hookMgr),
			}

			// Initialize LLM provider registry and agent runner
			registry := llm.NewRegistryFromConfig(cfg.Models, cfg.CLI, cfg.APIProvider, cfg.APIKey, cfg.APIModel, cfg.APIEndpoint, log)
			providers := registry.List()

			// Initialize session store (SQLite or in-memory)
			var sessions agent.SessionStore
			var db *store.DB
			if cfg.Session.Store == "sqlite" {
				dbPath := filepath.Join(paths.Data, "hunter3.db")
				var err error
				db, err = store.Open(dbPath, log)
				if err != nil {
					return fmt.Errorf("opening database: %w", err)
				}
				defer db.Close()
				sessions = store.NewSQLiteSessionStore(db)
				log.Info().Str("path", dbPath).Msg("using SQLite session store")
			} else {
				sessions = agent.NewMemorySessionStore()
				log.Info().Msg("using in-memory session store")
			}

			// Initialize tool registry
			toolReg := agent.NewToolRegistry()

			// Block until SIGINT/SIGTERM
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			// Initialize plugin registry
			pluginReg := plugin.NewRegistry(hookMgr, log)
			if err := pluginReg.InitAll(ctx); err != nil {
				return fmt.Errorf("initializing plugins: %w", err)
			}
			defer pluginReg.CloseAll()

			var runner *agent.Runner
			if len(providers) > 0 {
				log.Info().Strs("providers", providers).Msg("LLM providers available")

				model := cfg.Agents.Defaults.Model
				if model == "" {
					model = providers[0]
				}
				agentName := "Hunter3"
				agentID := "default"
				if len(cfg.Agents.List) > 0 {
					agentName = cfg.Agents.List[0].Name
					agentID = cfg.Agents.List[0].ID
					if cfg.Agents.List[0].Model != "" {
						model = cfg.Agents.List[0].Model
					}
				}
				if agentName == "" {
					agentName = "Hunter3"
				}

				runner = agent.NewRunner(
					agent.RunnerConfig{
						AgentID:     agentID,
						AgentName:   agentName,
						Model:       model,
						MaxTokens:   cfg.Agents.Defaults.MaxTokens,
						Temperature: cfg.Agents.Defaults.Temperature,
				},
					registry,
					sessions,
					toolReg,
					log,
				)
				opts = append(opts, gateway.WithRunner(runner))
			} else {
				log.Warn().Msg("no LLM providers found — chat.send will be unavailable")
			}

			// Initialize channel registry
			channels := channel.NewRegistry(log)

			// Register IRC channel if configured
			if cfg.Channels.IRC != nil {
				ircCh := irc.New(*cfg.Channels.IRC, log)
				channels.Register(ircCh)
			}

			opts = append(opts, gateway.WithChannels(channels))

			srv := gateway.New(cfg, log, opts...)

			_ = force // TODO: check for existing instance

			// Start channels and wire message routing
			if channels.Count() > 0 {
				if err := channels.StartAll(ctx); err != nil {
					return fmt.Errorf("starting channels: %w", err)
				}
				defer channels.StopAll(ctx)

				if runner != nil {
					scope := cfg.Session.Scope
					// Get IRC owner for completion messages
					ircOwner := ""
					if cfg.Channels.IRC != nil && cfg.Channels.IRC.Owner != nil {
						ircOwner = *cfg.Channels.IRC.Owner
					}
					router := routing.NewRouter(channels, runner, scope, ircOwner, log)
					if cfg.Channels.IRC != nil && cfg.Channels.IRC.Stream {
						router.WireStream()
						log.Info().Msg("streaming mode enabled for IRC")
					} else {
						router.Wire()
					}
					log.Info().
						Int("channels", channels.Count()).
						Str("scope", scope).
						Msg("message routing active")
				} else {
					log.Warn().Msg("channels started but no LLM provider — messages will not be processed")
				}
			}

			return srv.Start(ctx)
		},
	}

	cmd.Flags().IntVar(&port, "port", 0, "override gateway port")
	cmd.Flags().StringVar(&bind, "bind", "", "override bind mode (auto, lan, loopback, custom, tailnet)")
	cmd.Flags().BoolVar(&force, "force", false, "force start even if another instance is running")

	return cmd
}

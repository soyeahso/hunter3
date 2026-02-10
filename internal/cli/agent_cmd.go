package cli

import (
	"fmt"

	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/llm"
	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agents",
	}

	cmd.AddCommand(newAgentListCmd())
	cmd.AddCommand(newAgentInfoCmd())
	return cmd
}

func newAgentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(paths.Config)
			if err != nil {
				cfg = config.Defaults()
			}

			registry := llm.NewRegistryFromConfig(cfg.Models, cfg.CLI, log)
			providers := registry.List()

			if len(cfg.Agents.List) == 0 {
				defaultModel := cfg.Agents.Defaults.Model
				if defaultModel == "" && len(providers) > 0 {
					defaultModel = providers[0]
				}
				fmt.Printf("  default  Hunter3  model=%s\n", defaultModel)
				return nil
			}

			for _, a := range cfg.Agents.List {
				model := a.Model
				if model == "" {
					model = cfg.Agents.Defaults.Model
				}
				if model == "" && len(providers) > 0 {
					model = providers[0]
				}
				def := ""
				if a.Default {
					def = " (default)"
				}
				fmt.Printf("  %-12s %-16s model=%s%s\n", a.ID, a.Name, model, def)
			}

			return nil
		},
	}
}

func newAgentInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info [agent-id]",
		Short: "Show details about an agent",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(paths.Config)
			if err != nil {
				cfg = config.Defaults()
			}

			registry := llm.NewRegistryFromConfig(cfg.Models, cfg.CLI, log)
			providers := registry.List()

			targetID := ""
			if len(args) > 0 {
				targetID = args[0]
			}

			if len(cfg.Agents.List) == 0 {
				model := cfg.Agents.Defaults.Model
				if model == "" && len(providers) > 0 {
					model = providers[0]
				}
				fmt.Println("Agent: default (Hunter3)")
				fmt.Printf("  Model:     %s\n", model)
				fmt.Printf("  MaxTokens: %d\n", cfg.Agents.Defaults.MaxTokens)
				if cfg.Agents.Defaults.Temperature != nil {
					fmt.Printf("  Temp:      %.2f\n", *cfg.Agents.Defaults.Temperature)
				}
				fmt.Printf("  Providers: %v\n", providers)
				return nil
			}

			for _, a := range cfg.Agents.List {
				if targetID != "" && a.ID != targetID {
					continue
				}

				model := a.Model
				if model == "" {
					model = cfg.Agents.Defaults.Model
				}
				if model == "" && len(providers) > 0 {
					model = providers[0]
				}

				fmt.Printf("Agent: %s (%s)\n", a.ID, a.Name)
				fmt.Printf("  Model:     %s\n", model)
				fmt.Printf("  MaxTokens: %d\n", cfg.Agents.Defaults.MaxTokens)
				if cfg.Agents.Defaults.Temperature != nil {
					fmt.Printf("  Temp:      %.2f\n", *cfg.Agents.Defaults.Temperature)
				}
				if a.Workspace != "" {
					fmt.Printf("  Workspace: %s\n", a.Workspace)
				}
				fmt.Printf("  Providers: %v\n", providers)

				if targetID != "" {
					return nil
				}
				fmt.Println()
			}

			if targetID != "" {
				return fmt.Errorf("agent not found: %s", targetID)
			}

			return nil
		},
	}
}

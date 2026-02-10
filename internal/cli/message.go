package cli

import (
	"context"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/soyeahso/hunter3/internal/agent"
	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/llm"
	"github.com/spf13/cobra"
)

func newMessageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Send and manage messages",
	}

	cmd.AddCommand(newMessageSendCmd())
	return cmd
}

func newMessageSendCmd() *cobra.Command {
	var (
		model     string
		agentID   string
		stream    bool
	)

	cmd := &cobra.Command{
		Use:   "send [message]",
		Short: "Send a message to the agent and print the response",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			message := strings.Join(args, " ")

			cfg, err := config.Load(paths.Config)
			if err != nil {
				cfg = config.Defaults()
			}

			registry := llm.NewRegistryFromConfig(cfg.Models, cfg.CLI, log)
			providers := registry.List()
			if len(providers) == 0 {
				return fmt.Errorf("no LLM providers available")
			}

			if model == "" {
				model = cfg.Agents.Defaults.Model
				if model == "" {
					model = providers[0]
				}
			}

			if agentID == "" {
				agentID = "default"
				if len(cfg.Agents.List) > 0 {
					agentID = cfg.Agents.List[0].ID
				}
			}

			agentName := "Hunter3"
			if len(cfg.Agents.List) > 0 {
				agentName = cfg.Agents.List[0].Name
				if agentName == "" {
					agentName = "Hunter3"
				}
			}

			runner := agent.NewRunner(
				agent.RunnerConfig{
					AgentID:     agentID,
					AgentName:   agentName,
					Model:       model,
					MaxTokens:   cfg.Agents.Defaults.MaxTokens,
					Temperature: cfg.Agents.Defaults.Temperature,
				},
				registry,
				agent.NewMemorySessionStore(),
				agent.NewToolRegistry(),
				log,
			)

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			msg := domain.InboundMessage{
				ID:        "cli-msg",
				ChannelID: "cli",
				From:      "user",
				FromName:  "User",
				ChatID:    "cli",
				ChatType:  domain.ChatTypeDM,
				Body:      message,
				Timestamp: time.Now(),
			}

			if stream {
				result, err := runner.RunStream(ctx, msg, func(evt llm.StreamEvent) {
					if evt.Type == "delta" {
						fmt.Print(evt.Content)
					}
				})
				if err != nil {
					return err
				}
				fmt.Println()
				if result.Model != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "\n[model=%s tokens=%d+%d]\n",
						result.Model, result.Usage.InputTokens, result.Usage.OutputTokens)
				}
			} else {
				result, err := runner.Run(ctx, msg)
				if err != nil {
					return err
				}
				fmt.Println(result.Response)
				if result.Model != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "\n[model=%s tokens=%d+%d]\n",
						result.Model, result.Usage.InputTokens, result.Usage.OutputTokens)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "LLM model to use")
	cmd.Flags().StringVar(&agentID, "agent", "", "agent ID to use")
	cmd.Flags().BoolVar(&stream, "stream", false, "stream the response")

	return cmd
}

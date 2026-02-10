package cli

import (
	"github.com/soyeahso/hunter3/internal/config"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/spf13/cobra"
)

var (
	cfgFile  string
	logLevel string

	// loaded at init time
	paths config.Paths
	log   *logging.Logger
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hunter3",
		Short: "Hunter3 — personal AI assistant platform",
		Long:  "Hunter3 is a personal AI assistant that connects to messaging channels and uses LLMs to help you.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			paths, err = config.ResolvePaths()
			if err != nil {
				return err
			}
			if cfgFile != "" {
				paths.Config = cfgFile
			}
			level := logLevel
			if level == "" {
				level = "info"
			}
			log = logging.New(nil, level)
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.hunter3/config.yaml)")
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level (trace, debug, info, warn, error, fatal, silent)")

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newGatewayCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newMessageCmd())
	cmd.AddCommand(newAgentCmd())

	return cmd
}

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}

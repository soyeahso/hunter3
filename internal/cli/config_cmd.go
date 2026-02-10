package cli

import (
	"fmt"
	"strings"

	"github.com/soyeahso/hunter3/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Get or set configuration values",
	}

	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigUnsetCmd())
	cmd.AddCommand(newConfigPathCmd())

	return cmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ParseConfigPath(args[0])
			if err != nil {
				return err
			}

			raw, err := config.LoadRaw(paths.Config)
			if err != nil {
				return err
			}

			val, ok := config.GetValueAtPath(raw, path)
			if !ok {
				return fmt.Errorf("key %q not found", args[0])
			}

			return printValue(val)
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ParseConfigPath(args[0])
			if err != nil {
				return err
			}

			raw, err := config.LoadRaw(paths.Config)
			if err != nil {
				return err
			}

			value := parseValue(args[1])
			config.SetValueAtPath(raw, path, value)

			if err := config.SaveRaw(paths.Config, raw); err != nil {
				return err
			}

			fmt.Printf("Set %s = %v\n", args[0], value)
			return nil
		},
	}
}

func newConfigUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ParseConfigPath(args[0])
			if err != nil {
				return err
			}

			raw, err := config.LoadRaw(paths.Config)
			if err != nil {
				return err
			}

			if !config.UnsetValueAtPath(raw, path) {
				return fmt.Errorf("key %q not found", args[0])
			}

			if err := config.SaveRaw(paths.Config, raw); err != nil {
				return err
			}

			fmt.Printf("Unset %s\n", args[0])
			return nil
		},
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(paths.Config)
		},
	}
}

// printValue outputs a value in a human-readable format.
func printValue(v any) error {
	switch val := v.(type) {
	case string:
		fmt.Println(val)
	case map[string]any:
		data, err := yaml.Marshal(val)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
	case []any:
		data, err := yaml.Marshal(val)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
	default:
		fmt.Println(val)
	}
	return nil
}

// parseValue attempts to interpret a string as a typed value.
func parseValue(s string) any {
	lower := strings.ToLower(s)
	if lower == "true" {
		return true
	}
	if lower == "false" {
		return false
	}

	// Try integer
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil && fmt.Sprintf("%d", n) == s {
		return n
	}

	// Try float
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f
	}

	return s
}

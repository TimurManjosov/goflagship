package commands

import (
	"context"
	"fmt"

	"github.com/TimurManjosov/goflagship/internal/cli"
	"github.com/TimurManjosov/goflagship/internal/client"
	"github.com/TimurManjosov/goflagship/internal/store"
	"github.com/spf13/cobra"
)

var (
	listEnabledOnly bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all feature flags",
	Long: `List all feature flags in the specified environment.

Examples:
  flagship list --env prod
  flagship list --env prod --format json
  flagship list --env prod --enabled-only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get environment configuration
		envCfg, effectiveEnv, err := cli.GetEnvConfig(env, baseURL, apiKey)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		// Create API client
		c := client.NewClient(envCfg.BaseURL, envCfg.APIKey)

		// List flags
		ctx := context.Background()
		flags, err := c.ListFlags(ctx, effectiveEnv)
		if err != nil {
			return fmt.Errorf("failed to list flags: %w", err)
		}

		// Filter enabled only if requested
		if listEnabledOnly {
			var enabled []store.Flag
			for _, f := range flags {
				if f.Enabled {
					enabled = append(enabled, f)
				}
			}
			flags = enabled
		}

		if !quiet {
			if len(flags) == 0 {
				fmt.Println("No flags found")
				return nil
			}
			return cli.PrintFlags(flags, cli.OutputFormat(format))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolVar(&listEnabledOnly, "enabled-only", false, "Show only enabled flags")
}

package commands

import (
	"context"
	"fmt"

	"github.com/TimurManjosov/goflagship/internal/cli"
	"github.com/TimurManjosov/goflagship/internal/client"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a feature flag",
	Long: `Get details of a specific feature flag.

Examples:
  flagship get feature_x --env prod
  flagship get feature_x --env prod --format json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		// Get environment configuration
		envCfg, err := cli.GetEnvConfig(env, baseURL, apiKey)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		// Create API client
		c := client.NewClient(envCfg.BaseURL, envCfg.APIKey)

		// Get flag
		ctx := context.Background()
		flag, err := c.GetFlag(ctx, key, env)
		if err != nil {
			return fmt.Errorf("failed to get flag: %w", err)
		}

		if !quiet {
			return cli.PrintFlag(flag, cli.OutputFormat(format))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}

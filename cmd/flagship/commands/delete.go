package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/TimurManjosov/goflagship/internal/cli"
	"github.com/TimurManjosov/goflagship/internal/client"
	"github.com/spf13/cobra"
)

var (
	deleteForce bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a feature flag",
	Long: `Delete a feature flag from the specified environment.

Examples:
  flagship delete feature_x --env prod
  flagship delete feature_x --env prod --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		// Get environment configuration
		envCfg, err := cli.GetEnvConfig(env, baseURL, apiKey)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		// Confirm deletion unless --force
		if !deleteForce && !quiet {
			fmt.Printf("Are you sure you want to delete flag '%s' from environment '%s'? (y/N): ", key, env)
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Deletion cancelled")
				return nil
			}
		}

		// Create API client
		c := client.NewClient(envCfg.BaseURL, envCfg.APIKey)

		// Delete flag
		ctx := context.Background()
		if err := c.DeleteFlag(ctx, key, env); err != nil {
			return fmt.Errorf("failed to delete flag: %w", err)
		}

		if !quiet {
			fmt.Printf("Successfully deleted flag '%s' from environment '%s'\n", key, env)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	deleteCmd.Flags().BoolVar(&deleteForce, "force", false, "Skip confirmation prompt")
}

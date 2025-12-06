package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TimurManjosov/goflagship/internal/cli"
	"github.com/TimurManjosov/goflagship/internal/client"
	"github.com/TimurManjosov/goflagship/internal/store"
	"github.com/spf13/cobra"
)

var (
	createEnabled     bool
	createRollout     int32
	createConfig      string
	createDescription string
)

var createCmd = &cobra.Command{
	Use:   "create <key>",
	Short: "Create a new feature flag",
	Long: `Create a new feature flag with the specified key and options.

Examples:
  flagship create feature_x --enabled --rollout 50 --env prod
  flagship create feature_y --config '{"color":"blue"}' --description "New feature Y"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		// Get environment configuration
		envCfg, effectiveEnv, err := cli.GetEnvConfig(env, baseURL, apiKey)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		// Parse config JSON if provided
		var config map[string]any
		if createConfig != "" {
			if err := json.Unmarshal([]byte(createConfig), &config); err != nil {
				return fmt.Errorf("invalid config JSON: %w", err)
			}
		}

		// Create API client
		c := client.NewClient(envCfg.BaseURL, envCfg.APIKey)

		// Create flag
		params := store.UpsertParams{
			Key:         key,
			Description: createDescription,
			Enabled:     createEnabled,
			Rollout:     createRollout,
			Config:      config,
			Env:         effectiveEnv,
		}

		ctx := context.Background()
		if err := c.CreateFlag(ctx, params); err != nil {
			return fmt.Errorf("failed to create flag: %w", err)
		}

		if !quiet {
			fmt.Printf("Successfully created flag '%s' in environment '%s'\n", key, effectiveEnv)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().BoolVar(&createEnabled, "enabled", false, "Enable the flag")
	createCmd.Flags().Int32Var(&createRollout, "rollout", 100, "Rollout percentage (0-100)")
	createCmd.Flags().StringVar(&createConfig, "config", "", "Flag configuration as JSON")
	createCmd.Flags().StringVar(&createDescription, "description", "", "Flag description")
}

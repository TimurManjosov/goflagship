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
	updateEnabled     *bool
	updateRollout     *int32
	updateConfig      string
	updateDescription string
)

var updateCmd = &cobra.Command{
	Use:   "update <key>",
	Short: "Update a feature flag",
	Long: `Update an existing feature flag.

Examples:
  flagship update feature_x --enabled=false --env prod
  flagship update feature_x --rollout 75 --env prod
  flagship update feature_x --config '{"color":"red"}' --env prod`,
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

		// First, get the existing flag to preserve values
		ctx := context.Background()
		existingFlag, err := c.GetFlag(ctx, key, env)
		if err != nil {
			return fmt.Errorf("failed to get existing flag: %w", err)
		}

		// Build update params, starting with existing values
		params := store.UpsertParams{
			Key:         key,
			Description: existingFlag.Description,
			Enabled:     existingFlag.Enabled,
			Rollout:     existingFlag.Rollout,
			Config:      existingFlag.Config,
			Variants:    existingFlag.Variants,
			Expression:  existingFlag.Expression,
			Env:         env,
		}

		// Update with new values if provided
		if updateEnabled != nil {
			params.Enabled = *updateEnabled
		}
		if updateRollout != nil {
			params.Rollout = *updateRollout
		}
		if updateDescription != "" {
			params.Description = updateDescription
		}
		if updateConfig != "" {
			var config map[string]any
			if err := json.Unmarshal([]byte(updateConfig), &config); err != nil {
				return fmt.Errorf("invalid config JSON: %w", err)
			}
			params.Config = config
		}

		// Update flag
		if err := c.UpdateFlag(ctx, params); err != nil {
			return fmt.Errorf("failed to update flag: %w", err)
		}

		if !quiet {
			fmt.Printf("Successfully updated flag '%s' in environment '%s'\n", key, env)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	// Use pointers to distinguish between "not set" and "set to false/0"
	updateCmd.Flags().BoolVar(new(bool), "enabled", false, "Enable/disable the flag")
	updateCmd.Flags().Int32Var(new(int32), "rollout", 0, "Rollout percentage (0-100)")
	updateCmd.Flags().StringVar(&updateConfig, "config", "", "Flag configuration as JSON")
	updateCmd.Flags().StringVar(&updateDescription, "description", "", "Flag description")

	// Bind the pointers
	updateEnabled = new(bool)
	updateRollout = new(int32)
	
	// Mark flags as changed only if explicitly set
	updateCmd.Flags().Lookup("enabled").NoOptDefVal = "true"
}

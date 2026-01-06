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

The flag will be created in the specified environment with the given configuration.
By default, flags are created disabled with 100% rollout.

Examples:
  # Create an enabled flag with 50% rollout
  flagship create feature_x --enabled --rollout 50 --env prod

  # Create a flag with custom configuration
  flagship create feature_y --config '{"color":"blue","size":"large"}' --description "New feature Y"

  # Create a disabled flag (default)
  flagship create feature_z --env staging`,
	Args: cobra.ExactArgs(1),
	RunE: runCreateCommand,
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().BoolVar(&createEnabled, "enabled", false, "Enable the flag immediately")
	createCmd.Flags().Int32Var(&createRollout, "rollout", 100, "Rollout percentage (0-100)")
	createCmd.Flags().StringVar(&createConfig, "config", "", "Flag configuration as JSON string")
	createCmd.Flags().StringVar(&createDescription, "description", "", "Human-readable flag description")
}

// runCreateCommand executes the create flag command.
func runCreateCommand(cmd *cobra.Command, args []string) error {
	flagKey := args[0]

	// Get environment configuration (from file, env vars, or flags)
	envConfig, effectiveEnv, err := cli.GetEnvConfig(env, baseURL, apiKey)
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Parse and validate config JSON if provided
	parsedConfig, err := parseConfigJSON(createConfig)
	if err != nil {
		return err
	}

	// Validate rollout percentage
	if err := validateRolloutPercentage(createRollout); err != nil {
		return err
	}

	// Create API client
	apiClient := client.NewClient(envConfig.BaseURL, envConfig.APIKey)

	// Prepare flag creation parameters
	params := store.UpsertParams{
		Key:         flagKey,
		Description: createDescription,
		Enabled:     createEnabled,
		Rollout:     createRollout,
		Config:      parsedConfig,
		Env:         effectiveEnv,
	}

	// Create the flag via API
	ctx := context.Background()
	if err := apiClient.CreateFlag(ctx, params); err != nil {
		return fmt.Errorf("failed to create flag '%s': %w", flagKey, err)
	}

	// Print success message (unless in quiet mode)
	if !quiet {
		printSuccessMessage(flagKey, effectiveEnv, createEnabled, createRollout)
	}

	return nil
}

// parseConfigJSON parses and validates a JSON config string.
// Returns nil if the config string is empty.
func parseConfigJSON(configStr string) (map[string]any, error) {
	if configStr == "" {
		return nil, nil
	}

	var config map[string]any
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, fmt.Errorf("invalid config JSON: %w\nProvided: %s", err, configStr)
	}

	return config, nil
}

// validateRolloutPercentage checks if the rollout value is within the valid range.
func validateRolloutPercentage(rollout int32) error {
	if rollout < 0 || rollout > 100 {
		return fmt.Errorf("rollout percentage must be between 0 and 100, got: %d", rollout)
	}
	return nil
}

// printSuccessMessage prints a formatted success message after flag creation.
func printSuccessMessage(flagKey, environment string, enabled bool, rollout int32) {
	status := "disabled"
	if enabled {
		status = "enabled"
	}
	
	fmt.Printf("✓ Successfully created flag '%s' in environment '%s'\n", flagKey, environment)
	fmt.Printf("  Status: %s\n", status)
	fmt.Printf("  Rollout: %d%%\n", rollout)
}

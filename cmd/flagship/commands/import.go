package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/TimurManjosov/goflagship/internal/cli"
	"github.com/TimurManjosov/goflagship/internal/client"
	"github.com/TimurManjosov/goflagship/internal/store"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	importDryRun bool
	importForce  bool
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import flags from a file",
	Long: `Import flags from a YAML or JSON file.

Examples:
  flagship import flags.yaml --env prod
  flagship import flags.yaml --env staging --dry-run
  flagship import flags.yaml --env prod --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]

		// Read file
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Parse file
		var importData ExportFormat
		if err := yaml.Unmarshal(data, &importData); err != nil {
			return fmt.Errorf("failed to parse file: %w", err)
		}

		// Validate flags
		if len(importData.Flags) == 0 {
			return fmt.Errorf("no flags found in file")
		}

		if verbose {
			fmt.Printf("Found %d flag(s) to import\n", len(importData.Flags))
		}

		// Dry run mode - just validate and show what would be imported
		if importDryRun {
			fmt.Println("Dry run mode - the following flags would be imported:")
			for _, flag := range importData.Flags {
				fmt.Printf("  - %s (enabled: %v, rollout: %d%%, env: %s)\n",
					flag.Key, flag.Enabled, flag.Rollout, flag.Env)
			}
			return nil
		}

		// Get environment configuration
		envCfg, effectiveEnv, err := cli.GetEnvConfig(env, baseURL, apiKey)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		// Create API client
		c := client.NewClient(envCfg.BaseURL, envCfg.APIKey)
		ctx := context.Background()

		// Import flags
		successCount := 0
		errorCount := 0

		for _, flag := range importData.Flags {
			// Use the environment from the flag or override with --env flag
			targetEnv := flag.Env
			if effectiveEnv != "" {
				targetEnv = effectiveEnv
			}

			params := store.UpsertParams{
				Key:         flag.Key,
				Description: flag.Description,
				Enabled:     flag.Enabled,
				Rollout:     flag.Rollout,
				Expression:  flag.Expression,
				Config:      flag.Config,
				Variants:    flag.Variants,
				Env:         targetEnv,
			}

			if verbose {
				fmt.Printf("Importing flag: %s\n", flag.Key)
			}

			if err := c.CreateFlag(ctx, params); err != nil {
				errorCount++
				fmt.Fprintf(os.Stderr, "Failed to import flag '%s': %v\n", flag.Key, err)
				if !importForce {
					return fmt.Errorf("import failed, use --force to continue on errors")
				}
			} else {
				successCount++
			}
		}

		if !quiet {
			fmt.Printf("Import complete: %d succeeded, %d failed\n", successCount, errorCount)
		}

		if errorCount > 0 && !importForce {
			return fmt.Errorf("import completed with errors")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "Validate without importing")
	importCmd.Flags().BoolVar(&importForce, "force", false, "Continue on errors")
}

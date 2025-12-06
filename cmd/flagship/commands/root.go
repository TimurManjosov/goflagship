package commands

import (
	"github.com/spf13/cobra"
)

var (
	// Global flags
	baseURL string
	apiKey  string
	env     string
	format  string
	quiet   bool
	verbose bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "flagship",
	Short: "CLI tool for managing feature flags",
	Long: `Flagship is a command-line tool for managing feature flags in the go-flagship service.

It provides commands for creating, reading, updating, and deleting flags,
as well as importing and exporting flag configurations.

Examples:
  flagship list --env prod
  flagship create my_flag --enabled --env prod
  flagship get my_flag --env prod
  flagship export --env prod --output flags.yaml
  flagship import flags.yaml --env staging`,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags available to all commands
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "Base URL of the flagship API")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	rootCmd.PersistentFlags().StringVar(&env, "env", "", "Environment (dev, staging, prod)")
	rootCmd.PersistentFlags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress output")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Verbose output")
}

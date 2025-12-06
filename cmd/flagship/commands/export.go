package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/TimurManjosov/goflagship/internal/cli"
	"github.com/TimurManjosov/goflagship/internal/client"
	"github.com/TimurManjosov/goflagship/internal/store"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	exportOutput string
)

// ExportFormat represents the structure for exporting flags
type ExportFormat struct {
	Flags []store.Flag `yaml:"flags" json:"flags"`
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export flags to a file",
	Long: `Export all flags from the specified environment to a YAML or JSON file.

Examples:
  flagship export --env prod --output flags.yaml
  flagship export --env prod --output flags.json --format json
  flagship export --env prod > backup.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get environment configuration
		envCfg, err := cli.GetEnvConfig(env, baseURL, apiKey)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		// Create API client
		c := client.NewClient(envCfg.BaseURL, envCfg.APIKey)

		// List flags
		ctx := context.Background()
		flags, err := c.ListFlags(ctx, env)
		if err != nil {
			return fmt.Errorf("failed to list flags: %w", err)
		}

		exportData := ExportFormat{Flags: flags}

		// Determine output destination
		var output *os.File
		if exportOutput == "" || exportOutput == "-" {
			output = os.Stdout
		} else {
			output, err = os.Create(exportOutput)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer output.Close()
		}

		// Export based on format
		switch format {
		case "json":
			encoder := json.NewEncoder(output)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(exportData); err != nil {
				return fmt.Errorf("failed to encode JSON: %w", err)
			}
		case "yaml", "table":
			// Default to YAML for export
			encoder := yaml.NewEncoder(output)
			defer encoder.Close()
			encoder.SetIndent(2)
			if err := encoder.Encode(exportData); err != nil {
				return fmt.Errorf("failed to encode YAML: %w", err)
			}
		default:
			return fmt.Errorf("unsupported export format: %s", format)
		}

		if exportOutput != "" && exportOutput != "-" && !quiet {
			fmt.Fprintf(os.Stderr, "Successfully exported %d flag(s) to %s\n", len(flags), exportOutput)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")
}

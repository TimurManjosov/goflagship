package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/TimurManjosov/goflagship/internal/store"
	"github.com/olekukonko/tablewriter"
	"gopkg.in/yaml.v3"
)

// OutputFormat specifies the output format for CLI commands
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
)

// PrintFlags outputs flags in the specified format
func PrintFlags(flags []store.Flag, format OutputFormat) error {
	switch format {
	case FormatJSON:
		return printJSON(flags)
	case FormatYAML:
		return printYAML(flags)
	case FormatTable:
		return printTable(flags)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// PrintFlag outputs a single flag in the specified format
func PrintFlag(flag *store.Flag, format OutputFormat) error {
	switch format {
	case FormatJSON:
		return printJSON(flag)
	case FormatYAML:
		return printYAML(flag)
	case FormatTable:
		return printTable([]store.Flag{*flag})
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func printJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	// Wrap slices of store.Flag in a "flags" key for consistency with documentation
	if flags, ok := data.([]store.Flag); ok {
		return encoder.Encode(map[string][]store.Flag{"flags": flags})
	}
	return encoder.Encode(data)
}

func printYAML(data interface{}) error {
	encoder := yaml.NewEncoder(os.Stdout)
	defer encoder.Close()
	encoder.SetIndent(2)
	return encoder.Encode(data)
}

func printTable(flags []store.Flag) error {
	table := tablewriter.NewWriter(os.Stdout)
	
	// Set headers
	table.Header("Key", "Enabled", "Rollout", "Env", "Description", "Updated At")

	// Add rows
	for _, flag := range flags {
		enabled := "false"
		if flag.Enabled {
			enabled = "true"
		}

		description := flag.Description
		if len(description) > 40 {
			description = description[:37] + "..."
		}

		table.Append(
			flag.Key,
			enabled,
			fmt.Sprintf("%d%%", flag.Rollout),
			flag.Env,
			description,
			flag.UpdatedAt.Format("2006-01-02 15:04"),
		)
	}

	return table.Render()
}

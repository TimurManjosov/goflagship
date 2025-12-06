package commands

import (
	"fmt"
	"strings"

	"github.com/TimurManjosov/goflagship/internal/cli"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `Manage flagship CLI configuration file.`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Long: `Create a default configuration file at ~/.flagship/config.yaml

Example:
  flagship config init`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cli.InitConfig(); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		configPath, _ := cli.GetConfigPath()
		fmt.Printf("Configuration file created at: %s\n", configPath)
		fmt.Println("\nPlease edit the file to set your API keys and base URLs.")
		fmt.Println("Example:")
		fmt.Println("  vi ~/.flagship/config.yaml")

		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration",
	Long: `Display the current configuration.

Example:
  flagship config list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cli.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fmt.Printf("Default Environment: %s\n\n", cfg.DefaultEnv)
		fmt.Println("Environments:")
		for name, envCfg := range cfg.Environments {
			fmt.Printf("  %s:\n", name)
			fmt.Printf("    base_url: %s\n", envCfg.BaseURL)
			// Mask API key for security
			maskedKey := "***"
			if len(envCfg.APIKey) > 4 {
				maskedKey = envCfg.APIKey[:4] + "***"
			}
			fmt.Printf("    api_key: %s\n", maskedKey)
		}

		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <env.key>",
	Short: "Get a configuration value",
	Long: `Get a specific configuration value.

Examples:
  flagship config get dev.base_url
  flagship config get prod.api_key`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cli.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		parts := strings.Split(args[0], ".")
		if len(parts) != 2 {
			return fmt.Errorf("invalid key format, expected 'env.key' (e.g., 'dev.base_url')")
		}

		envName := parts[0]
		key := parts[1]

		envCfg, ok := cfg.Environments[envName]
		if !ok {
			return fmt.Errorf("environment '%s' not found", envName)
		}

		switch key {
		case "base_url":
			fmt.Println(envCfg.BaseURL)
		case "api_key":
			fmt.Println(envCfg.APIKey)
		default:
			return fmt.Errorf("unknown key '%s', valid keys: base_url, api_key", key)
		}

		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <env.key> <value>",
	Short: "Set a configuration value",
	Long: `Set a specific configuration value.

Examples:
  flagship config set dev.base_url http://localhost:8080
  flagship config set prod.api_key my-secret-key`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cli.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		parts := strings.Split(args[0], ".")
		if len(parts) != 2 {
			return fmt.Errorf("invalid key format, expected 'env.key' (e.g., 'dev.base_url')")
		}

		envName := parts[0]
		key := parts[1]
		value := args[1]

		// Create environment if it doesn't exist
		if cfg.Environments == nil {
			cfg.Environments = make(map[string]cli.EnvConfig)
		}

		envCfg := cfg.Environments[envName]

		switch key {
		case "base_url":
			envCfg.BaseURL = value
		case "api_key":
			envCfg.APIKey = value
		default:
			return fmt.Errorf("unknown key '%s', valid keys: base_url, api_key", key)
		}

		cfg.Environments[envName] = envCfg

		if err := cli.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Successfully set %s.%s\n", envName, key)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}

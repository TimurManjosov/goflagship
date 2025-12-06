package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the CLI configuration
type Config struct {
	DefaultEnv   string                  `yaml:"default_env"`
	Environments map[string]EnvConfig    `yaml:"environments"`
}

// EnvConfig represents configuration for a specific environment
type EnvConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

// GetConfigPath returns the path to the config file
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".flagship", "config.yaml"), nil
}

// LoadConfig loads the configuration from file
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &Config{
				DefaultEnv:   "prod",
				Environments: make(map[string]EnvConfig),
			}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to file
func SaveConfig(cfg *Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetEnvConfig returns configuration for a specific environment
// Priority: command flags > environment variables > config file
// Returns the environment config and the effective environment name
func GetEnvConfig(envName, baseURLFlag, apiKeyFlag string) (*EnvConfig, string, error) {
	// First check command line flags
	if baseURLFlag != "" && apiKeyFlag != "" {
		// When using direct flags, env must be specified
		if envName == "" {
			return nil, "", fmt.Errorf("--env flag is required when using --base-url and --api-key flags")
		}
		return &EnvConfig{
			BaseURL: baseURLFlag,
			APIKey:  apiKeyFlag,
		}, envName, nil
	}

	// Then check environment variables
	envBaseURL := os.Getenv("FLAGSHIP_BASE_URL")
	envAPIKey := os.Getenv("FLAGSHIP_API_KEY")
	if envBaseURL != "" && envAPIKey != "" {
		// When using env vars, env must be specified
		if envName == "" {
			return nil, "", fmt.Errorf("--env flag is required when using FLAGSHIP_BASE_URL and FLAGSHIP_API_KEY environment variables")
		}
		return &EnvConfig{
			BaseURL: envBaseURL,
			APIKey:  envAPIKey,
		}, envName, nil
	}

	// Finally check config file
	cfg, err := LoadConfig()
	if err != nil {
		return nil, "", err
	}

	// Use default env if not specified
	if envName == "" {
		envName = cfg.DefaultEnv
	}

	envCfg, ok := cfg.Environments[envName]
	if !ok {
		return nil, "", fmt.Errorf("environment '%s' not found in config", envName)
	}

	// Override with flags/env vars if provided
	if baseURLFlag != "" {
		envCfg.BaseURL = baseURLFlag
	} else if envBaseURL != "" {
		envCfg.BaseURL = envBaseURL
	}

	if apiKeyFlag != "" {
		envCfg.APIKey = apiKeyFlag
	} else if envAPIKey != "" {
		envCfg.APIKey = envAPIKey
	}

	if envCfg.BaseURL == "" || envCfg.APIKey == "" {
		return nil, "", fmt.Errorf("base_url and api_key must be configured for environment '%s'", envName)
	}

	return &envCfg, envName, nil
}

// InitConfig creates a default config file
func InitConfig() error {
	cfg := &Config{
		DefaultEnv: "prod",
		Environments: map[string]EnvConfig{
			"dev": {
				BaseURL: "http://localhost:8080",
				APIKey:  "dev-key-123",
			},
			"staging": {
				BaseURL: "https://staging.example.com",
				APIKey:  "staging-key-456",
			},
			"prod": {
				BaseURL: "https://flagship.example.com",
				APIKey:  "prod-key-789",
			},
		},
	}

	return SaveConfig(cfg)
}

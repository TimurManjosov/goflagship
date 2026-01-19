// Package config provides application configuration loading from environment variables and .env files.
// It uses viper for flexible configuration management with sensible defaults.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/spf13/viper"
)

// Config holds all application configuration loaded from environment variables or .env file.
// Configuration priority: environment variables > .env file > defaults.
type Config struct {
	AppEnv               string // Application environment (dev, staging, prod)
	HTTPAddr             string // HTTP server bind address (e.g., ":8080")
	DatabaseDSN          string // PostgreSQL connection string
	Env                  string // Flag environment to operate on (prod, dev, etc.)
	AdminAPIKey          string // Admin API key for write operations
	ClientAPIKey         string // Client API key for read operations (legacy)
	MetricsAddr          string // Metrics/pprof server bind address
	StoreType            string // Storage backend type (postgres or memory)
	RateLimitPerIP       int    // Rate limit for unauthenticated requests per IP
	RateLimitPerKey      int    // Rate limit for authenticated requests per key
	RateLimitAdminPerKey int    // Rate limit for admin operations per key
	AuthTokenPrefix      string // Prefix for API tokens (e.g., "fsk_")
	RolloutSalt          string // Salt for deterministic user bucketing in rollouts
	rolloutSaltGenerated bool   // internal: tracks if rollout salt was auto-generated
}

const (
	saltByteSize           = 16 // 16 bytes = 128 bits of entropy
	defaultSaltFallback    = "default-random-salt"
	rolloutSaltWarningMsg  = "WARNING: ROLLOUT_SALT not configured. Generated random salt: %s. User bucket assignments will change on restart. Set ROLLOUT_SALT in production for consistent rollout behavior."
)

// generateRandomSalt creates a cryptographically secure random 16-byte hex-encoded salt.
// Returns a fallback value if random generation fails (should never happen in practice).
func generateRandomSalt() string {
	bytes := make([]byte, saltByteSize)
	if _, err := rand.Read(bytes); err != nil {
		log.Printf("ERROR: Failed to generate random salt: %v. Using fallback.", err)
		return defaultSaltFallback
	}
	return hex.EncodeToString(bytes)
}

// Load reads configuration from environment variables and .env file (if present).
// Environment variables take precedence over .env file values.
// Returns a Config struct with all values populated (either from env or defaults).
//
// Validation:
//   This function performs basic configuration loading but does NOT validate
//   configuration constraints (e.g., postgres store requires valid DSN).
//   Use Validate() method to check production-readiness constraints.
func Load() (*Config, error) {
	viperInstance := viper.New()
	viperInstance.SetConfigFile(".env") // Optional; silently ignored if file doesn't exist
	_ = viperInstance.ReadInConfig()    // Ignore error - .env is optional
	viperInstance.AutomaticEnv()        // Read from environment variables

	setConfigDefaults(viperInstance)
	rolloutSalt, rolloutSaltGenerated := getOrGenerateRolloutSalt(viperInstance)

	return &Config{
		AppEnv:               viperInstance.GetString("APP_ENV"),
		HTTPAddr:             viperInstance.GetString("APP_HTTP_ADDR"),
		DatabaseDSN:          viperInstance.GetString("DB_DSN"),
		Env:                  viperInstance.GetString("ENV"),
		AdminAPIKey:          viperInstance.GetString("ADMIN_API_KEY"),
		ClientAPIKey:         viperInstance.GetString("CLIENT_API_KEY"),
		MetricsAddr:          viperInstance.GetString("METRICS_ADDR"),
		StoreType:            viperInstance.GetString("STORE_TYPE"),
		RateLimitPerIP:       viperInstance.GetInt("RATE_LIMIT_PER_IP"),
		RateLimitPerKey:      viperInstance.GetInt("RATE_LIMIT_PER_KEY"),
		RateLimitAdminPerKey: viperInstance.GetInt("RATE_LIMIT_ADMIN_PER_KEY"),
		AuthTokenPrefix:      viperInstance.GetString("AUTH_TOKEN_PREFIX"),
		RolloutSalt:          rolloutSalt,
		rolloutSaltGenerated: rolloutSaltGenerated,
	}, nil
}

// setConfigDefaults sets default values for all configuration options.
// These defaults are suitable for local development but should be overridden in production.
func setConfigDefaults(v *viper.Viper) {
	v.SetDefault("APP_ENV", "dev")
	v.SetDefault("APP_HTTP_ADDR", ":8080")
	v.SetDefault("DB_DSN", "postgres://flagship:flagship@localhost:5432/flagship?sslmode=disable")
	v.SetDefault("ENV", "prod")
	v.SetDefault("ADMIN_API_KEY", "admin-123") // Change in production!
	v.SetDefault("CLIENT_API_KEY", "client-xyz")
	v.SetDefault("METRICS_ADDR", ":9090")
	v.SetDefault("STORE_TYPE", "postgres")
	v.SetDefault("RATE_LIMIT_PER_IP", 100)
	v.SetDefault("RATE_LIMIT_PER_KEY", 1000)
	v.SetDefault("RATE_LIMIT_ADMIN_PER_KEY", 60)
	v.SetDefault("AUTH_TOKEN_PREFIX", "fsk_")
}

// getOrGenerateRolloutSalt retrieves the ROLLOUT_SALT from config or generates a random one.
// Logs a warning if a random salt is generated, as this will cause inconsistent user bucketing
// across server restarts. In production, ROLLOUT_SALT must be explicitly set.
// Returns the salt and a boolean indicating if it was auto-generated.
func getOrGenerateRolloutSalt(v *viper.Viper) (string, bool) {
	rolloutSalt := v.GetString("ROLLOUT_SALT")
	if rolloutSalt == "" {
		rolloutSalt = generateRandomSalt()
		log.Printf(rolloutSaltWarningMsg, rolloutSalt)
		return rolloutSalt, true // Salt was auto-generated
	}
	return rolloutSalt, false // Salt was explicitly configured
}

// ValidationError represents a configuration validation error with details about what failed.
type ValidationError struct {
	Field   string // Name of the configuration field
	Message string // Human-readable error message
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	return fmt.Sprintf("config validation failed [%s]: %s", e.Field, e.Message)
}

// Validate checks that the configuration is suitable for production use.
//
// This performs stricter validation than Load() and is intended to be called
// at application startup to fail fast on misconfiguration.
//
// Validation Rules:
//   1. StoreType must be one of: "memory", "postgres"
//   2. If StoreType is "postgres", DatabaseDSN must be non-empty
//   3. HTTPAddr must be non-empty
//   4. MetricsAddr must be non-empty
//   5. Env must be non-empty
//   6. RolloutSalt must be non-empty (enforced for production safety)
//
// Production Safety:
//   In production (AppEnv != "dev"), additional constraints apply:
//   - AdminAPIKey must not be the default value "admin-123"
//   - RolloutSalt should be explicitly configured (not auto-generated)
//
// Returns:
//   - nil if configuration is valid
//   - ValidationError describing the first validation failure
//
// Example:
//   cfg, _ := config.Load()
//   if err := cfg.Validate(); err != nil {
//       log.Fatalf("Configuration error: %v", err)
//   }
func (c *Config) Validate() error {
	// 1. Validate store type
	if c.StoreType != "memory" && c.StoreType != "postgres" {
		return ValidationError{
			Field:   "STORE_TYPE",
			Message: fmt.Sprintf("must be 'memory' or 'postgres', got '%s'", c.StoreType),
		}
	}

	// 2. If using postgres, DSN is required
	if c.StoreType == "postgres" && c.DatabaseDSN == "" {
		return ValidationError{
			Field:   "DB_DSN",
			Message: "database DSN is required when STORE_TYPE=postgres",
		}
	}

	// 3. HTTP address is required
	if c.HTTPAddr == "" {
		return ValidationError{
			Field:   "APP_HTTP_ADDR",
			Message: "HTTP server address cannot be empty",
		}
	}

	// 4. Metrics address is required
	if c.MetricsAddr == "" {
		return ValidationError{
			Field:   "METRICS_ADDR",
			Message: "metrics server address cannot be empty",
		}
	}

	// 5. Environment name is required
	if c.Env == "" {
		return ValidationError{
			Field:   "ENV",
			Message: "environment name cannot be empty",
		}
	}

	// 6. Rollout salt is required (critical for deterministic bucketing)
	// Note: Empty check is redundant since getOrGenerateRolloutSalt always returns a value,
	// but we check for auto-generation in production mode below.
	if c.RolloutSalt == "" {
		return ValidationError{
			Field:   "ROLLOUT_SALT",
			Message: "rollout salt cannot be empty (required for consistent user bucketing)",
		}
	}

	// Production-specific checks (stricter validation)
	if c.AppEnv == "prod" || c.AppEnv == "production" {
		// In production, admin key must not be the default
		if c.AdminAPIKey == "admin-123" {
			return ValidationError{
				Field:   "ADMIN_API_KEY",
				Message: "default admin API key 'admin-123' is not allowed in production",
			}
		}
		
		// In production, rollout salt must be explicitly configured (not auto-generated)
		if c.rolloutSaltGenerated {
			return ValidationError{
				Field:   "ROLLOUT_SALT",
				Message: "rollout salt must be explicitly configured in production (not auto-generated). Set ROLLOUT_SALT environment variable.",
			}
		}
	}

	return nil
}

// Package config provides application configuration loading from environment variables and .env files.
// It uses viper for flexible configuration management with sensible defaults.
package config

import (
	"crypto/rand"
	"encoding/hex"
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
func Load() (*Config, error) {
	viperInstance := viper.New()
	viperInstance.SetConfigFile(".env") // Optional; silently ignored if file doesn't exist
	_ = viperInstance.ReadInConfig()    // Ignore error - .env is optional
	viperInstance.AutomaticEnv()        // Read from environment variables

	setConfigDefaults(viperInstance)
	rolloutSalt := getOrGenerateRolloutSalt(viperInstance)

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
func getOrGenerateRolloutSalt(v *viper.Viper) string {
	rolloutSalt := v.GetString("ROLLOUT_SALT")
	if rolloutSalt == "" {
		rolloutSalt = generateRandomSalt()
		log.Printf(rolloutSaltWarningMsg, rolloutSalt)
	}
	return rolloutSalt
}

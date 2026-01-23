// Package config provides application configuration loading from environment variables and .env files.
// It uses viper for flexible configuration management with sensible defaults.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

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
	defaultAdminAPIKey     = "admin-123"
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
	bindEnvAliases(viperInstance)
	viperInstance.AutomaticEnv() // Read from environment variables

	setConfigDefaults(viperInstance)
	appEnv := strings.TrimSpace(viperInstance.GetString("APP_ENV"))
	rolloutSalt, rolloutSaltConfigured, err := getRolloutSalt(viperInstance, appEnv)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		AppEnv:               appEnv,
		HTTPAddr:             strings.TrimSpace(viperInstance.GetString("APP_HTTP_ADDR")),
		DatabaseDSN:          strings.TrimSpace(viperInstance.GetString("DB_DSN")),
		Env:                  strings.TrimSpace(viperInstance.GetString("ENV")),
		AdminAPIKey:          strings.TrimSpace(viperInstance.GetString("ADMIN_API_KEY")),
		ClientAPIKey:         strings.TrimSpace(viperInstance.GetString("CLIENT_API_KEY")),
		MetricsAddr:          strings.TrimSpace(viperInstance.GetString("METRICS_ADDR")),
		StoreType:            strings.ToLower(strings.TrimSpace(viperInstance.GetString("STORE_TYPE"))),
		RateLimitPerIP:       viperInstance.GetInt("RATE_LIMIT_PER_IP"),
		RateLimitPerKey:      viperInstance.GetInt("RATE_LIMIT_PER_KEY"),
		RateLimitAdminPerKey: viperInstance.GetInt("RATE_LIMIT_ADMIN_PER_KEY"),
		AuthTokenPrefix:      strings.TrimSpace(viperInstance.GetString("AUTH_TOKEN_PREFIX")),
		RolloutSalt:          rolloutSalt,
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	warnOnUnsafeDefaults(cfg, rolloutSaltConfigured)

	return cfg, nil
}

// setConfigDefaults sets default values for all configuration options.
// These defaults are suitable for local development but should be overridden in production.
func setConfigDefaults(v *viper.Viper) {
	v.SetDefault("APP_ENV", "dev")
	v.SetDefault("APP_HTTP_ADDR", ":8080")
	v.SetDefault("DB_DSN", "postgres://flagship:flagship@localhost:5432/flagship?sslmode=disable")
	v.SetDefault("ENV", "prod")
	v.SetDefault("ADMIN_API_KEY", defaultAdminAPIKey) // Change in production!
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
func getRolloutSalt(v *viper.Viper, appEnv string) (string, bool, error) {
	rolloutSalt := strings.TrimSpace(v.GetString("ROLLOUT_SALT"))
	if rolloutSalt != "" {
		return rolloutSalt, true, nil
	}
	if strings.EqualFold(appEnv, "prod") {
		return "", false, fmt.Errorf("ROLLOUT_SALT must be set when APP_ENV=prod")
	}
	rolloutSalt = generateRandomSalt()
	log.Printf(rolloutSaltWarningMsg, rolloutSalt)
	return rolloutSalt, false, nil
}

func bindEnvAliases(v *viper.Viper) {
	_ = v.BindEnv("APP_HTTP_ADDR", "APP_HTTP_ADDR", "HTTP_ADDR")
	_ = v.BindEnv("METRICS_ADDR", "METRICS_ADDR", "APP_METRICS_ADDR")
}

func validateConfig(cfg *Config) error {
	if cfg.AppEnv == "" {
		return fmt.Errorf("APP_ENV must not be empty")
	}
	if cfg.HTTPAddr == "" {
		return fmt.Errorf("APP_HTTP_ADDR must not be empty")
	}
	if cfg.MetricsAddr == "" {
		return fmt.Errorf("METRICS_ADDR must not be empty")
	}
	if cfg.Env == "" {
		return fmt.Errorf("ENV must not be empty")
	}
	if cfg.StoreType == "" {
		return fmt.Errorf("STORE_TYPE must not be empty")
	}
	switch cfg.StoreType {
	case "postgres", "memory":
	default:
		return fmt.Errorf("unsupported STORE_TYPE %q (expected postgres or memory)", cfg.StoreType)
	}
	if cfg.StoreType == "postgres" && cfg.DatabaseDSN == "" {
		return fmt.Errorf("DB_DSN must be set when STORE_TYPE=postgres")
	}
	return nil
}

func warnOnUnsafeDefaults(cfg *Config, rolloutSaltConfigured bool) {
	if strings.EqualFold(cfg.AppEnv, "prod") && !rolloutSaltConfigured {
		log.Printf("WARNING: APP_ENV=prod with generated rollout salt. Set ROLLOUT_SALT to stabilize bucketing.")
	}
	if strings.EqualFold(cfg.AppEnv, "prod") && (cfg.AdminAPIKey == "" || cfg.AdminAPIKey == defaultAdminAPIKey) {
		log.Printf("WARNING: APP_ENV=prod with default ADMIN_API_KEY. Set a strong ADMIN_API_KEY before production use.")
	}
}

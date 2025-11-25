package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv               string
	HTTPAddr             string
	DB_DSN               string
	Env                  string
	AdminAPIKey          string
	ClientAPIKey         string
	MetricsAddr          string
	StoreType            string
	RateLimitPerIP       int
	RateLimitPerKey      int
	RateLimitAdminPerKey int
	AuthTokenPrefix      string
	RolloutSalt          string
}

// generateRandomSalt creates a random 16-byte hex-encoded salt
func generateRandomSalt() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "default-random-salt"
	}
	return hex.EncodeToString(b)
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigFile(".env") // optional; ignored if missing
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("APP_ENV", "dev")
	v.SetDefault("APP_HTTP_ADDR", ":8080")
	v.SetDefault("DB_DSN", "postgres://flagship:flagship@localhost:5432/flagship?sslmode=disable")
	v.SetDefault("ENV", "prod")
	v.SetDefault("ADMIN_API_KEY", "admin-123")
	v.SetDefault("CLIENT_API_KEY", "client-xyz")
	v.SetDefault("METRICS_ADDR", ":9090")
	v.SetDefault("STORE_TYPE", "postgres")
	v.SetDefault("RATE_LIMIT_PER_IP", 100)
	v.SetDefault("RATE_LIMIT_PER_KEY", 1000)
	v.SetDefault("RATE_LIMIT_ADMIN_PER_KEY", 60)
	v.SetDefault("AUTH_TOKEN_PREFIX", "fsk_")
	// ROLLOUT_SALT is not set by default - will be generated if not provided

	// Get rollout salt - generate random if not configured
	rolloutSalt := v.GetString("ROLLOUT_SALT")
	if rolloutSalt == "" {
		rolloutSalt = generateRandomSalt()
		log.Printf("WARNING: ROLLOUT_SALT not configured. Generated random salt: %s. User bucket assignments will change on restart. Set ROLLOUT_SALT for consistent rollout behavior.", rolloutSalt)
	}

	return &Config{
		AppEnv:               v.GetString("APP_ENV"),
		HTTPAddr:             v.GetString("APP_HTTP_ADDR"),
		DB_DSN:               v.GetString("DB_DSN"),
		Env:                  v.GetString("ENV"),
		AdminAPIKey:          v.GetString("ADMIN_API_KEY"),
		ClientAPIKey:         v.GetString("CLIENT_API_KEY"),
		MetricsAddr:          v.GetString("METRICS_ADDR"),
		StoreType:            v.GetString("STORE_TYPE"),
		RateLimitPerIP:       v.GetInt("RATE_LIMIT_PER_IP"),
		RateLimitPerKey:      v.GetInt("RATE_LIMIT_PER_KEY"),
		RateLimitAdminPerKey: v.GetInt("RATE_LIMIT_ADMIN_PER_KEY"),
		AuthTokenPrefix:      v.GetString("AUTH_TOKEN_PREFIX"),
		RolloutSalt:          rolloutSalt,
	}, nil
}

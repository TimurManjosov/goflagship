package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Clear any environment variables to test defaults
	env := []string{
		"APP_ENV", "APP_HTTP_ADDR", "DB_DSN", "ENV", "ADMIN_API_KEY",
		"CLIENT_API_KEY", "METRICS_ADDR", "STORE_TYPE", "RATE_LIMIT_PER_IP",
		"RATE_LIMIT_PER_KEY", "RATE_LIMIT_ADMIN_PER_KEY", "AUTH_TOKEN_PREFIX",
	}
	
	for _, key := range env {
		os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify default values
	if cfg.AppEnv != "dev" {
		t.Errorf("Expected AppEnv='dev', got '%s'", cfg.AppEnv)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("Expected HTTPAddr=':8080', got '%s'", cfg.HTTPAddr)
	}
	if cfg.Env != "prod" {
		t.Errorf("Expected Env='prod', got '%s'", cfg.Env)
	}
	if cfg.AdminAPIKey != "admin-123" {
		t.Errorf("Expected AdminAPIKey='admin-123', got '%s'", cfg.AdminAPIKey)
	}
	if cfg.MetricsAddr != ":9090" {
		t.Errorf("Expected MetricsAddr=':9090', got '%s'", cfg.MetricsAddr)
	}
	if cfg.StoreType != "postgres" {
		t.Errorf("Expected StoreType='postgres', got '%s'", cfg.StoreType)
	}
	if cfg.RateLimitPerIP != 100 {
		t.Errorf("Expected RateLimitPerIP=100, got %d", cfg.RateLimitPerIP)
	}
	if cfg.AuthTokenPrefix != "fsk_" {
		t.Errorf("Expected AuthTokenPrefix='fsk_', got '%s'", cfg.AuthTokenPrefix)
	}
}

func TestLoad_EnvironmentOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("APP_ENV", "test")
	os.Setenv("APP_HTTP_ADDR", ":9999")
	os.Setenv("ENV", "staging")
	os.Setenv("ADMIN_API_KEY", "custom-key")
	os.Setenv("METRICS_ADDR", ":7777")
	os.Setenv("STORE_TYPE", "memory")
	os.Setenv("RATE_LIMIT_PER_IP", "200")
	os.Setenv("AUTH_TOKEN_PREFIX", "custom_")
	
	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("APP_HTTP_ADDR")
		os.Unsetenv("ENV")
		os.Unsetenv("ADMIN_API_KEY")
		os.Unsetenv("METRICS_ADDR")
		os.Unsetenv("STORE_TYPE")
		os.Unsetenv("RATE_LIMIT_PER_IP")
		os.Unsetenv("AUTH_TOKEN_PREFIX")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify environment overrides
	if cfg.AppEnv != "test" {
		t.Errorf("Expected AppEnv='test', got '%s'", cfg.AppEnv)
	}
	if cfg.HTTPAddr != ":9999" {
		t.Errorf("Expected HTTPAddr=':9999', got '%s'", cfg.HTTPAddr)
	}
	if cfg.Env != "staging" {
		t.Errorf("Expected Env='staging', got '%s'", cfg.Env)
	}
	if cfg.AdminAPIKey != "custom-key" {
		t.Errorf("Expected AdminAPIKey='custom-key', got '%s'", cfg.AdminAPIKey)
	}
	if cfg.MetricsAddr != ":7777" {
		t.Errorf("Expected MetricsAddr=':7777', got '%s'", cfg.MetricsAddr)
	}
	if cfg.StoreType != "memory" {
		t.Errorf("Expected StoreType='memory', got '%s'", cfg.StoreType)
	}
	if cfg.RateLimitPerIP != 200 {
		t.Errorf("Expected RateLimitPerIP=200, got %d", cfg.RateLimitPerIP)
	}
	if cfg.AuthTokenPrefix != "custom_" {
		t.Errorf("Expected AuthTokenPrefix='custom_', got '%s'", cfg.AuthTokenPrefix)
	}
}

func TestLoad_MissingEnvFileIsAcceptable(t *testing.T) {
	// Even if .env file doesn't exist, Load should succeed with defaults
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should not fail when .env is missing: %v", err)
	}
	if cfg == nil {
		t.Fatal("Config should not be nil")
	}
}

func TestLoad_AllFieldsPopulated(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify all required fields are populated (non-empty)
	if cfg.HTTPAddr == "" {
		t.Error("HTTPAddr should not be empty")
	}
	if cfg.DatabaseDSN == "" {
		t.Error("DatabaseDSN should not be empty")
	}
	if cfg.Env == "" {
		t.Error("Env should not be empty")
	}
	if cfg.MetricsAddr == "" {
		t.Error("MetricsAddr should not be empty")
	}
	if cfg.StoreType == "" {
		t.Error("StoreType should not be empty")
	}
	// Note: AdminAPIKey and ClientAPIKey can be empty in theory,
	// but defaults should populate them
}

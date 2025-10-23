package config

import "github.com/spf13/viper"

type Config struct {
	AppEnv       string
	HTTPAddr     string
	DB_DSN       string
	Env          string
	AdminAPIKey  string
	ClientAPIKey string
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

	return &Config{
		AppEnv:       v.GetString("APP_ENV"),
		HTTPAddr:     v.GetString("APP_HTTP_ADDR"),
		DB_DSN:       v.GetString("DB_DSN"),
		Env:          v.GetString("ENV"),
		AdminAPIKey:  v.GetString("ADMIN_API_KEY"),
		ClientAPIKey: v.GetString("CLIENT_API_KEY"),
	}, nil
}

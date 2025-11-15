package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort    string
	DatabaseURL string
	AdminToken  string
	UserToken   string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		HTTPPort:    getEnv("APP_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AdminToken:  getEnv("ADMIN_TOKEN", "admin-secret"),
		UserToken:   getEnv("USER_TOKEN", "user-secret"),
	}

	if cfg.DatabaseURL == "" {
		host := getEnv("DB_HOST", "postgres")
		port := getEnv("DB_PORT", "5432")
		user := getEnv("DB_USER", "postgres")
		password := getEnv("DB_PASSWORD", "postgres")
		dbName := getEnv("DB_NAME", "pr_service")
		sslMode := getEnv("DB_SSLMODE", "disable")

		cfg.DatabaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, dbName, sslMode)
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultVal
}

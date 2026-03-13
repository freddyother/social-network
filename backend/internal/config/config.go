package config

import (
	"os"
	"strings"
)

type Config struct {
	AppEnv        string
	Host          string
	Port          string
	DatabaseURL   string
	MigrationsDir string
	CORS          CORSConfig
}

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

func Load() Config {
	return Config{
		AppEnv:        getEnv("APP_ENV", "development"),
		Host:          getEnv("HOST", "0.0.0.0"),
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/social_network?sslmode=disable"),
		MigrationsDir: getEnv("MIGRATIONS_DIR", "./pkg/db/migrations/postgres"),
		CORS: CORSConfig{
			AllowedOrigins: splitCSV(
				getEnv(
					"CORS_ALLOWED_ORIGINS",
					"http://localhost:5173,http://127.0.0.1:5173,http://localhost:4173,http://127.0.0.1:4173",
				),
			),
			AllowedMethods:   splitCSV(getEnv("CORS_ALLOWED_METHODS", "GET,POST,PUT,PATCH,DELETE,OPTIONS")),
			AllowedHeaders:   splitCSV(getEnv("CORS_ALLOWED_HEADERS", "Accept,Authorization,Content-Type,X-Requested-With")),
			AllowCredentials: true,
		},
	}
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(strings.Trim(os.Getenv(key), "\"")); value != "" {
		return value
	}

	return fallback
}

func splitCSV(value string) []string {
	rawItems := strings.Split(value, ",")
	items := make([]string, 0, len(rawItems))

	for _, item := range rawItems {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}

	return items
}

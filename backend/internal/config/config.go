package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv        string
	Host          string
	Port          string
	DatabaseURL   string
	MigrationsDir string
	UploadsDir    string
	PublicBaseURL string
	CORS          CORSConfig
	Session       SessionConfig
}

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

type SessionConfig struct {
	CookieName string
	TTL        time.Duration
	Secure     bool
}

func Load() Config {
	appEnv := getEnv("APP_ENV", "development")

	return Config{
		AppEnv:        appEnv,
		Host:          getEnv("HOST", "0.0.0.0"),
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/social_network?sslmode=disable"),
		MigrationsDir: getEnv("MIGRATIONS_DIR", "./pkg/db/migrations/postgres"),
		UploadsDir:    getEnv("UPLOADS_DIR", "./uploads"),
		PublicBaseURL: strings.TrimRight(getEnv("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
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
		Session: SessionConfig{
			CookieName: getEnv("SESSION_COOKIE_NAME", "social_network_session"),
			TTL:        getEnvDurationHours("SESSION_TTL_HOURS", 24*30),
			Secure:     getEnvBool("SESSION_COOKIE_SECURE", appEnv == "production"),
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

func getEnvDurationHours(key string, fallbackHours int) time.Duration {
	value := getEnv(key, "")
	if value == "" {
		return time.Duration(fallbackHours) * time.Hour
	}

	hours, err := strconv.Atoi(value)
	if err != nil || hours <= 0 {
		return time.Duration(fallbackHours) * time.Hour
	}

	return time.Duration(hours) * time.Hour
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.ToLower(getEnv(key, ""))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

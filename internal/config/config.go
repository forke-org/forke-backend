// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	DatabaseURL  string
	JWTSecret    string
	CookieDomain string
	CORSOrigins  []string
	AppEnv       string
}

func Load() *Config {
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://forke:forke_secret@localhost:5433/forke_dev?sslmode=disable")
	jwtSecret := getEnv("JWT_SECRET", "forke-super-secret-jwt-key-change-in-prod")
	cookieDomain := getEnv("COOKIE_DOMAIN", ".forke.space")
	appEnv := getEnv("APP_ENV", "development")

	corsStr := getEnv("CORS_ORIGINS", "http://localhost:3000,http://localhost:3001,http://localhost:3002,https://forke.space,https://dashboard.forke.space,https://admin.forke.space")
	corsOrigins := strings.Split(corsStr, ",")
	for i := range corsOrigins {
		corsOrigins[i] = strings.TrimSpace(corsOrigins[i])
	}

	log.Printf("[Config] Loaded environment: %s, Port: %s", appEnv, port)

	return &Config{
		Port:         port,
		DatabaseURL:  dbURL,
		JWTSecret:    jwtSecret,
		CookieDomain: cookieDomain,
		CORSOrigins:  corsOrigins,
		AppEnv:       appEnv,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

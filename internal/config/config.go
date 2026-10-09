package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// Config owns process configuration. Secrets come from env, never hardcoded.
type Config struct {
	HTTPAddr    string
	JWTSecret   string
	DatabaseURL string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
	BcryptCost  int
}

// Load reads environment with safe defaults for non-secret values.
func Load() Config {
	if err := godotenv.Load(".env.local"); err != nil {
		log.Println("Info: file .env.local tidak ditemukan, membaca dari environment system")
	}

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return Config{
		HTTPAddr:    addr,
		JWTSecret:   os.Getenv("JWT_SECRET"),
		DatabaseURL: firstNonEmpty(os.Getenv("DATABASE_URL"), os.Getenv("DB_URL")),
		AccessTTL:   parseDuration(os.Getenv("JWT_ACCESS_TTL"), domain.AccessTokenTTL),
		RefreshTTL:  parseDuration(os.Getenv("JWT_REFRESH_TTL"), domain.RefreshTokenTTL),
		BcryptCost:  parseInt(os.Getenv("BCRYPT_COST"), 12),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	return fallback
}

func parseInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n
	}
	return fallback
}

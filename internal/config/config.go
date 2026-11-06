package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// Storage holds S3-compatible object storage settings.
// Enabled turns true only when endpoint/bucket/access/secret are all set.
type Storage struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	Region    string
	UseTLS    bool
	// Enabled reports whether object storage is configured. When false
	// the upload endpoint is not registered (dev still boots fine).
	Enabled bool
}

// Config owns process configuration. Secrets come from env, never hardcoded.
type Config struct {
	HTTPAddr           string
	JWTSecret          string
	DatabaseURL        string
	AccessTTL          time.Duration
	RefreshTTL         time.Duration
	BcryptCost         int
	CORSAllowedOrigins []string
	Storage            Storage
}

// LoadStorage reads STORAGE_* env values (opsi upload foto).
func LoadStorage() Storage {
	endpoint := os.Getenv("STORAGE_ENDPOINT")
	bucket := os.Getenv("STORAGE_BUCKET")
	accessKey := os.Getenv("STORAGE_ACCESS_KEY")
	secretKey := os.Getenv("STORAGE_SECRET_KEY")
	region := os.Getenv("STORAGE_REGION")
	useTLS := parseBool(os.Getenv("STORAGE_USE_TLS"))
	enabled := endpoint != "" && bucket != "" && accessKey != "" && secretKey != ""
	return Storage{
		Endpoint:  endpoint,
		Bucket:    bucket,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    region,
		UseTLS:    useTLS,
		Enabled:   enabled,
	}
}

// parseBool accepts "1"/"true" (case-insensitive) as true.
func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
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
		HTTPAddr:           addr,
		JWTSecret:          os.Getenv("JWT_SECRET"),
		DatabaseURL:        firstNonEmpty(os.Getenv("DATABASE_URL"), os.Getenv("DB_URL")),
		AccessTTL:          parseDuration(os.Getenv("JWT_ACCESS_TTL"), domain.AccessTokenTTL),
		RefreshTTL:         parseDuration(os.Getenv("JWT_REFRESH_TTL"), domain.RefreshTokenTTL),
		BcryptCost:         parseInt(os.Getenv("BCRYPT_COST"), 12),
		CORSAllowedOrigins: parseCSV(os.Getenv("CORS_ALLOWED_ORIGINS"), []string{"http://localhost:5173"}),
		Storage:            LoadStorage(),
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

// parseCSV splits a comma-separated env value; empty entries are dropped.
// When nothing remains, the fallback is returned.
func parseCSV(raw string, fallback []string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

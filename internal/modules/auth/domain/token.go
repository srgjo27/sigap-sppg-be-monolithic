package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	// AccessTokenTTL is 15 minutes per MVP-001 general rules.
	AccessTokenTTL = 15 * time.Minute
	// RefreshTokenTTL is 7 days per MVP-001 general rules.
	RefreshTokenTTL = 7 * 24 * time.Hour
)

// Claims is the authenticated principal carried in JWT and context.
type Claims struct {
	UserID    int64
	Role      Role
	SPPGID    *int64
	SekolahID *int64
}

// RefreshToken mirrors the refresh_token table (hash only is stored).
type RefreshToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	UserAgent *string
	IPAddress *string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// Active reports whether the token is usable at now.
func (t *RefreshToken) Active(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}

// GenerateRefreshToken creates a 256-bit random token and its SHA-256 hex hash.
// The plain token is returned to the client once; only the hash is stored.
func GenerateRefreshToken() (plain string, hash string, err error) {
	raw := make([]byte, 32) // 256-bit
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(raw)
	hash = HashRefreshToken(plain)
	return plain, hash, nil
}

// HashRefreshToken returns the hex SHA-256 of a plain refresh token.
func HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

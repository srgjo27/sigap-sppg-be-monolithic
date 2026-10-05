package infrastructure

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// JWTIssuer signs HS256 access tokens containing sub, role, sppg_id, sekolah_id.
type JWTIssuer struct {
	Secret []byte
}

// NewJWTIssuer builds an issuer. Secret must be non-empty in production.
func NewJWTIssuer(secret string) *JWTIssuer {
	return &JWTIssuer{Secret: []byte(secret)}
}

// IssueAccess creates a signed token.
func (j *JWTIssuer) IssueAccess(user *domain.User, ttl time.Duration) (string, time.Time, error) {
	if len(j.Secret) == 0 {
		return "", time.Time{}, fmt.Errorf("jwt secret is empty")
	}
	now := time.Now()
	expires := now.Add(ttl)
	claims := jwt.MapClaims{
		"sub":  user.ID,
		"role": string(user.Peran),
		"exp":  expires.Unix(),
		"iat":  now.Unix(),
	}
	if user.SPPGID != nil {
		claims["sppg_id"] = *user.SPPGID
	}
	if user.SekolahID != nil {
		claims["sekolah_id"] = *user.SekolahID
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.Secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}
	return signed, expires, nil
}

// ParseAccess validates and extracts claims.
func (j *JWTIssuer) ParseAccess(tokenString string) (*domain.Claims, error) {
	if len(j.Secret) == 0 {
		return nil, fmt.Errorf("jwt secret is empty")
	}
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return j.Secret, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	m, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	sub, err := m.GetSubject()
	if err != nil {
		// Fall back to numeric sub.
		var id int64
		switch v := m["sub"].(type) {
		case float64:
			id = int64(v)
		case int64:
			id = v
		default:
			return nil, fmt.Errorf("invalid sub")
		}
		return claimsFromMap(m, id)
	}
	var id int64
	if _, err := fmt.Sscanf(sub, "%d", &id); err != nil {
		// sub may be numeric float already handled; try direct.
		if f, ok := m["sub"].(float64); ok {
			id = int64(f)
		} else {
			return nil, fmt.Errorf("invalid sub")
		}
	}
	return claimsFromMap(m, id)
}

func claimsFromMap(m jwt.MapClaims, id int64) (*domain.Claims, error) {
	roleStr, _ := m["role"].(string)
	role := domain.Role(roleStr)
	if !role.Valid() {
		return nil, fmt.Errorf("invalid role")
	}
	c := &domain.Claims{UserID: id, Role: role}
	if v, ok := m["sppg_id"].(float64); ok {
		sppg := int64(v)
		c.SPPGID = &sppg
	}
	if v, ok := m["sekolah_id"].(float64); ok {
		sek := int64(v)
		c.SekolahID = &sek
	}
	return c, nil
}

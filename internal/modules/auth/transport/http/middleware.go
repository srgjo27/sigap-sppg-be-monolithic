package http

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

const claimsKey = "current_user"

// Middleware holds auth/RBAC/rate-limit dependencies.
type Middleware struct {
	tokens  application.TokenIssuer
	mu      sync.Mutex
	attempt map[string][]time.Time
	now     func() time.Time
}

// NewMiddleware builds transport middleware.
func NewMiddleware(tokens application.TokenIssuer) *Middleware {
	return &Middleware{tokens: tokens, attempt: map[string][]time.Time{}, now: time.Now}
}

// CurrentUser extracts claims stored by Authenticate.
func CurrentUser(c *gin.Context) (*domain.Claims, bool) {
	v, ok := c.Get(claimsKey)
	if !ok {
		return nil, false
	}
	claims, ok := v.(*domain.Claims)
	return claims, ok
}

// Authenticate validates Bearer JWT (MVP-001.8).
func (m *Middleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorEnvelope{Error: ErrorBody{Code: "UNAUTHENTICATED", Message: "tidak login atau token kedaluwarsa"}})
			return
		}
		claims, err := m.tokens.ParseAccess(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorEnvelope{Error: ErrorBody{Code: "UNAUTHENTICATED", Message: "tidak login atau token kedaluwarsa"}})
			return
		}
		c.Set(claimsKey, claims)
		c.Next()
	}
}

// RequireRoles denies roles outside the allowlist (MVP-001.8).
func (m *Middleware) RequireRoles(allowed ...domain.Role) gin.HandlerFunc {
	set := map[domain.Role]bool{}
	for _, r := range allowed {
		set[r] = true
	}
	return func(c *gin.Context) {
		claims, ok := CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorEnvelope{Error: ErrorBody{Code: "UNAUTHENTICATED", Message: "tidak login atau token kedaluwarsa"}})
			return
		}
		if !set[claims.Role] {
			c.AbortWithStatusJSON(http.StatusForbidden, ErrorEnvelope{Error: ErrorBody{Code: "FORBIDDEN", Message: "peran tidak berhak"}})
			return
		}
		c.Next()
	}
}

// RateLimitLogin enforces 10 attempts/minute/IP on login (MVP-001.2).
func (m *Middleware) RateLimitLogin() gin.HandlerFunc {
	const limit = 10
	const window = time.Minute
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := m.now()
		m.mu.Lock()
		cutoff := now.Add(-window)
		kept := make([]time.Time, 0, len(m.attempt[ip])+1)
		for _, t := range m.attempt[ip] {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) >= limit {
			m.attempt[ip] = kept
			m.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, ErrorEnvelope{Error: ErrorBody{Code: "RATE_LIMITED", Message: "terlalu banyak percobaan dari IP yang sama"}})
			return
		}
		kept = append(kept, now)
		m.attempt[ip] = kept
		m.mu.Unlock()
		c.Next()
	}
}

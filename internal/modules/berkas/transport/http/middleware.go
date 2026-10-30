package http

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
)

// UploadLimiter is an in-memory sliding-window rate limiter keyed by
// user ID, in the style of auth's RateLimitLogin (30 uploads/minute/user).
// It is created by the Handler and must run after
// the auth middleware so the user id exists.
type UploadLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	limit    int
	attempts map[string][]time.Time
	now      func() time.Time
}

// NewUploadLimiter builds a limiter allowing limit requests per window per key.
func NewUploadLimiter(limit int, window time.Duration) *UploadLimiter {
	return &UploadLimiter{
		window:   window,
		limit:    limit,
		attempts: map[string][]time.Time{},
		now:      time.Now,
	}
}

// Allow records one attempt for key and reports whether it passes.
func (l *UploadLimiter) Allow(key string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-l.window)
	kept := make([]time.Time, 0, len(l.attempts[key])+1)
	for _, t := range l.attempts[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.limit {
		l.attempts[key] = kept
		return false
	}
	kept = append(kept, now)
	l.attempts[key] = kept
	return true
}

// Reset clears all recorded attempts (used by tests between cases).
func (l *UploadLimiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts = map[string][]time.Time{}
}

// RateLimitUpload enforces 30 uploads/minute/user.
func (h *Handler) RateLimitUpload() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := authhttp.CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorEnvelope{Error: ErrorBody{Code: "UNAUTHENTICATED", Message: "tidak login atau token kedaluwarsa"}})
			return
		}
		if !h.limiter.Allow(strconv.FormatInt(claims.UserID, 10)) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, ErrorEnvelope{Error: ErrorBody{Code: "RATE_LIMITED", Message: "terlalu banyak unggahan"}})
			return
		}
		c.Next()
	}
}

// bodyTooLarge reports whether the multipart decode failed because the
// request exceeded the MaxBytesReader cap.
func bodyTooLarge(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "request body too large") ||
		strings.Contains(msg, "http2: request body too large")
}

// readUploadForm decodes the multipart form (file + konteks) into
// memory (bounded by the MaxBytesReader set by the handler).
func readUploadForm(c *gin.Context) (payload []byte, konteks string, err error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return nil, "", err
	}
	f, err := fileHeader.Open()
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	payload, err = io.ReadAll(f)
	if err != nil {
		return nil, "", err
	}
	return payload, c.PostForm("konteks"), nil
}

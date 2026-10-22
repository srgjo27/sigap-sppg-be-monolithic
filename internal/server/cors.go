package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DefaultCORSOrigins allows the local frontend dev server (Vite on 5173).
// Production origins come from CORS_ALLOWED_ORIGINS instead.
var DefaultCORSOrigins = []string{"http://localhost:5173"}

// CORS answers cross-origin requests from allowlisted origins only.
// The request Origin is echoed back (never "*") so credentialed calls
// (Authorization header) keep working without opening the API to any site.
func CORS(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range origins {
		if o != "" {
			allowed[o] = true
		}
	}
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" || !allowed[origin] {
			c.Next()
			return
		}
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Vary", "Origin")
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Origin")
		h.Set("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newCORSTestRouter(origins []string) *gin.Engine {
	r := gin.New()
	r.Use(CORS(origins))
	r.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"pong": true}) })
	return r
}

func TestCORSAllowsWhitelistedOrigin(t *testing.T) {
	r := newCORSTestRouter([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow-origin = %q", got)
	}
	if got := w.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("vary = %q", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	r := newCORSTestRouter([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("preflight must have no body, got %q", w.Body.String())
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow-origin = %q", got)
	}
	if methods := w.Header().Get("Access-Control-Allow-Methods"); methods == "" {
		t.Fatal("allow-methods must be set")
	}
	if headers := w.Header().Get("Access-Control-Allow-Headers"); headers == "" {
		t.Fatal("allow-headers must be set")
	}
}

func TestCORSDeniesUnknownOrigin(t *testing.T) {
	r := newCORSTestRouter([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "http://evil.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("request must pass through, status = %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unknown origin must get no CORS headers, got %q", got)
	}
}

func TestCORSNoOriginPassesThrough(t *testing.T) {
	r := newCORSTestRouter([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("non-CORS request must get no CORS headers, got %q", got)
	}
}

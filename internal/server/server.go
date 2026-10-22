package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/config"
	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
	menuhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/transport/http"
	pengadaanhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/transport/http"
)

// Deps wires module handlers into the Gin engine.
type Deps struct {
	AuthHandler      *authhttp.Handler
	AuthMiddleware   *authhttp.Middleware
	MenuHandler      *menuhttp.Handler
	PengadaanHandler *pengadaanhttp.Handler
}

// New constructs the Gin engine with health and versioned API routes.
func New(cfg config.Config, deps Deps) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), CORS(cfg.CORSAllowedOrigins))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if deps.AuthHandler != nil && deps.AuthMiddleware != nil {
		v1 := router.Group("/api/v1")
		authhttp.RegisterRoutes(v1, deps.AuthHandler, deps.AuthMiddleware)
		if deps.MenuHandler != nil {
			menuhttp.RegisterRoutes(v1, deps.MenuHandler, deps.AuthMiddleware)
		}
		if deps.PengadaanHandler != nil {
			pengadaanhttp.RegisterRoutes(v1, deps.PengadaanHandler, deps.AuthMiddleware)
		}
	}

	return router
}

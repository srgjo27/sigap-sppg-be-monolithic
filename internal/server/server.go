package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/config"
)

func New(cfg config.Config) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return router
}

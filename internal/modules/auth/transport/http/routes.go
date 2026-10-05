package http

import (
	"github.com/gin-gonic/gin"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// RegisterRoutes mounts MVP-001 endpoints under /api/v1.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, mw *Middleware) {
	manage := mw.RequireRoles(domain.RoleAdmin, domain.RoleKepalaSPPG)
	canReadAudit := mw.RequireRoles(domain.RoleAdmin, domain.RoleKepalaSPPG, domain.RolePengawas)

	rg.POST("/users", mw.Authenticate(), manage, h.CreateUser)
	rg.PATCH("/users/:id", mw.Authenticate(), manage, h.UpdateUser)

	auth := rg.Group("/auth")
	auth.POST("/login", mw.RateLimitLogin(), h.Login)
	auth.POST("/refresh", h.Refresh)
	auth.POST("/logout", mw.Authenticate(), h.Logout)
	auth.GET("/me", mw.Authenticate(), h.Me)
	auth.PUT("/password", mw.Authenticate(), h.ChangePassword)

	rg.GET("/audit-logs", mw.Authenticate(), canReadAudit, h.ListAuditLogs)
}

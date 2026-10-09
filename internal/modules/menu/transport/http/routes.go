package http

import (
	"github.com/gin-gonic/gin"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
)

// RegisterRoutes mounts MVP-002.1 and MVP-002.2 endpoints under /api/v1.
// RBAC follows docs/product/mvp-002-menu-gizi.md and reuses MVP-001.8 middleware.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, mw *authhttp.Middleware) {
	freshPassword := mw.RequirePasswordChanged()
	readOnly := mw.PengawasReadOnly()

	canReadBahan := mw.RequireRoles(
		authdomain.RoleAdmin,
		authdomain.RoleKepalaSPPG,
		authdomain.RoleAhliGizi,
		authdomain.RoleAkuntan,
		authdomain.RolePetugasDapur,
		authdomain.RolePetugasDistribusi,
		authdomain.RolePengawas,
	)
	canWriteBahan := mw.RequireRoles(authdomain.RoleAdmin, authdomain.RoleAhliGizi)
	canCreateMenu := mw.RequireRoles(authdomain.RoleAhliGizi)

	rg.GET("/bahan", mw.Authenticate(), freshPassword, readOnly, canReadBahan, h.ListBahan)
	rg.POST("/bahan", mw.Authenticate(), freshPassword, readOnly, canWriteBahan, h.CreateBahan)
	rg.PATCH("/bahan/:id", mw.Authenticate(), freshPassword, readOnly, canWriteBahan, h.UpdateBahan)

	rg.POST("/menus", mw.Authenticate(), freshPassword, readOnly, canCreateMenu, h.CreateMenu)
}

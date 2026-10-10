package http

import (
	"github.com/gin-gonic/gin"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
)

// RegisterRoutes mounts MVP-002.1 through MVP-002.4 endpoints under /api/v1.
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
	canReadMenu := mw.RequireRoles(
		authdomain.RoleAdmin,
		authdomain.RoleKepalaSPPG,
		authdomain.RoleAhliGizi,
		authdomain.RoleAkuntan,
		authdomain.RolePetugasDapur,
		authdomain.RolePetugasDistribusi,
		authdomain.RolePICsekolah,
		authdomain.RolePengawas,
	)
	canWriteMenu := mw.RequireRoles(authdomain.RoleAhliGizi)
	canApproveMenu := mw.RequireRoles(authdomain.RoleKepalaSPPG)
	canReadKebutuhan := mw.RequireRoles(
		authdomain.RoleAhliGizi,
		authdomain.RoleAkuntan,
		authdomain.RoleKepalaSPPG,
	)

	rg.GET("/bahan", mw.Authenticate(), freshPassword, readOnly, canReadBahan, h.ListBahan)
	rg.POST("/bahan", mw.Authenticate(), freshPassword, readOnly, canWriteBahan, h.CreateBahan)
	rg.PATCH("/bahan/:id", mw.Authenticate(), freshPassword, readOnly, canWriteBahan, h.UpdateBahan)

	rg.POST("/menus", mw.Authenticate(), freshPassword, readOnly, canCreateMenu, h.CreateMenu)
	rg.GET("/menus", mw.Authenticate(), freshPassword, readOnly, canReadMenu, h.ListMenus)
	rg.GET("/menus/:id", mw.Authenticate(), freshPassword, readOnly, canReadMenu, h.GetMenu)
	rg.PATCH("/menus/:id", mw.Authenticate(), freshPassword, readOnly, canWriteMenu, h.UpdateMenu)
	rg.DELETE("/menus/:id", mw.Authenticate(), freshPassword, readOnly, canWriteMenu, h.DeleteMenu)
	rg.POST("/menus/:id/approve", mw.Authenticate(), freshPassword, readOnly, canApproveMenu, h.ApproveMenu)
	rg.POST("/menus/:id/revert", mw.Authenticate(), freshPassword, readOnly, canApproveMenu, h.RevertMenu)
	rg.GET("/menus/:id/kebutuhan-bahan", mw.Authenticate(), freshPassword, readOnly, canReadKebutuhan, h.GetKebutuhan)
}

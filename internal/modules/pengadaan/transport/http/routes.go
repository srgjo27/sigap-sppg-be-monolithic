package http

import (
	"github.com/gin-gonic/gin"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
)

// RegisterRoutes mounts MVP-003.1, MVP-003.2, MVP-003.3, MVP-003.4 endpoints under /api/v1.
// RBAC follows docs/product/mvp-003-pengadaan-penerimaan.md and reuses MVP-001.8 middleware.
// MVP-003.7 POST /uploads is mounted by the berkas module.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, mw *authhttp.Middleware) {
	freshPassword := mw.RequirePasswordChanged()
	readOnly := mw.PengawasReadOnly()

	canReadPemasok := mw.RequireRoles(
		authdomain.RoleAdmin,
		authdomain.RoleKepalaSPPG,
		authdomain.RoleAkuntan,
		authdomain.RoleAhliGizi,
		authdomain.RolePengawas,
	)
	canWritePemasok := mw.RequireRoles(
		authdomain.RoleAdmin,
		authdomain.RoleKepalaSPPG,
		authdomain.RoleAkuntan,
	)

	rg.GET("/pemasok", mw.Authenticate(), freshPassword, readOnly, canReadPemasok, h.ListPemasok)
	rg.POST("/pemasok", mw.Authenticate(), freshPassword, readOnly, canWritePemasok, h.CreatePemasok)
	rg.GET("/pemasok/:id", mw.Authenticate(), freshPassword, readOnly, canReadPemasok, h.GetPemasok)
	rg.PATCH("/pemasok/:id", mw.Authenticate(), freshPassword, readOnly, canWritePemasok, h.UpdatePemasok)

	canReadKebutuhan := mw.RequireRoles(
		authdomain.RoleAkuntan,
		authdomain.RoleAhliGizi,
		authdomain.RoleKepalaSPPG,
	)

	rg.GET("/pengadaan/kebutuhan", mw.Authenticate(), freshPassword, readOnly, canReadKebutuhan, h.GetKebutuhan)

	canCreatePO := mw.RequireRoles(authdomain.RoleAkuntan)

	canReadPO := mw.RequireRoles(
		authdomain.RoleAdmin,
		authdomain.RoleKepalaSPPG,
		authdomain.RoleAkuntan,
		authdomain.RoleAhliGizi,
		authdomain.RolePetugasDapur,
		authdomain.RolePengawas,
	)

	rg.POST("/purchase-orders", mw.Authenticate(), freshPassword, readOnly, canCreatePO, h.CreatePurchaseOrder)
	rg.GET("/purchase-orders", mw.Authenticate(), freshPassword, readOnly, canReadPO, h.ListPurchaseOrders)
	rg.GET("/purchase-orders/:id", mw.Authenticate(), freshPassword, readOnly, canReadPO, h.GetPurchaseOrder)
	rg.PATCH("/purchase-orders/:id", mw.Authenticate(), freshPassword, readOnly, canCreatePO, h.UpdatePurchaseOrder)
	rg.DELETE("/purchase-orders/:id", mw.Authenticate(), freshPassword, readOnly, canCreatePO, h.DeletePurchaseOrder)

	canBatalPO := mw.RequireRoles(
		authdomain.RoleAkuntan,
		authdomain.RoleKepalaSPPG,
	)
	rg.POST("/purchase-orders/:id/kirim", mw.Authenticate(), freshPassword, readOnly, canCreatePO, h.KirimPurchaseOrder)
	rg.POST("/purchase-orders/:id/batal", mw.Authenticate(), freshPassword, readOnly, canBatalPO, h.BatalPurchaseOrder)
	rg.POST("/purchase-orders/:id/tutup", mw.Authenticate(), freshPassword, readOnly, canCreatePO, h.TutupPurchaseOrder)
}

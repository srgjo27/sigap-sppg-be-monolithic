package http

import (
	"github.com/gin-gonic/gin"

	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
)

// RegisterRoutes mounts MVP-003.7 POST /api/v1/uploads.
// Chain: Authenticate (claims) -> PengawasReadOnly (pengawas denied for
// uploads) -> RequirePasswordChanged -> RateLimitUpload (per user) ->
// Upload. The application service re-checks konteks/role rules as
// defense in depth.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, mw *authhttp.Middleware) {
	rg.POST("/uploads",
		mw.Authenticate(),
		mw.PengawasReadOnly(),
		mw.RequirePasswordChanged(),
		h.RateLimitUpload(),
		h.Upload,
	)
}

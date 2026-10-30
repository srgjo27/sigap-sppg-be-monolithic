package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/domain"
)

const (
	// maxRequestBodyBytes bounds the raw multipart body: max upload
	// (5 MB) plus generous multipart-form overhead.
	maxRequestBodyBytes = 6 << 20
	rateLimitUpload     = 30
	rateLimitWindow     = time.Minute
)

// Handler implements POST /api/v1/uploads.
type Handler struct {
	svc     *application.Service
	limiter *UploadLimiter
}

// NewHandler builds the transport handler.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc, limiter: NewUploadLimiter(rateLimitUpload, rateLimitWindow)}
}

// Upload handles POST /uploads (multipart/form-data: file + konteks).
func (h *Handler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
	payload, konteks, err := readUploadForm(c)
	if err != nil {
		if bodyTooLarge(err) {
			c.JSON(http.StatusRequestEntityTooLarge, ErrorEnvelope{Error: ErrorBody{
				Code:    "FILE_TOO_LARGE",
				Message: "file lebih dari 5 MB",
			}})
			return
		}
		MapError(c, domain.ErrFileEmpty)
		return
	}

	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.Upload(c.Request.Context(), claims, konteks, payload)
	if err != nil {
		MapError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"berkas_id":   res.BerkasID,
		"konteks":     res.Konteks,
		"mime_type":   res.MimeType,
		"ukuran_byte": res.UkuranByte,
		"sha256":      res.SHA256,
		"diunggah_at": res.DiunggahAt,
		"url":         res.URL,
	})
}

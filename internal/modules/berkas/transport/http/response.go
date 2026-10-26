package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/domain"
)

// ErrorBody / ErrorEnvelope — same envelope shape as other modules.
type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// MapError maps berkas domain sentinels to stable HTTP error responses.
// Everything unmapped falls back to 500 INTERNAL_ERROR.
func MapError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrFileEmpty):
		c.JSON(http.StatusBadRequest, ErrorEnvelope{Error: ErrorBody{Code: "VALIDATION_ERROR", Message: "file kosong atau tidak terbaca"}})
	case errors.Is(err, domain.ErrKonteksUnknown):
		c.JSON(http.StatusBadRequest, ErrorEnvelope{Error: ErrorBody{Code: "KONTEKS_TIDAK_DIKENAL", Message: "konteks tidak dikenal"}})
	case errors.Is(err, domain.ErrKonteksForbidden):
		c.JSON(http.StatusForbidden, ErrorEnvelope{Error: ErrorBody{Code: "FORBIDDEN", Message: "peran tidak berhak untuk konteks ini"}})
	case errors.Is(err, domain.ErrFileTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, ErrorEnvelope{Error: ErrorBody{Code: "FILE_TOO_LARGE", Message: "file lebih dari 5 MB"}})
	case errors.Is(err, domain.ErrUnsupportedImage):
		c.JSON(http.StatusUnsupportedMediaType, ErrorEnvelope{Error: ErrorBody{Code: "TIPE_BERKAS_TIDAK_DIDUKUNG", Message: "tipe file harus JPEG, PNG, atau WebP"}})
	case errors.Is(err, domain.ErrSPPGNotFound):
		c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "SPPG_NOT_FOUND", Message: "sppg tidak ditemukan"}})
	case errors.Is(err, domain.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, ErrorEnvelope{Error: ErrorBody{Code: "UNAUTHENTICATED", Message: "tidak login"}})
	default:
		c.JSON(http.StatusInternalServerError, ErrorEnvelope{Error: ErrorBody{Code: "INTERNAL_ERROR", Message: "unexpected internal error"}})
	}
}

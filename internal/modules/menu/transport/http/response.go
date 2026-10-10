package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
)

// ErrorEnvelope is the stable transport error shape.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody carries machine-readable code and safe message.
type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: message}})
}

// MapError converts domain/application errors to HTTP responses.
func MapError(c *gin.Context, err error) {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		c.JSON(http.StatusBadRequest, ErrorEnvelope{Error: ErrorBody{Code: "VALIDATION_ERROR", Message: "invalid input", Fields: ve.Fields}})
		return
	}
	switch {
	case errors.Is(err, domain.ErrBahanExists):
		writeError(c, http.StatusConflict, "BAHAN_EXISTS", "nama bahan already exists")
	case errors.Is(err, domain.ErrMenuExists):
		writeError(c, http.StatusConflict, "MENU_EXISTS", "menu untuk tanggal tersebut already exists")
	case errors.Is(err, domain.ErrBahanNotFound):
		writeError(c, http.StatusNotFound, "BAHAN_NOT_FOUND", "bahan not found")
	case errors.Is(err, domain.ErrMenuNotFound):
		writeError(c, http.StatusNotFound, "MENU_NOT_FOUND", "menu not found")
	case errors.Is(err, domain.ErrBahanInactive):
		writeError(c, 422, "BAHAN_INACTIVE", "bahan nonaktif")
	case errors.Is(err, domain.ErrDuplicateBahan):
		writeError(c, 422, "DUPLICATE_BAHAN", "duplikat bahan dalam komposisi")
	case errors.Is(err, domain.ErrMenuApproved):
		writeError(c, http.StatusConflict, "MENU_APPROVED", "menu sudah disetujui")
	case errors.Is(err, domain.ErrMenuInUse):
		writeError(c, http.StatusConflict, "MENU_IN_USE", "menu sudah dipakai di batch produksi")
	case errors.Is(err, domain.ErrUnauthorized):
		writeError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "tidak login atau token kedaluwarsa")
	case errors.Is(err, domain.ErrForbidden):
		writeError(c, http.StatusForbidden, "FORBIDDEN", "peran tidak berhak")
	case errors.Is(err, domain.ErrNotFound):
		writeError(c, http.StatusNotFound, "NOT_FOUND", "data tidak ditemukan")
	default:
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected internal error")
	}
}

package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// ErrorEnvelope is the stable transport error shape.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody carries machine-readable code and safe message.
type ErrorBody struct {
	Code           string            `json:"code"`
	Message        string            `json:"message"`
	Fields         map[string]string `json:"fields,omitempty"`
	TerkunciSampai *string           `json:"terkunci_sampai,omitempty"`
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
	var locked *application.AccountLockedError
	if errors.As(err, &locked) {
		ts := locked.Until.Format(time.RFC3339)
		c.JSON(423, ErrorEnvelope{Error: ErrorBody{Code: "ACCOUNT_LOCKED", Message: "akun terkunci sementara", TerkunciSampai: &ts}})
		return
	}
	switch {
	case errors.Is(err, domain.ErrEmailExists):
		writeError(c, http.StatusConflict, "EMAIL_EXISTS", "email already exists")
	case errors.Is(err, domain.ErrInvalidCredentials):
		writeError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email atau password salah")
	case errors.Is(err, domain.ErrRefreshInvalid):
		writeError(c, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "invalid refresh token")
	case errors.Is(err, domain.ErrRefreshExpired):
		writeError(c, http.StatusUnauthorized, "REFRESH_EXPIRED", "refresh token expired")
	case errors.Is(err, domain.ErrRefreshRevoked):
		writeError(c, http.StatusUnauthorized, "REFRESH_REVOKED", "refresh token revoked")
	case errors.Is(err, domain.ErrUnauthorized):
		writeError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "tidak login atau token kedaluwarsa")
	case errors.Is(err, domain.ErrAccountInactive):
		writeError(c, http.StatusForbidden, "ACCOUNT_INACTIVE", "akun nonaktif")
	case errors.Is(err, domain.ErrForbidden):
		writeError(c, http.StatusForbidden, "FORBIDDEN", "peran tidak berhak")
	case errors.Is(err, domain.ErrNotFound):
		writeError(c, http.StatusNotFound, "NOT_FOUND", "data tidak ditemukan")
	case errors.Is(err, domain.ErrSPPGNotFound):
		writeError(c, http.StatusNotFound, "SPPG_NOT_FOUND", "sppg_id tidak ditemukan")
	case errors.Is(err, domain.ErrSekolahNotFound):
		writeError(c, http.StatusNotFound, "SEKOLAH_NOT_FOUND", "sekolah_id tidak ditemukan")
	case errors.Is(err, domain.ErrSekolahScopeMismatch):
		c.JSON(http.StatusBadRequest, ErrorEnvelope{Error: ErrorBody{Code: "VALIDATION_ERROR", Message: "sekolah does not belong to sppg", Fields: map[string]string{"sekolah_id": "must belong to the same sppg_id"}}})
	case errors.Is(err, domain.ErrRateLimited):
		writeError(c, http.StatusTooManyRequests, "RATE_LIMITED", "terlalu banyak percobaan")
	default:
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected internal error")
	}
}

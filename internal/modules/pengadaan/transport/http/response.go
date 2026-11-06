package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// MapError converts a domain/application error into a stable HTTP response.
// Wrapped errors are unwrapped with errors.Is/As so sentinels survive
// fmt.Errorf("%w") context wrapping.
func MapError(c *gin.Context, err error) {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		c.JSON(http.StatusBadRequest, ErrorEnvelope{Error: ErrorBody{Code: "VALIDATION_ERROR", Message: "invalid input", Fields: ve.Fields}})
		return
	}

	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, ErrorEnvelope{Error: ErrorBody{Code: "UNAUTHENTICATED", Message: "tidak login atau token kedaluwarsa"}})
	case errors.Is(err, domain.ErrForbidden):
		c.JSON(http.StatusForbidden, ErrorEnvelope{Error: ErrorBody{Code: "FORBIDDEN", Message: "peran tidak berhak"}})
	case errors.Is(err, domain.ErrSelfCorrection):
		c.JSON(http.StatusForbidden, ErrorEnvelope{Error: ErrorBody{Code: "SELF_CORRECTION", Message: "cannot correct own receipt"}})
	case errors.Is(err, domain.ErrPemasokNotFound):
		c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "pemasok not found"}})
	case errors.Is(err, domain.ErrPemasokExists):
		c.JSON(http.StatusConflict, ErrorEnvelope{Error: ErrorBody{Code: "DUPLICATE_NAME", Message: "pemasok with this name already exists in this SPPG"}})
	case errors.Is(err, domain.ErrSPPGNotFound):
		c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "SPPG_NOT_FOUND", Message: "sppg not found"}})
	case errors.Is(err, domain.ErrPemasokInactive):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "PEMASOK_NONAKTIF", Message: "pemasok is inactive"}})
	case errors.Is(err, domain.ErrBahanNotFound):
		c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "bahan not found"}})
	case errors.Is(err, domain.ErrBahanInactive):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "BAHAN_NONAKTIF", Message: "bahan is inactive"}})
	case errors.Is(err, domain.ErrMenuNotFound):
		c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "menu not found"}})
	case errors.Is(err, domain.ErrMenuNotApproved):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "MENU_NOT_APPROVED", Message: "menu is not approved"}})
	case errors.Is(err, domain.ErrSiklusNotFound):
		c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "siklus not found"}})
	case errors.Is(err, domain.ErrSiklusClosed):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "SIKLUS_CLOSED", Message: "siklus is closed"}})
	case errors.Is(err, domain.ErrPONotFound):
		c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "purchase order not found"}})
	case errors.Is(err, domain.ErrPOInvalidStatus):
		c.JSON(http.StatusConflict, ErrorEnvelope{Error: ErrorBody{Code: "STATUS_TIDAK_SESUAI", Message: "purchase order status does not allow this action"}})
	case errors.Is(err, domain.ErrPOHasReceipt):
		c.JSON(http.StatusConflict, ErrorEnvelope{Error: ErrorBody{Code: "PO_SUDAH_ADA_PENERIMAAN", Message: "purchase order already has receipt"}})
	case errors.Is(err, domain.ErrKodeLotConflict):
		c.JSON(http.StatusConflict, ErrorEnvelope{Error: ErrorBody{Code: "KODE_LOT_GANDA", Message: "kode_lot conflict, please retry"}})
	case errors.Is(err, domain.ErrPONumberConflict):
		c.JSON(http.StatusConflict, ErrorEnvelope{Error: ErrorBody{Code: "PO_NOMOR_GANDA", Message: "purchase order number conflict, please retry"}})
	case errors.Is(err, domain.ErrItemDuplicate):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "DUPLICATE_ITEM", Message: "duplicate bahan in purchase order"}})
	case errors.Is(err, domain.ErrItemNotFound):
		c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "po item not found"}})
	case errors.Is(err, domain.ErrExceedsTolerance):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "MELEBIHI_PESANAN", Message: "quantity exceeds 110% tolerance"}})
	case errors.Is(err, domain.ErrTemperatureExceeded):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "SUHU_MELEBIHI_BATAS", Message: "temperature exceeds maximum storage limit"}})
	case errors.Is(err, domain.ErrExpired):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "BAHAN_KEDALUWARSA", Message: "item is expired"}})
	case errors.Is(err, domain.ErrDamagedAccepted):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "INVALID_KONDISI", Message: "damaged item cannot be accepted"}})
	case errors.Is(err, domain.ErrBerkasInvalid):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "BERKAS_TIDAK_VALID", Message: "berkas is invalid or already used"}})
	case errors.Is(err, domain.ErrLotNotFound):
		c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "lot not found"}})
	case errors.Is(err, domain.ErrLotUsedInBatch):
		c.JSON(http.StatusConflict, ErrorEnvelope{Error: ErrorBody{Code: "LOT_SUDAH_DIPAKAI", Message: "lot already used in production batch"}})
	case errors.Is(err, domain.ErrCorrectionViolates):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "CORRECTION_VIOLATES", Message: "correction violates receipt rules"}})
	case errors.Is(err, domain.ErrPOCannotBeSent):
		c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "PO_TIDAK_DAPAT_DIKIRIM", Message: "purchase order cannot be sent"}})
	default:
		c.JSON(http.StatusInternalServerError, ErrorEnvelope{Error: ErrorBody{Code: "INTERNAL_ERROR", Message: "unexpected internal error"}})
	}
}

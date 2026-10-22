package http

import (
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

func MapError(c *gin.Context, err error) {
	switch e := err.(type) {
	case *domain.ValidationError:
		c.JSON(http.StatusBadRequest, ErrorEnvelope{Error: ErrorBody{Code: "VALIDATION_ERROR", Message: "invalid input", Fields: e.Fields}})
	default:
		switch err {
		case domain.ErrUnauthorized:
			c.JSON(http.StatusUnauthorized, ErrorEnvelope{Error: ErrorBody{Code: "UNAUTHENTICATED", Message: "tidak login atau token kedaluwarsa"}})
		case domain.ErrForbidden:
			c.JSON(http.StatusForbidden, ErrorEnvelope{Error: ErrorBody{Code: "FORBIDDEN", Message: "peran tidak berhak"}})
		case domain.ErrPemasokNotFound:
			c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "pemasok not found"}})
		case domain.ErrPemasokExists:
			c.JSON(http.StatusConflict, ErrorEnvelope{Error: ErrorBody{Code: "DUPLICATE_NAME", Message: "pemasok with this name already exists in this SPPG"}})
		case domain.ErrPemasokInactive:
			c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "PEMASOK_NONAKTIF", Message: "pemasok is inactive"}})
		case domain.ErrBahanNotFound:
			c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "bahan not found"}})
		case domain.ErrBahanInactive:
			c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "BAHAN_NONAKTIF", Message: "bahan is inactive"}})
		case domain.ErrMenuNotFound:
			c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "menu not found"}})
		case domain.ErrMenuNotApproved:
			c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "MENU_NOT_APPROVED", Message: "menu is not approved"}})
		case domain.ErrSiklusNotFound:
			c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "siklus not found"}})
		case domain.ErrSiklusClosed:
			c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "SIKLUS_CLOSED", Message: "siklus is closed"}})
		case domain.ErrPONotFound:
			c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "purchase order not found"}})
		case domain.ErrPOInvalidStatus:
			c.JSON(http.StatusConflict, ErrorEnvelope{Error: ErrorBody{Code: "STATUS_TIDAK_SESUAI", Message: "purchase order status does not allow this action"}})
		case domain.ErrPOHasReceipt:
			c.JSON(http.StatusConflict, ErrorEnvelope{Error: ErrorBody{Code: "PO_SUDAH_ADA_PENERIMAAN", Message: "purchase order already has receipt"}})
		case domain.ErrItemDuplicate:
			c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "DUPLICATE_ITEM", Message: "duplicate bahan in purchase order"}})
		case domain.ErrItemNotFound:
			c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "po item not found"}})
		case domain.ErrExceedsTolerance:
			c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "MELEBIHI_PESANAN", Message: "quantity exceeds 110% tolerance"}})
		case domain.ErrTemperatureExceeded:
			c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "SUHU_MELEBIHI_BATAS", Message: "temperature exceeds maximum storage limit"}})
		case domain.ErrExpired:
			c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "BAHAN_KEDALUWARSA", Message: "item is expired"}})
		case domain.ErrDamagedAccepted:
			c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "INVALID_KONDISI", Message: "damaged item cannot be accepted"}})
		case domain.ErrBerkasInvalid:
			c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "BERKAS_TIDAK_VALID", Message: "berkas is invalid or already used"}})
		case domain.ErrLotNotFound:
			c.JSON(http.StatusNotFound, ErrorEnvelope{Error: ErrorBody{Code: "NOT_FOUND", Message: "lot not found"}})
		case domain.ErrLotUsedInBatch:
			c.JSON(http.StatusConflict, ErrorEnvelope{Error: ErrorBody{Code: "LOT_SUDAH_DIPAKAI", Message: "lot already used in production batch"}})
		case domain.ErrSelfCorrection:
			c.JSON(http.StatusForbidden, ErrorEnvelope{Error: ErrorBody{Code: "SELF_CORRECTION", Message: "cannot correct own receipt"}})
		case domain.ErrCorrectionViolates:
			c.JSON(http.StatusUnprocessableEntity, ErrorEnvelope{Error: ErrorBody{Code: "CORRECTION_VIOLATES", Message: "correction violates receipt rules"}})
		default:
			c.JSON(http.StatusInternalServerError, ErrorEnvelope{Error: ErrorBody{Code: "INTERNAL_ERROR", Message: "unexpected internal error"}})
		}
	}
}

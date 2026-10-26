package domain

import "errors"

// Sentinel domain errors for pengadaan module.
var (
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrPemasokNotFound     = errors.New("pemasok not found")
	ErrPemasokExists       = errors.New("pemasok with this name already exists in this SPPG")
	ErrPemasokInactive     = errors.New("pemasok is inactive")
	ErrPemasokHasPO        = errors.New("pemasok has associated purchase orders")
	ErrBahanNotFound       = errors.New("bahan not found")
	ErrBahanInactive       = errors.New("bahan is inactive")
	ErrMenuNotFound        = errors.New("menu not found")
	ErrMenuNotApproved     = errors.New("menu is not approved")
	ErrSiklusNotFound      = errors.New("siklus not found")
	ErrSiklusClosed        = errors.New("siklus is closed")
	ErrPONotFound          = errors.New("purchase order not found")
	ErrPOInvalidStatus     = errors.New("purchase order status does not allow this action")
	ErrPOHasReceipt        = errors.New("purchase order already has receipt")
	ErrItemDuplicate       = errors.New("duplicate bahan in purchase order")
	ErrItemNotFound        = errors.New("po item not found")
	ErrExceedsTolerance    = errors.New("quantity exceeds 110% tolerance")
	ErrTemperatureExceeded = errors.New("temperature exceeds maximum storage limit")
	ErrExpired             = errors.New("item is expired")
	ErrDamagedAccepted     = errors.New("damaged item cannot be accepted")
	ErrBerkasInvalid       = errors.New("berkas is invalid or already used")
	ErrLotNotFound         = errors.New("lot not found")
	ErrLotUsedInBatch      = errors.New("lot already used in production batch")
	ErrSelfCorrection      = errors.New("cannot correct own receipt")
	ErrCorrectionViolates  = errors.New("correction violates receipt rules")
	ErrPOCannotBeSent      = errors.New("purchase order cannot be sent (no items, past delivery date)")
	ErrPONumberConflict    = errors.New("purchase order number conflict")
	ErrSPPGNotFound        = errors.New("sppg not found")
)

// ValidationError holds field-level validation errors.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return "validation error"
}

// IsValidationError reports whether err is a ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

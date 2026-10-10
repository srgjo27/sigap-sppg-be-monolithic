package domain

import "errors"

// Sentinel menu-domain errors mapped to HTTP by transport.
var (
	ErrBahanExists    = errors.New("bahan already exists")
	ErrBahanNotFound  = errors.New("bahan not found")
	ErrMenuExists     = errors.New("menu for the date already exists")
	ErrMenuNotFound   = errors.New("menu not found")
	ErrBahanInactive  = errors.New("bahan is inactive")
	ErrDuplicateBahan = errors.New("duplicate bahan in composition")
	ErrMenuApproved   = errors.New("menu already approved")
	ErrMenuInUse      = errors.New("menu already used in production")
	ErrMenuIncomplete = errors.New("menu is incomplete")
	ErrStatusConflict = errors.New("menu status conflict")
	ErrForbidden      = errors.New("forbidden")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrNotFound       = errors.New("not found")
)

// ValidationError carries field-level validation failures.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "validation failed" }

// MenuDateConflictError carries the conflicting target date for copy (409).
// Unlike ErrMenuExists it names the date, as required by MVP-002.7.
type MenuDateConflictError struct {
	Tanggal string
}

func (e *MenuDateConflictError) Error() string {
	return "menu for tanggal " + e.Tanggal + " already exists"
}

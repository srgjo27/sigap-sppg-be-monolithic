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
	ErrForbidden      = errors.New("forbidden")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrNotFound       = errors.New("not found")
)

// ValidationError carries field-level validation failures.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "validation failed" }

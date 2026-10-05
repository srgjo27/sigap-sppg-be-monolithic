package domain

import "errors"

// Sentinel domain errors mapped to HTTP by transport.
var (
	ErrEmailExists          = errors.New("email already exists")
	ErrInvalidCredentials   = errors.New("invalid email or password")
	ErrAccountInactive      = errors.New("account is inactive")
	ErrAccountLocked        = errors.New("account is temporarily locked")
	ErrNotFound             = errors.New("not found")
	ErrForbidden            = errors.New("forbidden")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrRefreshInvalid       = errors.New("invalid refresh token")
	ErrRefreshExpired       = errors.New("refresh token expired")
	ErrRefreshRevoked       = errors.New("refresh token revoked")
	ErrRateLimited          = errors.New("too many attempts")
	ErrSPPGNotFound         = errors.New("sppg not found")
	ErrSekolahNotFound      = errors.New("sekolah not found")
	ErrSekolahScopeMismatch = errors.New("sekolah does not belong to sppg")
	ErrMustChangePassword   = errors.New("password change required")
	ErrSelfModification     = errors.New("cannot deactivate or demote self")
	ErrLastKepalaRequired   = errors.New("sppg must retain an active kepala_sppg")
	ErrOldPasswordMismatch  = errors.New("old password is incorrect")
)

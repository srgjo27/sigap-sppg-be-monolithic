package domain

import (
	"crypto/rand"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	minNamaLen = 2
	maxNamaLen = 120

	minPasswordLen = 8

	maxFailedAttempts = 5
	lockoutDuration   = 15 * time.Minute
)

var digitsOnly = regexp.MustCompile(`^\d+$`)

// User mirrors the users table in db/schema.sql.
type User struct {
	ID                 int64
	SPPGID             *int64
	SekolahID          *int64
	Nama               string
	Email              string
	NoHP               *string
	PasswordHash       string
	Peran              Role
	Aktif              bool
	LastLoginAt        *time.Time
	GagalLogin         int
	TerkunciSampai     *time.Time
	WajibGantiPassword bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ValidationError carries field-level validation failures.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "validation failed" }

// NormalizeEmail lowercases and trims an email address.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail checks presence and basic format.
func ValidateEmail(email string) bool {
	if email == "" || len(email) > 150 {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

// ValidateNama checks 2-120 characters.
func ValidateNama(nama string) bool {
	n := len([]rune(strings.TrimSpace(nama)))
	return n >= minNamaLen && n <= maxNamaLen
}

// ValidatePhone checks Indonesian format: 08... or +628... or 628...,
// 10-15 digits total.
func ValidatePhone(phone string) bool {
	p := strings.TrimSpace(phone)
	p = strings.ReplaceAll(p, " ", "")
	p = strings.ReplaceAll(p, "-", "")
	switch {
	case strings.HasPrefix(p, "+62"):
		if !strings.HasPrefix(p, "+628") {
			return false
		}
	case strings.HasPrefix(p, "62"):
		if !strings.HasPrefix(p, "628") {
			return false
		}
	case strings.HasPrefix(p, "08"):
		// ok
	default:
		return false
	}
	if len(p) < 10 || len(p) > 16 { // + adds one char; digits 10-15
		return false
	}
	// Count digits only (strip leading +).
	raw := strings.TrimPrefix(p, "+")
	if len(raw) < 10 || len(raw) > 15 {
		return false
	}
	return digitsOnly.MatchString(raw)
}

// ValidatePasswordPolicy requires >=8 chars with at least one letter and one digit.
func ValidatePasswordPolicy(password string) bool {
	if len(password) < minPasswordLen {
		return false
	}
	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}

// GenerateRandomPassword creates a 12-char alphanumeric password satisfying policy.
func GenerateRandomPassword() (string, error) {
	const charset = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const length = 12
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random password: %w", err)
	}
	for i := range buf {
		buf[i] = charset[int(buf[i])%len(charset)]
	}
	// Guarantee policy: force a letter and a digit at deterministic positions
	// derived from random bytes to avoid weakening entropy meaningfully.
	buf[0] = charset[int(buf[0])%26] + 0 // keep letter-ish range; charset starts with letters
	// Ensure at least one digit: map last byte into digit subset of charset.
	digits := "23456789"
	buf[length-1] = digits[int(buf[length-1])%len(digits)]
	out := string(buf)
	if !ValidatePasswordPolicy(out) {
		// Extremely unlikely; fall back to a fixed-policy suffix.
		out = out[:length-2] + "a1"
	}
	return out, nil
}

// CreateUserInput is the validated domain input for MVP-001.1.
type CreateUserInput struct {
	Nama         string
	Email        string
	NoHP         *string
	Peran        Role
	SPPGID       *int64
	SekolahID    *int64
	PasswordAwal *string
}

// ValidateCreate enforces MVP-001.1 rules that do not need the database
// (format, required fields, role/scope shape). Existence checks
// (sppg_id/sekolah_id found, email unique) happen in application service.
func ValidateCreate(in CreateUserInput) error {
	fields := map[string]string{}

	if !ValidateNama(in.Nama) {
		fields["nama"] = "must be 2-120 characters"
	}
	email := NormalizeEmail(in.Email)
	if !ValidateEmail(email) {
		fields["email"] = "invalid email format"
	}
	if !in.Peran.Valid() {
		fields["peran"] = "unknown role"
	}
	if in.NoHP != nil && *in.NoHP != "" && !ValidatePhone(*in.NoHP) {
		fields["no_hp"] = "invalid Indonesian phone number"
	}
	if in.PasswordAwal != nil && *in.PasswordAwal != "" && !ValidatePasswordPolicy(*in.PasswordAwal) {
		fields["password_awal"] = "minimum 8 characters with letters and numbers"
	}

	// Scope shape rules.
	switch in.Peran {
	case RoleAdmin, RolePengawas:
		// sppg_id optional for these roles.
	case RolePICsekolah:
		if in.SPPGID == nil {
			fields["sppg_id"] = "required"
		}
		if in.SekolahID == nil {
			fields["sekolah_id"] = "required for pic_sekolah"
		}
	default:
		if in.SPPGID == nil {
			fields["sppg_id"] = "required"
		}
		if in.SekolahID != nil {
			fields["sekolah_id"] = "must be empty unless role is pic_sekolah"
		}
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// UpdateUserInput is the domain patch for MVP-001.5. Nil pointers mean no change.
type UpdateUserInput struct {
	Nama      *string
	NoHP      *string
	Peran     *Role
	SPPGID    *int64
	SekolahID *int64
	Aktif     *bool
}

// ValidateUpdate checks patch shape.
func ValidateUpdate(in UpdateUserInput) error {
	fields := map[string]string{}
	if in.Nama != nil && !ValidateNama(*in.Nama) {
		fields["nama"] = "must be 2-120 characters"
	}
	if in.NoHP != nil && *in.NoHP != "" && !ValidatePhone(*in.NoHP) {
		fields["no_hp"] = "invalid Indonesian phone number"
	}
	if in.Peran != nil && !in.Peran.Valid() {
		fields["peran"] = "unknown role"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// IsLocked reports whether the account is currently locked.
func (u *User) IsLocked(now time.Time) bool {
	return u.TerkunciSampai != nil && now.Before(*u.TerkunciSampai)
}

// RecordFailedLogin increments the failure counter and locks after 5 attempts.
func (u *User) RecordFailedLogin(now time.Time) {
	u.GagalLogin++
	if u.GagalLogin >= maxFailedAttempts {
		until := now.Add(lockoutDuration)
		u.TerkunciSampai = &until
	}
	u.UpdatedAt = now
}

// RecordSuccessfulLogin resets counters and stamps last login.
func (u *User) RecordSuccessfulLogin(now time.Time) {
	u.GagalLogin = 0
	u.TerkunciSampai = nil
	u.LastLoginAt = &now
	u.UpdatedAt = now
}

// MaxFailedAttempts exposes the lockout threshold for tests/docs.
func MaxFailedAttempts() int { return maxFailedAttempts }

// LockoutDuration exposes the lockout window for tests/docs.
func LockoutDuration() time.Duration { return lockoutDuration }

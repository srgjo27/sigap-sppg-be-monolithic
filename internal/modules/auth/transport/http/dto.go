package http

import (
	"time"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// CreateUserRequest is POST /api/v1/users input (MVP-001.1).
type CreateUserRequest struct {
	Nama         string  `json:"nama" binding:"required"`
	Email        string  `json:"email" binding:"required"`
	NoHP         *string `json:"no_hp"`
	Peran        string  `json:"peran" binding:"required"`
	SPPGID       *int64  `json:"sppg_id"`
	SekolahID    *int64  `json:"sekolah_id"`
	PasswordAwal *string `json:"password_awal"`
}

// UserResponse is the public user shape (password never returned).
type UserResponse struct {
	ID        int64     `json:"id"`
	Nama      string    `json:"nama"`
	Email     string    `json:"email"`
	NoHP      *string   `json:"no_hp,omitempty"`
	Peran     string    `json:"peran"`
	SPPGID    *int64    `json:"sppg_id,omitempty"`
	SekolahID *int64    `json:"sekolah_id,omitempty"`
	Aktif     bool      `json:"aktif"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateUserResponse includes a generated password exactly once.
type CreateUserResponse struct {
	User              UserResponse `json:"user"`
	GeneratedPassword *string      `json:"generated_password,omitempty"`
}

// LoginRequest is POST /api/v1/auth/login input.
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginUserBrief is the nested user in login responses.
type LoginUserBrief struct {
	ID        int64  `json:"id"`
	Nama      string `json:"nama"`
	Peran     string `json:"peran"`
	SPPGID    *int64 `json:"sppg_id,omitempty"`
	SekolahID *int64 `json:"sekolah_id,omitempty"`
}

// LoginResponse is the MVP-001.2 success payload.
type LoginResponse struct {
	AccessToken        string         `json:"access_token"`
	RefreshToken       string         `json:"refresh_token"`
	ExpiresIn          int64          `json:"expires_in"`
	WajibGantiPassword bool           `json:"wajib_ganti_password"`
	User               LoginUserBrief `json:"user"`
}

// RefreshRequest rotates tokens.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshResponse returns rotated tokens.
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// LogoutRequest revokes a refresh token.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// NamedRef is an id+nama reference for profile responses.
type NamedRef struct {
	ID   int64  `json:"id"`
	Nama string `json:"nama"`
}

// MeResponse is GET /api/v1/auth/me (MVP-001.4).
type MeResponse struct {
	ID                 int64     `json:"id"`
	Nama               string    `json:"nama"`
	Email              string    `json:"email"`
	NoHP               *string   `json:"no_hp,omitempty"`
	Peran              string    `json:"peran"`
	SPPG               *NamedRef `json:"sppg,omitempty"`
	Sekolah            *NamedRef `json:"sekolah,omitempty"`
	WajibGantiPassword bool      `json:"wajib_ganti_password"`
	Permissions        []string  `json:"permissions"`
}

// UpdateUserRequest is PATCH /api/v1/users/:id. Pointers detect omission;
// send sppg_id/sekolah_id as 0 to clear, no_hp as "" to clear.
type UpdateUserRequest struct {
	Nama      *string `json:"nama"`
	NoHP      *string `json:"no_hp"`
	Peran     *string `json:"peran"`
	SPPGID    *int64  `json:"sppg_id"`
	SekolahID *int64  `json:"sekolah_id"`
	Aktif     *bool   `json:"aktif"`
}

// ChangePasswordRequest is PUT /api/v1/auth/password.
type ChangePasswordRequest struct {
	PasswordLama string `json:"password_lama" binding:"required"`
	PasswordBaru string `json:"password_baru" binding:"required"`
}

// AuditLogItem is one audit row.
type AuditLogItem struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id,omitempty"`
	Aksi      string    `json:"aksi"`
	Tabel     string    `json:"tabel"`
	RecordID  *int64    `json:"record_id,omitempty"`
	IPAddress *string   `json:"ip_address,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditLogListResponse wraps paginated audit rows.
type AuditLogListResponse struct {
	Data []AuditLogItem `json:"data"`
	Meta PageMeta       `json:"meta"`
}

// PageMeta describes pagination.
type PageMeta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

func toUserResponse(u *domain.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Nama:      u.Nama,
		Email:     u.Email,
		NoHP:      u.NoHP,
		Peran:     string(u.Peran),
		SPPGID:    u.SPPGID,
		SekolahID: u.SekolahID,
		Aktif:     u.Aktif,
		CreatedAt: u.CreatedAt,
	}
}

func toLoginBrief(u *domain.User) LoginUserBrief {
	return LoginUserBrief{
		ID:        u.ID,
		Nama:      u.Nama,
		Peran:     string(u.Peran),
		SPPGID:    u.SPPGID,
		SekolahID: u.SekolahID,
	}
}

func toMeResponse(p *application.Profile) MeResponse {
	u := p.User
	res := MeResponse{
		ID:                 u.ID,
		Nama:               u.Nama,
		Email:              u.Email,
		NoHP:               u.NoHP,
		Peran:              string(u.Peran),
		WajibGantiPassword: u.WajibGantiPassword,
		Permissions:        p.Permissions,
	}
	if p.SPPG != nil {
		res.SPPG = &NamedRef{ID: p.SPPG.ID, Nama: p.SPPG.Nama}
	}
	if p.Sekolah != nil {
		res.Sekolah = &NamedRef{ID: p.Sekolah.ID, Nama: p.Sekolah.Nama}
	}
	return res
}

func toAuditItem(e domain.AuditEntry) AuditLogItem {
	return AuditLogItem{
		ID:        e.ID,
		UserID:    e.UserID,
		Aksi:      e.Aksi,
		Tabel:     e.Tabel,
		RecordID:  e.RecordID,
		IPAddress: e.IPAddress,
		CreatedAt: e.CreatedAt,
	}
}

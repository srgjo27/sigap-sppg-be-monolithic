package domain

import (
	"math"
	"regexp"
	"strings"
	"time"
)

// Allowed jenis_usaha for pemasok.
var allowedJenisUsaha = map[string]bool{
	"UMKM":     true,
	"PT":       true,
	"CV":       true,
	"koperasi": true,
	"petani":   true,
	"lainnya":  true,
}

// Pemasok mirrors the pemasok table in db/schema.sql.
type Pemasok struct {
	ID         int64
	SPPGID     int64
	Nama       string
	JenisUsaha string
	Kontak     string
	NoHP       string
	Alamat     string
	Aktif      bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Kinerja holds supplier performance metrics for the last 90 days.
type Kinerja struct {
	PeriodeHari      int
	JumlahPO         int
	TotalNilaiPO     string // NUMERIC(14,2) as string
	TotalPotongNota  string // NUMERIC(14,2) as string
	PersenDitolak    float64
	PersenTepatWaktu float64
}

// PemasokDetail is the full response for GET /pemasok/{id}.
type PemasokDetail struct {
	Pemasok
	Kinerja *Kinerja
}

// PemasokFilter scopes GET /pemasok.
type PemasokFilter struct {
	Q          *string
	JenisUsaha *string
	Aktif      *bool
	Page       int
	Limit      int
}

// Normalize applies defaults (page 1, limit 20, max 100).
func (f *PemasokFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
}

// Offset returns the SQL offset.
func (f PemasokFilter) Offset() int { return (f.Page - 1) * f.Limit }

// CreatePemasokInput is the validated domain input for POST /pemasok.
type CreatePemasokInput struct {
	Nama       string
	JenisUsaha string
	Kontak     string
	NoHP       string
	Alamat     string
	SPPGID     *int64 // only for admin
}

// UpdatePemasokInput is the validated domain patch for PATCH /pemasok/{id}.
type UpdatePemasokInput struct {
	Nama       *string
	JenisUsaha *string
	Kontak     *string
	NoHP       *string
	Alamat     *string
	Aktif      *bool
	Alasan     *string // required when aktif = false
	SPPGID     *int64  // only for admin
}

// ValidateNoHP checks Indonesian phone format and normalizes to +628...
func ValidateNoHP(noHP string) (string, bool) {
	trimmed := strings.TrimSpace(noHP)
	if trimmed == "" {
		return "", false
	}
	// Accept 08... or +628...
	re := regexp.MustCompile(`^(?:\+62|0)8\d{8,13}$`)
	if !re.MatchString(trimmed) {
		return "", false
	}
	// Normalize to +628...
	if strings.HasPrefix(trimmed, "08") {
		return "+62" + trimmed[1:], true
	}
	if strings.HasPrefix(trimmed, "+628") {
		return trimmed, true
	}
	// Should not reach here due to regex
	return "", false
}

// ValidateCreatePemasok enforces rules without database access.
func ValidateCreatePemasok(in CreatePemasokInput) error {
	fields := map[string]string{}
	nama := strings.TrimSpace(in.Nama)
	if len([]rune(nama)) < 2 || len([]rune(nama)) > 150 {
		fields["nama"] = "must be 2-150 characters"
	}
	if !allowedJenisUsaha[in.JenisUsaha] {
		fields["jenis_usaha"] = "unknown jenis_usaha"
	}
	// NoHP is optional - only validate if provided
	if in.NoHP != "" {
		if _, ok := ValidateNoHP(in.NoHP); !ok {
			fields["no_hp"] = "invalid Indonesian phone format (08... or +628...)"
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// ValidateUpdatePemasok checks patch shape.
func ValidateUpdatePemasok(in UpdatePemasokInput) error {
	fields := map[string]string{}
	if in.Nama != nil {
		nama := strings.TrimSpace(*in.Nama)
		if len([]rune(nama)) < 2 || len([]rune(nama)) > 150 {
			fields["nama"] = "must be 2-150 characters"
		}
	}
	if in.JenisUsaha != nil && !allowedJenisUsaha[*in.JenisUsaha] {
		fields["jenis_usaha"] = "unknown jenis_usaha"
	}
	if in.NoHP != nil {
		if _, ok := ValidateNoHP(*in.NoHP); !ok {
			fields["no_hp"] = "invalid Indonesian phone format (08... or +628...)"
		}
	}
	if in.Aktif != nil && !*in.Aktif {
		if in.Alasan == nil || strings.TrimSpace(*in.Alasan) == "" {
			fields["alasan"] = "required when deactivating pemasok"
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// IsValidJenisUsaha reports whether j is allowed.
func IsValidJenisUsaha(j string) bool {
	return allowedJenisUsaha[j]
}

// RoundToTwoDecimals rounds a float64 to 2 decimal places.
func RoundToTwoDecimals(v float64) float64 {
	return math.Round(v*100) / 100
}

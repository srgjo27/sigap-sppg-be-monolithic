package domain

import (
	"math"
	"strings"
	"time"
)

// Bahan mirrors the bahan table in db/schema.sql.
type Bahan struct {
	ID             int64
	Nama           string
	Kategori       string
	Satuan         string
	GramPerSatuan  float64
	MudahRusak     bool
	SuhuSimpanMaks *float64
	Aktif          bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Allowed bahan kategori (MVP-002.1).
var allowedKategori = map[string]bool{
	"karbohidrat":    true,
	"protein_hewani": true,
	"protein_nabati": true,
	"sayur":          true,
	"buah":           true,
	"bumbu":          true,
	"lainnya":        true,
}

// Allowed bahan satuan (MVP-002.1).
var allowedSatuan = map[string]bool{
	"kg":    true,
	"liter": true,
	"butir": true,
	"ikat":  true,
	"pcs":   true,
}

// IsValidKategori reports whether k is an allowed kategori.
func IsValidKategori(k string) bool { return allowedKategori[k] }

// IsValidSatuan reports whether s is an allowed satuan.
func IsValidSatuan(s string) bool { return allowedSatuan[s] }

// ValidDecimalPlaces reports whether v has at most 2 decimal digits.
func ValidDecimalPlaces(v float64) bool {
	rounded := math.Round(v*100) / 100
	return math.Abs(v-rounded) < 1e-9
}

// CreateBahanInput is the validated domain input for POST /bahan.
type CreateBahanInput struct {
	Nama           string
	Kategori       string
	Satuan         string
	GramPerSatuan  *float64
	MudahRusak     bool
	SuhuSimpanMaks *float64
}

// UpdateBahanInput is the validated domain patch for PATCH /bahan/{id}.
// Nil pointers mean no change.
type UpdateBahanInput struct {
	Nama           *string
	Kategori       *string
	Satuan         *string
	GramPerSatuan  *float64
	MudahRusak     *bool
	SuhuSimpanMaks *float64
	Aktif          *bool
	// ClearSuhu signals explicit clearing of suhu_simpan_maks (JSON null).
	ClearSuhu bool
}

// ValidateBahanNama checks 2-100 characters.
func ValidateBahanNama(nama string) bool {
	n := len([]rune(strings.TrimSpace(nama)))
	return n >= 2 && n <= 100
}

// ResolveGramPerSatuan applies the kg/liter default of 1000 when omitted.
func ResolveGramPerSatuan(satuan string, in *float64) (float64, bool) {
	if in != nil {
		return *in, true
	}
	if satuan == "kg" || satuan == "liter" {
		return 1000, true
	}
	return 0, false
}

// ValidateCreateBahan enforces MVP-002.1 rules without database access.
func ValidateCreateBahan(in CreateBahanInput) error {
	fields := map[string]string{}
	if !ValidateBahanNama(in.Nama) {
		fields["nama"] = "must be 2-100 characters"
	}
	if !IsValidKategori(in.Kategori) {
		fields["kategori"] = "unknown kategori"
	}
	if !IsValidSatuan(in.Satuan) {
		fields["satuan"] = "unknown satuan"
	}
	gram, ok := ResolveGramPerSatuan(in.Satuan, in.GramPerSatuan)
	if !ok {
		fields["gram_per_satuan"] = "required"
	} else {
		if gram <= 0 {
			fields["gram_per_satuan"] = "must be greater than 0"
		} else if !ValidDecimalPlaces(gram) {
			fields["gram_per_satuan"] = "max 2 decimal places"
		}
	}
	if in.MudahRusak && in.SuhuSimpanMaks == nil {
		fields["suhu_simpan_maks"] = "required when mudah_rusak is true"
	}
	if in.SuhuSimpanMaks != nil && !ValidDecimalPlaces(*in.SuhuSimpanMaks) {
		fields["suhu_simpan_maks"] = "max 2 decimal places"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// ValidateUpdateBahan checks patch shape. Cross-field rules that need the
// resulting entity (e.g. mudah_rusak => suhu) are enforced in application
// service after merging.
func ValidateUpdateBahan(in UpdateBahanInput) error {
	fields := map[string]string{}
	if in.Nama != nil && !ValidateBahanNama(*in.Nama) {
		fields["nama"] = "must be 2-100 characters"
	}
	if in.Kategori != nil && !IsValidKategori(*in.Kategori) {
		fields["kategori"] = "unknown kategori"
	}
	if in.Satuan != nil && !IsValidSatuan(*in.Satuan) {
		fields["satuan"] = "unknown satuan"
	}
	if in.GramPerSatuan != nil {
		if *in.GramPerSatuan <= 0 {
			fields["gram_per_satuan"] = "must be greater than 0"
		} else if !ValidDecimalPlaces(*in.GramPerSatuan) {
			fields["gram_per_satuan"] = "max 2 decimal places"
		}
	}
	if in.SuhuSimpanMaks != nil && !ValidDecimalPlaces(*in.SuhuSimpanMaks) {
		fields["suhu_simpan_maks"] = "max 2 decimal places"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// BahanFilter scopes GET /bahan.
type BahanFilter struct {
	Q        *string
	Kategori *string
	Aktif    *bool
	Page     int
	Limit    int
}

// Normalize applies defaults (page 1, limit 20, max 100).
func (f *BahanFilter) Normalize() {
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
func (f BahanFilter) Offset() int { return (f.Page - 1) * f.Limit }

package domain

import (
	"math"
	"time"
)

// KebutuhanItem is one computed row in GET /pengadaan/kebutuhan.
type KebutuhanItem struct {
	BahanID             int64
	Nama                string
	Kategori            string
	Satuan              string
	MudahRusak          bool
	TanggalPakaiPertama string // YYYY-MM-DD
	TotalGram           float64
	QtyKebutuhan        float64
	QtySudahDipesan     float64
	QtySisa             float64
	Rincian             []KebutuhanRincian
}

// KebutuhanRincian is the per-menu breakdown for a bahan.
type KebutuhanRincian struct {
	Tanggal string // YYYY-MM-DD
	MenuID  int64
	Qty     float64
}

// MenuBelumDisetujui is reported in meta for draft menus in range.
type MenuBelumDisetujui struct {
	Tanggal  string // YYYY-MM-DD
	MenuID   int64
	NamaMenu string
}

// POLintasRentang is reported in meta for POs linked to menus both inside and outside range.
type POLintasRentang struct {
	POID    int64
	NomorPO string
	Pemasok string
	ItemIDs []int64
}

// KebutuhanFilter holds query params for GET /pengadaan/kebutuhan.
type KebutuhanFilter struct {
	Dari           string  // YYYY-MM-DD (required)
	Sampai         string  // YYYY-MM-DD (required)
	CadanganPersen float64 // default 0, max 20
	Kategori       *string
}

// KebutuhanResult is the response for GET /pengadaan/kebutuhan.
type KebutuhanResult struct {
	Meta  KebutuhanMeta
	Items []KebutuhanItem
}

// KebutuhanMeta carries metadata about the computation.
type KebutuhanMeta struct {
	Dari               string
	Sampai             string
	CadanganPersen     float64
	JumlahMenu         int
	MenuBelumDisetujui []MenuBelumDisetujui
	POLintasRentang    []POLintasRentang
}

// IsValidKategori checks if kategori is valid (from bahan).
func IsValidKategori(k string) bool {
	allowed := map[string]bool{
		"karbohidrat":    true,
		"protein_hewani": true,
		"protein_nabati": true,
		"sayur":          true,
		"buah":           true,
		"bumbu":          true,
		"lainnya":        true,
	}
	return allowed[k]
}

// ValidateKebutuhanFilter validates the query params.
func ValidateKebutuhanFilter(f KebutuhanFilter) error {
	fields := map[string]string{}
	if f.Dari == "" || f.Sampai == "" {
		fields["dari"] = "required (YYYY-MM-DD)"
		fields["sampai"] = "required (YYYY-MM-DD)"
	} else {
		dari, err1 := time.Parse("2006-01-02", f.Dari)
		sampai, err2 := time.Parse("2006-01-02", f.Sampai)
		if err1 != nil {
			fields["dari"] = "use YYYY-MM-DD"
		}
		if err2 != nil {
			fields["sampai"] = "use YYYY-MM-DD"
		}
		if err1 == nil && err2 == nil {
			if sampai.Before(dari) {
				fields["sampai"] = "must not be before dari"
			}
			days := sampai.Sub(dari).Hours() / 24
			if days > 14 {
				fields["sampai"] = "range must not exceed 14 days"
			}
		}
	}
	if f.CadanganPersen < 0 || f.CadanganPersen > 20 {
		fields["cadangan_persen"] = "must be between 0 and 20"
	}
	if f.Kategori != nil {
		if !IsValidKategori(*f.Kategori) {
			fields["kategori"] = "unknown kategori"
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// ComputeKebutuhan computes qty per menu (same as MVP-002.6).
// qty = ceil((gram_per_porsi * target_porsi * (1 + cadangan/100)) / gram_per_satuan)
// For butir and pcs, result is integer; others max 2 decimals.
func ComputeKebutuhan(gramPerPorsi float64, satuan string, gramPerSatuan float64, targetPorsi int, cadanganPersen float64) (totalGram float64, qty float64) {
	totalGram = gramPerPorsi * float64(targetPorsi) * (1 + cadanganPersen/100)
	if gramPerSatuan <= 0 {
		return totalGram, 0
	}
	raw := totalGram / gramPerSatuan
	if satuan == "butir" || satuan == "pcs" {
		qty = math.Ceil(raw)
	} else {
		qty = math.Ceil(raw*100) / 100 // round up to 2 decimals
	}
	return totalGram, qty
}

// NormalizeSatuanQty ensures integer for butir/pcs, 2 decimals for others.
func NormalizeSatuanQty(satuan string, qty float64) float64 {
	if satuan == "butir" || satuan == "pcs" {
		return math.Ceil(qty)
	}
	return math.Round(qty*100) / 100
}

// ParseDate parses YYYY-MM-DD string to time.Time.
func ParseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// FormatDate formats time.Time to YYYY-MM-DD.
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// ValidateSatuanForQty validates qty matches satuan rules.
func ValidateSatuanForQty(satuan string, qty float64) error {
	if satuan == "butir" || satuan == "pcs" {
		if qty != math.Trunc(qty) {
			return &ValidationError{Fields: map[string]string{"qty_pesan": "must be integer for butir/pcs"}}
		}
	} else {
		// max 2 decimals
		if math.Abs(qty*100-math.Trunc(qty*100)) > 1e-9 {
			return &ValidationError{Fields: map[string]string{"qty_pesan": "max 2 decimal places"}}
		}
	}
	return nil
}

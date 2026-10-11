package domain

import (
	"math"
	"strings"
	"time"
)

// Menu mirrors the menu table in db/schema.sql.
type Menu struct {
	ID            int64
	SPPGID        int64
	Tanggal       time.Time
	NamaMenu      string
	EnergiKkal    float64
	ProteinG      float64
	KarbohidratG  float64
	LemakG        float64
	TargetPorsi   int
	Catatan       *string
	DibuatOleh    int64
	Status        string
	DisetujuiOleh *int64
	DisetujuiAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// MenuBahan mirrors one menu_bahan row plus denormalized nama for responses.
type MenuBahan struct {
	ID           int64
	MenuID       int64
	BahanID      int64
	BahanNama    string
	GramPerPorsi float64
}

// MenuStatusDraf is the only status assigned at creation (MVP-002.2).
const MenuStatusDraf = "draf"

// MenuStatusDisetujui marks an approved menu (MVP-002.5).
const MenuStatusDisetujui = "disetujui"

// IsValidMenuStatus reports whether s is a known menu status.
func IsValidMenuStatus(s string) bool {
	return s == MenuStatusDraf || s == MenuStatusDisetujui
}

// MenuBahanInput is one composition item in POST /menus.
type MenuBahanInput struct {
	BahanID      int64
	GramPerPorsi float64
}

// CreateMenuInput is the validated domain input for POST /menus.
type CreateMenuInput struct {
	Tanggal      time.Time
	NamaMenu     string
	EnergiKkal   float64
	ProteinG     float64
	KarbohidratG float64
	LemakG       float64
	TargetPorsi  *int
	Catatan      *string
	Items        []MenuBahanInput
}

// ValidateMenuNama checks 3-200 characters.
func ValidateMenuNama(nama string) bool {
	n := len([]rune(strings.TrimSpace(nama)))
	return n >= 3 && n <= 200
}

func truncDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// ValidateCreateMenu enforces MVP-002.2 rules that do not need the database.
// today is the current date used to reject past tanggal.
func ValidateCreateMenu(in CreateMenuInput, today time.Time) error {
	fields := map[string]string{}
	if truncDate(in.Tanggal).Before(truncDate(today)) {
		fields["tanggal"] = "must not be in the past"
	}
	if !ValidateMenuNama(in.NamaMenu) {
		fields["nama_menu"] = "must be 3-200 characters"
	}
	if in.EnergiKkal <= 0 {
		fields["energi_kkal"] = "must be greater than 0"
	} else if in.EnergiKkal > 2000 {
		fields["energi_kkal"] = "max 2000"
	} else if !ValidDecimalPlaces(in.EnergiKkal) {
		fields["energi_kkal"] = "max 2 decimal places"
	}
	if in.ProteinG <= 0 {
		fields["protein_g"] = "must be greater than 0"
	} else if in.ProteinG > 300 {
		fields["protein_g"] = "max 300"
	} else if !ValidDecimalPlaces(in.ProteinG) {
		fields["protein_g"] = "max 2 decimal places"
	}
	if in.KarbohidratG <= 0 {
		fields["karbohidrat_g"] = "must be greater than 0"
	} else if in.KarbohidratG > 300 {
		fields["karbohidrat_g"] = "max 300"
	} else if !ValidDecimalPlaces(in.KarbohidratG) {
		fields["karbohidrat_g"] = "max 2 decimal places"
	}
	if in.LemakG <= 0 {
		fields["lemak_g"] = "must be greater than 0"
	} else if in.LemakG > 300 {
		fields["lemak_g"] = "max 300"
	} else if !ValidDecimalPlaces(in.LemakG) {
		fields["lemak_g"] = "max 2 decimal places"
	}
	if in.TargetPorsi != nil && *in.TargetPorsi <= 0 {
		fields["target_porsi"] = "must be greater than 0"
	}
	if len(in.Items) == 0 {
		fields["bahan"] = "at least 1 item is required"
	} else {
		seen := map[int64]bool{}
		dup := false
		for i, it := range in.Items {
			if seen[it.BahanID] {
				dup = true
				continue
			}
			seen[it.BahanID] = true
			if it.BahanID <= 0 {
				fields["bahan"] = "invalid bahan_id"
				_ = i
				continue
			}
			if it.GramPerPorsi <= 0 || it.GramPerPorsi > 1000 {
				fields["bahan"] = "gram_per_porsi must be > 0 and max 1000"
				continue
			}
			if !ValidDecimalPlaces(it.GramPerPorsi) {
				fields["bahan"] = "gram_per_porsi max 2 decimal places"
				continue
			}
		}
		if dup {
			return &ValidationError{Fields: map[string]string{"bahan": "duplicate bahan_id in composition"}}
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// MenuFilter scopes GET /menus (MVP-002.3).
type MenuFilter struct {
	Dari   time.Time
	Sampai time.Time
	Status *string
	SPPGID *int64
}

// ValidateMenuFilter enforces required range, order, and max 31 days inclusive.
func ValidateMenuFilter(f MenuFilter) error {
	fields := map[string]string{}
	if f.Dari.IsZero() {
		fields["dari"] = "required (YYYY-MM-DD)"
	}
	if f.Sampai.IsZero() {
		fields["sampai"] = "required (YYYY-MM-DD)"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	dari := truncDate(f.Dari)
	sampai := truncDate(f.Sampai)
	if sampai.Before(dari) {
		return &ValidationError{Fields: map[string]string{"sampai": "must not be before dari"}}
	}
	inclusiveDays := int(sampai.Sub(dari).Hours()/24) + 1
	if inclusiveDays > 31 {
		return &ValidationError{Fields: map[string]string{"sampai": "max 31 days range"}}
	}
	if f.Status != nil && !IsValidMenuStatus(*f.Status) {
		return &ValidationError{Fields: map[string]string{"status": "unknown status"}}
	}
	return nil
}

// MissingDates returns YYYY-MM-DD dates in [dari, sampai] without a menu.
func MissingDates(dari, sampai time.Time, present map[string]bool) []string {
	var out []string
	for d := truncDate(dari); !d.After(truncDate(sampai)); d = d.Add(24 * time.Hour) {
		key := d.Format("2006-01-02")
		if !present[key] {
			out = append(out, key)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// UpdateMenuInput is the domain patch for PATCH /menus/{id} (MVP-002.4).
// Nil pointers mean no change. Tanggal is intentionally absent: it cannot
// be changed; use copy (MVP-002.7) then delete.
type UpdateMenuInput struct {
	NamaMenu     *string
	EnergiKkal   *float64
	ProteinG     *float64
	KarbohidratG *float64
	LemakG       *float64
	TargetPorsi  *int
	Catatan      *string
	ClearCatatan bool
	Items        *[]MenuBahanInput
}

func checkGiziField(v *float64, max float64, name string, fields map[string]string) {
	if v == nil {
		return
	}
	if *v <= 0 {
		fields[name] = "must be greater than 0"
	} else if *v > max {
		fields[name] = "value too large"
	} else if !ValidDecimalPlaces(*v) {
		fields[name] = "max 2 decimal places"
	}
}

// ValidateUpdateMenu enforces MVP-002.4 rules without database access.
func ValidateUpdateMenu(in UpdateMenuInput) error {
	fields := map[string]string{}
	if in.NamaMenu != nil && !ValidateMenuNama(*in.NamaMenu) {
		fields["nama_menu"] = "must be 3-200 characters"
	}
	checkGiziField(in.EnergiKkal, 2000, "energi_kkal", fields)
	checkGiziField(in.ProteinG, 300, "protein_g", fields)
	checkGiziField(in.KarbohidratG, 300, "karbohidrat_g", fields)
	checkGiziField(in.LemakG, 300, "lemak_g", fields)
	if in.TargetPorsi != nil && *in.TargetPorsi <= 0 {
		fields["target_porsi"] = "must be greater than 0"
	}
	if in.Items != nil {
		items := *in.Items
		if len(items) == 0 {
			fields["bahan"] = "at least 1 item is required"
		} else {
			seen := map[int64]bool{}
			dup := false
			for _, it := range items {
				if seen[it.BahanID] {
					dup = true
					continue
				}
				seen[it.BahanID] = true
				if it.BahanID <= 0 {
					fields["bahan"] = "invalid bahan_id"
					continue
				}
				if it.GramPerPorsi <= 0 || it.GramPerPorsi > 1000 {
					fields["bahan"] = "gram_per_porsi must be > 0 and max 1000"
					continue
				}
				if !ValidDecimalPlaces(it.GramPerPorsi) {
					fields["bahan"] = "gram_per_porsi max 2 decimal places"
					continue
				}
			}
			if dup {
				return &ValidationError{Fields: map[string]string{"bahan": "duplicate bahan_id in composition"}}
			}
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// MenuComplete reports whether a draft is approvable (MVP-002.5):
// at least 1 composition item and complete per-portion nutrition.
func MenuComplete(m *Menu, items []MenuBahan) bool {
	if m == nil || len(items) == 0 {
		return false
	}
	return m.EnergiKkal > 0 && m.ProteinG > 0 && m.KarbohidratG > 0 && m.LemakG > 0
}

// ValidateRevertReason enforces the required revert alasan (MVP-002.5).
func ValidateRevertReason(alasan string) error {
	if strings.TrimSpace(alasan) == "" {
		return &ValidationError{Fields: map[string]string{"alasan": "required"}}
	}
	return nil
}

// ValidateKebutuhanParams validates simulation params (MVP-002.6).
func ValidateKebutuhanParams(target *int, cadangan *float64) error {
	fields := map[string]string{}
	if target != nil && *target <= 0 {
		fields["target_porsi"] = "must be greater than 0"
	}
	if cadangan != nil {
		if *cadangan < 0 || *cadangan > 20 {
			fields["cadangan_persen"] = "must be between 0 and 20"
		} else if !ValidDecimalPlaces(*cadangan) {
			fields["cadangan_persen"] = "max 2 decimal places"
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// IsCountableUnit reports whether qty must be whole numbers (butir, pcs).
func IsCountableUnit(satuan string) bool {
	return satuan == "butir" || satuan == "pcs"
}

// Round2 rounds half-up to 2 decimals for display totals.
func Round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// Ceil2 rounds up to 2 decimals (qty in kg/liter/ikat).
func Ceil2(v float64) float64 {
	return math.Ceil(v*100-1e-9) / 100
}

// ComputeKebutuhan applies the MVP-002.6 formula:
// total_gram = gram_per_porsi × target × (1 + cadangan/100),
// qty = total_gram ÷ gram_per_satuan, rounded up (2dp; whole for butir/pcs).
func ComputeKebutuhan(gramPerPorsi float64, satuan string, gramPerSatuan float64, target int, cadanganPersen float64) (totalGram, qty float64) {
	totalGram = Round2(gramPerPorsi * float64(target) * (1 + cadanganPersen/100))
	if gramPerSatuan <= 0 {
		return totalGram, 0
	}
	raw := totalGram / gramPerSatuan
	if IsCountableUnit(satuan) {
		qty = math.Ceil(raw - 1e-9)
	} else {
		qty = Ceil2(raw)
	}
	return totalGram, qty
}

// CopyMenuInput is the validated domain input for POST /menus/{id}/copy.
type CopyMenuInput struct {
	TanggalTujuan []time.Time
}

// ValidateCopyMenuInput enforces MVP-002.7 rules without database access:
// 1–7 target dates, no duplicates, none in the past.
func ValidateCopyMenuInput(in CopyMenuInput, today time.Time) error {
	if len(in.TanggalTujuan) == 0 {
		return &ValidationError{Fields: map[string]string{"tanggal_tujuan": "at least 1 date is required"}}
	}
	if len(in.TanggalTujuan) > 7 {
		return &ValidationError{Fields: map[string]string{"tanggal_tujuan": "max 7 dates"}}
	}
	seen := map[string]bool{}
	for _, t := range in.TanggalTujuan {
		key := truncDate(t).Format("2006-01-02")
		if seen[key] {
			return &ValidationError{Fields: map[string]string{"tanggal_tujuan": "duplicate date " + key}}
		}
		seen[key] = true
		if truncDate(t).Before(truncDate(today)) {
			return &ValidationError{Fields: map[string]string{"tanggal_tujuan": "must not be in the past"}}
		}
	}
	return nil
}

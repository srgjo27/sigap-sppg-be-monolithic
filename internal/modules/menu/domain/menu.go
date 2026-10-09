package domain

import (
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

package domain

import (
	"testing"
)

func TestValidateNoHP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		valid    bool
	}{
		{"081234567890", "+6281234567890", true},
		{"+6281234567890", "+6281234567890", true},
		{"08123456789", "+628123456789", true},
		{"+628123456789", "+628123456789", true},
		{"081234567890123", "+6281234567890123", true},
		{"", "", false},
		{"0812345678", "+62812345678", true}, // 10 digits (valid)
		{"0812345678901234", "", false},      // too long (16 digits)
		{"0211234567", "", false},            // not mobile
		{"+62812345678901234", "", false},
		{"abc", "", false},
	}

	for _, tc := range tests {
		result, ok := ValidateNoHP(tc.input)
		if ok != tc.valid {
			t.Errorf("ValidateNoHP(%q): expected valid=%v, got %v", tc.input, tc.valid, ok)
		}
		if ok && result != tc.expected {
			t.Errorf("ValidateNoHP(%q): expected %q, got %q", tc.input, tc.expected, result)
		}
	}
}

func TestValidateCreatePemasok(t *testing.T) {
	tests := []struct {
		name     string
		input    CreatePemasokInput
		wantErr  bool
		errField string
	}{
		{
			name: "valid input",
			input: CreatePemasokInput{
				Nama:       "PT ABC",
				JenisUsaha: "PT",
				NoHP:       "081234567890",
			},
			wantErr: false,
		},
		{
			name: "nama too short",
			input: CreatePemasokInput{
				Nama:       "A",
				JenisUsaha: "PT",
				NoHP:       "081234567890",
			},
			wantErr:  true,
			errField: "nama",
		},
		{
			name: "nama too long",
			input: CreatePemasokInput{
				Nama:       string(make([]rune, 151)),
				JenisUsaha: "PT",
				NoHP:       "081234567890",
			},
			wantErr:  true,
			errField: "nama",
		},
		{
			name: "invalid jenis_usaha",
			input: CreatePemasokInput{
				Nama:       "PT ABC",
				JenisUsaha: "INVALID",
				NoHP:       "081234567890",
			},
			wantErr:  true,
			errField: "jenis_usaha",
		},
		{
			name: "invalid no_hp",
			input: CreatePemasokInput{
				Nama:       "PT ABC",
				JenisUsaha: "PT",
				NoHP:       "0211234567",
			},
			wantErr:  true,
			errField: "no_hp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCreatePemasok(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if ve, ok := err.(*ValidationError); ok {
					if _, exists := ve.Fields[tc.errField]; !exists {
						t.Errorf("expected field %q in error, got %v", tc.errField, ve.Fields)
					}
				} else {
					t.Errorf("expected ValidationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateUpdatePemasok(t *testing.T) {
	tests := []struct {
		name     string
		input    UpdatePemasokInput
		wantErr  bool
		errField string
	}{
		{
			name: "valid update nama",
			input: UpdatePemasokInput{
				Nama: strPtr("New Name"),
			},
			wantErr: false,
		},
		{
			name: "valid update aktif with alasan",
			input: UpdatePemasokInput{
				Aktif:  boolPtr(false),
				Alasan: strPtr("Not reliable"),
			},
			wantErr: false,
		},
		{
			name: "deactivate without alasan",
			input: UpdatePemasokInput{
				Aktif: boolPtr(false),
			},
			wantErr:  true,
			errField: "alasan",
		},
		{
			name: "invalid no_hp",
			input: UpdatePemasokInput{
				NoHP: strPtr("0211234567"),
			},
			wantErr:  true,
			errField: "no_hp",
		},
		{
			name: "invalid jenis_usaha",
			input: UpdatePemasokInput{
				JenisUsaha: strPtr("INVALID"),
			},
			wantErr:  true,
			errField: "jenis_usaha",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateUpdatePemasok(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if ve, ok := err.(*ValidationError); ok {
					if _, exists := ve.Fields[tc.errField]; !exists {
						t.Errorf("expected field %q in error, got %v", tc.errField, ve.Fields)
					}
				} else {
					t.Errorf("expected ValidationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestComputeKebutuhan(t *testing.T) {
	tests := []struct {
		name              string
		gramPerPorsi      float64
		satuan            string
		gramPerSatuan     float64
		targetPorsi       int
		cadanganPersen    float64
		expectedTotalGram float64
		expectedQty       float64
	}{
		{
			name:              "kg satuan, no cadangan",
			gramPerPorsi:      100,
			satuan:            "kg",
			gramPerSatuan:     1000,
			targetPorsi:       100,
			cadanganPersen:    0,
			expectedTotalGram: 10000,
			expectedQty:       10.0,
		},
		{
			name:              "kg satuan, with 10% cadangan",
			gramPerPorsi:      100,
			satuan:            "kg",
			gramPerSatuan:     1000,
			targetPorsi:       100,
			cadanganPersen:    10,
			expectedTotalGram: 11000,
			expectedQty:       11.0,
		},
		{
			name:              "butir satuan, integer result",
			gramPerPorsi:      50,
			satuan:            "butir",
			gramPerSatuan:     60,
			targetPorsi:       100,
			cadanganPersen:    0,
			expectedTotalGram: 5000,
			expectedQty:       84.0, // ceil(5000/60) = ceil(83.33) = 84
		},
		{
			name:              "pcs satuan, integer result",
			gramPerPorsi:      100,
			satuan:            "pcs",
			gramPerSatuan:     1000,
			targetPorsi:       50,
			cadanganPersen:    0,
			expectedTotalGram: 5000,
			expectedQty:       5.0,
		},
		{
			name:              "liter satuan, 2 decimals",
			gramPerPorsi:      200,
			satuan:            "liter",
			gramPerSatuan:     1000,
			targetPorsi:       50,
			cadanganPersen:    5,
			expectedTotalGram: 10500,
			expectedQty:       10.5, // ceil(10500/1000 * 100)/100 = 10.5
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			totalGram, qty := ComputeKebutuhan(tc.gramPerPorsi, tc.satuan, tc.gramPerSatuan, tc.targetPorsi, tc.cadanganPersen)
			if totalGram != tc.expectedTotalGram {
				t.Errorf("totalGram: expected %v, got %v", tc.expectedTotalGram, totalGram)
			}
			if qty != tc.expectedQty {
				t.Errorf("qty: expected %v, got %v", tc.expectedQty, qty)
			}
		})
	}
}

func TestNormalizeSatuanQty(t *testing.T) {
	tests := []struct {
		name     string
		satuan   string
		input    float64
		expected float64
	}{
		{"butir integer", "butir", 10.0, 10.0},
		{"butir ceil", "butir", 10.1, 11.0},
		{"pcs integer", "pcs", 5.0, 5.0},
		{"pcs ceil", "pcs", 5.1, 6.0},
		{"kg 2 decimals", "kg", 10.123, 10.12},
		{"liter 2 decimals", "liter", 5.126, 5.13},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizeSatuanQty(tc.satuan, tc.input)
			if result != tc.expected {
				t.Errorf("NormalizeSatuanQty(%q, %v): expected %v, got %v", tc.satuan, tc.input, tc.expected, result)
			}
		})
	}
}

func TestIsValidKategori(t *testing.T) {
	valid := []string{"karbohidrat", "protein_hewani", "protein_nabati", "sayur", "buah", "bumbu", "lainnya"}
	invalid := []string{"invalid", "daging", "sayuran", ""}

	for _, k := range valid {
		if !IsValidKategori(k) {
			t.Errorf("IsValidKategori(%q): expected true", k)
		}
	}
	for _, k := range invalid {
		if IsValidKategori(k) {
			t.Errorf("IsValidKategori(%q): expected false", k)
		}
	}
}

func TestValidateKebutuhanFilter(t *testing.T) {
	tests := []struct {
		name     string
		filter   KebutuhanFilter
		wantErr  bool
		errField string
	}{
		{
			name: "valid filter",
			filter: KebutuhanFilter{
				Dari:   "2024-01-01",
				Sampai: "2024-01-10",
			},
			wantErr: false,
		},
		{
			name: "missing dari",
			filter: KebutuhanFilter{
				Sampai: "2024-01-10",
			},
			wantErr:  true,
			errField: "dari",
		},
		{
			name: "missing sampai",
			filter: KebutuhanFilter{
				Dari: "2024-01-01",
			},
			wantErr:  true,
			errField: "sampai",
		},
		{
			name: "sampai before dari",
			filter: KebutuhanFilter{
				Dari:   "2024-01-10",
				Sampai: "2024-01-01",
			},
			wantErr:  true,
			errField: "sampai",
		},
		{
			name: "range > 14 days",
			filter: KebutuhanFilter{
				Dari:   "2024-01-01",
				Sampai: "2024-01-20",
			},
			wantErr:  true,
			errField: "sampai",
		},
		{
			name: "cadangan_persen negative",
			filter: KebutuhanFilter{
				Dari:           "2024-01-01",
				Sampai:         "2024-01-10",
				CadanganPersen: -1,
			},
			wantErr:  true,
			errField: "cadangan_persen",
		},
		{
			name: "cadangan_persen > 20",
			filter: KebutuhanFilter{
				Dari:           "2024-01-01",
				Sampai:         "2024-01-10",
				CadanganPersen: 25,
			},
			wantErr:  true,
			errField: "cadangan_persen",
		},
		{
			name: "invalid kategori",
			filter: KebutuhanFilter{
				Dari:     "2024-01-01",
				Sampai:   "2024-01-10",
				Kategori: strPtr("invalid"),
			},
			wantErr:  true,
			errField: "kategori",
		},
		{
			name: "valid kategori",
			filter: KebutuhanFilter{
				Dari:     "2024-01-01",
				Sampai:   "2024-01-10",
				Kategori: strPtr("sayur"),
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateKebutuhanFilter(tc.filter)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if ve, ok := err.(*ValidationError); ok {
					if _, exists := ve.Fields[tc.errField]; !exists {
						t.Errorf("expected field %q in error, got %v", tc.errField, ve.Fields)
					}
				} else {
					t.Errorf("expected ValidationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

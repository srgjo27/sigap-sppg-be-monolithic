package domain

import "testing"

func ptrFloat(v float64) *float64 { return &v }

func TestValidateCreateBahan(t *testing.T) {
	suhu := 4.0
	tests := []struct {
		name    string
		in      CreateBahanInput
		wantErr bool
		field   string
	}{
		{"valid kg default", CreateBahanInput{Nama: "Beras", Kategori: "karbohidrat", Satuan: "kg"}, false, ""},
		{"valid butir explicit", CreateBahanInput{Nama: "Telur", Kategori: "protein_hewani", Satuan: "butir", GramPerSatuan: ptrFloat(60)}, false, ""},
		{"nama too short", CreateBahanInput{Nama: "A", Kategori: "sayur", Satuan: "ikat", GramPerSatuan: ptrFloat(100)}, true, "nama"},
		{"bad kategori", CreateBahanInput{Nama: "Bayam", Kategori: "soda", Satuan: "ikat", GramPerSatuan: ptrFloat(100)}, true, "kategori"},
		{"bad satuan", CreateBahanInput{Nama: "Bayam", Kategori: "sayur", Satuan: "gram", GramPerSatuan: ptrFloat(100)}, true, "satuan"},
		{"missing gram non-kg", CreateBahanInput{Nama: "Telur", Kategori: "protein_hewani", Satuan: "butir"}, true, "gram_per_satuan"},
		{"zero gram", CreateBahanInput{Nama: "Beras", Kategori: "karbohidrat", Satuan: "kg", GramPerSatuan: ptrFloat(0)}, true, "gram_per_satuan"},
		{"perishable needs suhu", CreateBahanInput{Nama: "Ayam", Kategori: "protein_hewani", Satuan: "kg", MudahRusak: true}, true, "suhu_simpan_maks"},
		{"perishable with suhu ok", CreateBahanInput{Nama: "Ayam", Kategori: "protein_hewani", Satuan: "kg", MudahRusak: true, SuhuSimpanMaks: &suhu}, false, ""},
		{"too many decimals", CreateBahanInput{Nama: "Beras", Kategori: "karbohidrat", Satuan: "kg", GramPerSatuan: ptrFloat(1.234)}, true, "gram_per_satuan"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCreateBahan(tc.in)
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr {
				ve, ok := err.(*ValidationError)
				if !ok {
					t.Fatalf("expected ValidationError, got %T %v", err, err)
				}
				if _, ok := ve.Fields[tc.field]; !ok {
					t.Fatalf("expected field %q in %v", tc.field, ve.Fields)
				}
			}
		})
	}
}

func TestResolveGramPerSatuanDefault(t *testing.T) {
	if v, ok := ResolveGramPerSatuan("kg", nil); !ok || v != 1000 {
		t.Fatalf("kg default = %v %v", v, ok)
	}
	if v, ok := ResolveGramPerSatuan("liter", nil); !ok || v != 1000 {
		t.Fatalf("liter default = %v %v", v, ok)
	}
	if _, ok := ResolveGramPerSatuan("butir", nil); ok {
		t.Fatal("butir must require explicit gram")
	}
}

func TestValidateUpdateBahanShape(t *testing.T) {
	bad := "x"
	if err := ValidateUpdateBahan(UpdateBahanInput{Nama: &bad}); err == nil {
		t.Fatal("expected nama error")
	}
	badKat := "soda"
	if err := ValidateUpdateBahan(UpdateBahanInput{Kategori: &badKat}); err == nil {
		t.Fatal("expected kategori error")
	}
}

package domain

import (
	"testing"
	"time"
)

func TestValidateCreateMenu(t *testing.T) {
	today := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	future := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	past := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	target := 100
	validItems := []MenuBahanInput{{BahanID: 1, GramPerPorsi: 50}}
	valid := func() CreateMenuInput {
		return CreateMenuInput{
			Tanggal: future, NamaMenu: "Nasi Ayam Bergizi",
			EnergiKkal: 500, ProteinG: 20, KarbohidratG: 60, LemakG: 15,
			TargetPorsi: &target, Items: validItems,
		}
	}
	if err := ValidateCreateMenu(valid(), today); err != nil {
		t.Fatalf("valid: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*CreateMenuInput)
		field  string
	}{
		{"past date", func(in *CreateMenuInput) { in.Tanggal = past }, "tanggal"},
		{"nama short", func(in *CreateMenuInput) { in.NamaMenu = "AB" }, "nama_menu"},
		{"zero energi", func(in *CreateMenuInput) { in.EnergiKkal = 0 }, "energi_kkal"},
		{"energi over max", func(in *CreateMenuInput) { in.EnergiKkal = 2001 }, "energi_kkal"},
		{"protein over max", func(in *CreateMenuInput) { in.ProteinG = 301 }, "protein_g"},
		{"empty bahan", func(in *CreateMenuInput) { in.Items = nil }, "bahan"},
		{"gram zero", func(in *CreateMenuInput) { in.Items = []MenuBahanInput{{BahanID: 1, GramPerPorsi: 0}} }, "bahan"},
		{"gram over max", func(in *CreateMenuInput) { in.Items = []MenuBahanInput{{BahanID: 1, GramPerPorsi: 1001}} }, "bahan"},
		{"bad target", func(in *CreateMenuInput) { z := 0; in.TargetPorsi = &z }, "target_porsi"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := valid()
			tc.mutate(&in)
			err := ValidateCreateMenu(in, today)
			if err == nil {
				t.Fatal("expected error")
			}
			ve, ok := err.(*ValidationError)
			if !ok {
				t.Fatalf("expected ValidationError got %T", err)
			}
			if _, ok := ve.Fields[tc.field]; !ok {
				t.Fatalf("expected field %q in %v", tc.field, ve.Fields)
			}
		})
	}
}

func TestValidateCreateMenuDuplicateBahan(t *testing.T) {
	today := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	in := CreateMenuInput{
		Tanggal: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), NamaMenu: "Nasi Ayam",
		EnergiKkal: 500, ProteinG: 20, KarbohidratG: 60, LemakG: 15,
		Items: []MenuBahanInput{{BahanID: 1, GramPerPorsi: 50}, {BahanID: 1, GramPerPorsi: 30}},
	}
	err := ValidateCreateMenu(in, today)
	if err == nil {
		t.Fatal("expected duplicate error")
	}
	ve, ok := err.(*ValidationError)
	if !ok || ve.Fields["bahan"] == "" {
		t.Fatalf("expected bahan duplicate field, got %v", err)
	}
}

package domain

import (
	"testing"
	"time"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func strPtr(s string) *string { return &s }

func TestValidateMenuFilter(t *testing.T) {
	valid := MenuFilter{Dari: date("2026-10-01"), Sampai: date("2026-10-31")}
	if err := ValidateMenuFilter(valid); err != nil {
		t.Fatalf("31-day inclusive range must pass: %v", err)
	}
	if err := ValidateMenuFilter(MenuFilter{Sampai: date("2026-10-02")}); err == nil {
		t.Fatal("missing dari must fail")
	}
	if err := ValidateMenuFilter(MenuFilter{Dari: date("2026-10-05"), Sampai: date("2026-10-01")}); err == nil {
		t.Fatal("reversed range must fail")
	} else if ve, ok := err.(*ValidationError); !ok || ve.Fields["sampai"] == "" {
		t.Fatalf("expected sampai field, got %v", err)
	}
	if err := ValidateMenuFilter(MenuFilter{Dari: date("2026-10-01"), Sampai: date("2026-11-05")}); err == nil {
		t.Fatal(">31 days must fail")
	}
	bad := "goreng"
	if err := ValidateMenuFilter(MenuFilter{Dari: date("2026-10-01"), Sampai: date("2026-10-02"), Status: &bad}); err == nil {
		t.Fatal("bad status must fail")
	}
	okStatus := MenuStatusDraf
	if err := ValidateMenuFilter(MenuFilter{Dari: date("2026-10-01"), Sampai: date("2026-10-02"), Status: &okStatus}); err != nil {
		t.Fatalf("draf status must pass: %v", err)
	}
}

func TestMissingDates(t *testing.T) {
	got := MissingDates(date("2026-10-01"), date("2026-10-03"), map[string]bool{"2026-10-02": true})
	if len(got) != 2 || got[0] != "2026-10-01" || got[1] != "2026-10-03" {
		t.Fatalf("missing = %v", got)
	}
	if full := MissingDates(date("2026-10-01"), date("2026-10-01"), map[string]bool{"2026-10-01": true}); len(full) != 0 {
		t.Fatalf("full range must have no missing, got %v", full)
	}
}

func TestValidateUpdateMenu(t *testing.T) {
	if err := ValidateUpdateMenu(UpdateMenuInput{}); err != nil {
		t.Fatalf("empty patch must pass: %v", err)
	}
	badNama := "AB"
	if err := ValidateUpdateMenu(UpdateMenuInput{NamaMenu: &badNama}); err == nil {
		t.Fatal("short nama must fail")
	}
	badEnergi := 2500.0
	if err := ValidateUpdateMenu(UpdateMenuInput{EnergiKkal: &badEnergi}); err == nil {
		t.Fatal("energi over max must fail")
	}
	badTarget := 0
	if err := ValidateUpdateMenu(UpdateMenuInput{TargetPorsi: &badTarget}); err == nil {
		t.Fatal("zero target must fail")
	}
	empty := []MenuBahanInput{}
	if err := ValidateUpdateMenu(UpdateMenuInput{Items: &empty}); err == nil {
		t.Fatal("empty bahan must fail")
	}
	dup := []MenuBahanInput{{BahanID: 1, GramPerPorsi: 50}, {BahanID: 1, GramPerPorsi: 30}}
	if err := ValidateUpdateMenu(UpdateMenuInput{Items: &dup}); err == nil {
		t.Fatal("duplicate bahan must fail")
	}
	badGram := []MenuBahanInput{{BahanID: 1, GramPerPorsi: 5000}}
	if err := ValidateUpdateMenu(UpdateMenuInput{Items: &badGram}); err == nil {
		t.Fatal("gram over max must fail")
	}
	goodNama := "Nasi Gudeg Lengkap"
	goodGizi := 550.0
	goodItems := []MenuBahanInput{{BahanID: 7, GramPerPorsi: 120}}
	if err := ValidateUpdateMenu(UpdateMenuInput{NamaMenu: &goodNama, EnergiKkal: &goodGizi, Items: &goodItems}); err != nil {
		t.Fatalf("valid patch must pass: %v", err)
	}
	_ = strPtr
}

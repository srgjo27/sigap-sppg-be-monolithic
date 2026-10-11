package domain

import "testing"

func TestComputeKebutuhanConversions(t *testing.T) {
	// kg: 100g/porsi × 250 porsi = 25000g → 25 kg exact.
	total, qty := ComputeKebutuhan(100, "kg", 1000, 250, 0)
	if total != 25000 || qty != 25 {
		t.Fatalf("kg: total=%v qty=%v", total, qty)
	}
	// 10% buffer: 25000 × 1.1 = 27500g → 27.5 kg.
	total, qty = ComputeKebutuhan(100, "kg", 1000, 250, 10)
	if total != 27500 || qty != 27.5 {
		t.Fatalf("buffer: total=%v qty=%v", total, qty)
	}
	// butir bulbung: telur 60g/satuan, 50g/porsi × 250 = 12500g → 208.34 → 209 butir.
	total, qty = ComputeKebutuhan(50, "butir", 60, 250, 0)
	if total != 12500 || qty != 209 {
		t.Fatalf("butir: total=%v qty=%v", total, qty)
	}
	// Ceil to 2dp: 100g × 3 porsi = 300g / 1000 = 0.3 kg exact; with odd grams
	// 33.33g × 3 = 99.99g → 0.1 kg after ceil (0.09999 → 0.1).
	total, qty = ComputeKebutuhan(33.33, "kg", 1000, 3, 0)
	if total != 99.99 || qty != 0.1 {
		t.Fatalf("ceil2: total=%v qty=%v", total, qty)
	}
	// Ceil-up case: total 100.001 → 100.01? Round2 gives 100.0; craft: 33.335
	// is rejected by validation, so use gram 10, target 7, cadangan 0.15:
	// 10×7×1.0015 = 70.105 → 70.11 (round) → 0.07011kg → ceil 0.08? No:
	// 70.11/1000 = 0.07011 → ceil2 = 0.08? 0.07011*100=7.011 → ceil 8 → 0.08.
	total, qty = ComputeKebutuhan(10, "liter", 1000, 7, 0.15)
	if total != 70.11 || qty != 0.08 {
		t.Fatalf("ceil-up: total=%v qty=%v", total, qty)
	}
	// pcs countable: 5g × 10 = 50g / 25g per pcs = 2 exact.
	if _, qty := ComputeKebutuhan(5, "pcs", 25, 10, 0); qty != 2 {
		t.Fatalf("pcs exact: qty=%v", qty)
	}
}

func TestIsCountableUnit(t *testing.T) {
	for _, u := range []string{"butir", "pcs"} {
		if !IsCountableUnit(u) {
			t.Fatalf("%s must be countable", u)
		}
	}
	for _, u := range []string{"kg", "liter", "ikat"} {
		if IsCountableUnit(u) {
			t.Fatalf("%s must not be countable", u)
		}
	}
}

func TestValidateKebutuhanParams(t *testing.T) {
	badTarget := 0
	if err := ValidateKebutuhanParams(&badTarget, nil); err == nil {
		t.Fatal("target 0 must fail")
	}
	neg := -1.0
	if err := ValidateKebutuhanParams(nil, &neg); err == nil {
		t.Fatal("negative cadangan must fail")
	}
	over := 21.0
	if err := ValidateKebutuhanParams(nil, &over); err == nil {
		t.Fatal("cadangan >20 must fail")
	}
	many := 1.234
	if err := ValidateKebutuhanParams(nil, &many); err == nil {
		t.Fatal("3 decimals must fail")
	}
	ok := 10.0
	if err := ValidateKebutuhanParams(nil, &ok); err != nil {
		t.Fatalf("valid cadangan: %v", err)
	}
	if err := ValidateKebutuhanParams(nil, nil); err != nil {
		t.Fatalf("empty params: %v", err)
	}
}

func TestValidateRevertReason(t *testing.T) {
	if err := ValidateRevertReason(""); err == nil {
		t.Fatal("empty alasan must fail")
	}
	if err := ValidateRevertReason("   "); err == nil {
		t.Fatal("blank alasan must fail")
	}
	if err := ValidateRevertReason("gizi kurang, perbaiki"); err != nil {
		t.Fatalf("valid alasan: %v", err)
	}
}

func TestMenuComplete(t *testing.T) {
	m := &Menu{EnergiKkal: 500, ProteinG: 20, KarbohidratG: 60, LemakG: 15}
	if MenuComplete(m, nil) {
		t.Fatal("no items must be incomplete")
	}
	if MenuComplete(m, []MenuBahan{{BahanID: 1, GramPerPorsi: 50}}) != true {
		t.Fatal("complete menu expected")
	}
	empty := &Menu{}
	if MenuComplete(empty, []MenuBahan{{BahanID: 1, GramPerPorsi: 50}}) {
		t.Fatal("zero gizi must be incomplete")
	}
}

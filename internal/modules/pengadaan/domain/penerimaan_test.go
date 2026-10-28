package domain

import (
	"strings"
	"testing"
	"time"
)

func TestValidateCreatePenerimaanInput(t *testing.T) {
	now := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	nowStr := now.Format("2006-01-02")

	valid := CreatePenerimaanInput{
		ClientRef: "550e8400-e29b-41d4-a716-446655440000",
		Items: []PenerimaanItem{
			{
				POItemID:        1,
				QtyDiterima:     10,
				QtyDitolak:      0,
				Kondisi:         KondisiBaik,
				FotoBerkasID:    100,
				NilaiPotongNota: 0,
			},
		},
	}
	if err := ValidateCreatePenerimaanInput(valid); err != nil {
		t.Fatalf("valid input should pass, got %v", err)
	}

	// Too few items.
	if err := ValidateCreatePenerimaanInput(CreatePenerimaanInput{Items: []PenerimaanItem{}}); err == nil {
		t.Fatal("expected error for empty items")
	}

	// Duplicate po_item_id.
	dup := CreatePenerimaanInput{
		Items: []PenerimaanItem{
			{POItemID: 1, QtyDiterima: 5, QtyDitolak: 0, Kondisi: KondisiBaik, FotoBerkasID: 1},
			{POItemID: 1, QtyDiterima: 5, QtyDitolak: 0, Kondisi: KondisiBaik, FotoBerkasID: 2},
		},
	}
	if err := ValidateCreatePenerimaanInput(dup); err == nil {
		t.Fatal("expected error for duplicate po_item_id")
	}

	// rusak but qty_diterima > 0.
	rusak := CreatePenerimaanInput{
		Items: []PenerimaanItem{
			{POItemID: 1, QtyDiterima: 5, QtyDitolak: 0, Kondisi: KondisiRusak, FotoBerkasID: 1},
		},
	}
	if err := ValidateCreatePenerimaanInput(rusak); err == nil {
		t.Fatal("expected error for rusak with qty_diterima > 0")
	}

	// kurang without catatan.
	kurang := CreatePenerimaanInput{
		Items: []PenerimaanItem{
			{POItemID: 1, QtyDiterima: 5, QtyDitolak: 0, Kondisi: KondisiKurang, FotoBerkasID: 1},
		},
	}
	if err := ValidateCreatePenerimaanInput(kurang); err == nil {
		t.Fatal("expected error for kurang without catatan")
	}

	// kurang with catatan -> valid.
	kurangCatatan := CreatePenerimaanInput{
		Items: []PenerimaanItem{
			{POItemID: 1, QtyDiterima: 5, QtyDitolak: 0, Kondisi: KondisiKurang, FotoBerkasID: 1, Catatan: strPtr("pakai wadah baru")},
		},
	}
	if err := ValidateCreatePenerimaanInput(kurangCatatan); err != nil {
		t.Fatalf("kurang with catatan should pass, got %v", err)
	}

	// qty_ditolak > 0 without alasan_tolak.
	tolak := CreatePenerimaanInput{
		Items: []PenerimaanItem{
			{POItemID: 1, QtyDiterima: 5, QtyDitolak: 2, Kondisi: KondisiBaik, FotoBerkasID: 1},
		},
	}
	if err := ValidateCreatePenerimaanInput(tolak); err == nil {
		t.Fatal("expected error for qty_ditolak > 0 without alasan_tolak")
	}

	// qty_ditolak > 0 with alasan_tolak -> valid.
	tolakAlasan := CreatePenerimaanInput{
		Items: []PenerimaanItem{
			{POItemID: 1, QtyDiterima: 5, QtyDitolak: 2, Kondisi: KondisiBaik, FotoBerkasID: 1, AlasanTolak: strPtr("bahan rusak saat kirim")},
		},
	}
	if err := ValidateCreatePenerimaanInput(tolakAlasan); err != nil {
		t.Fatalf("qty_ditolak with alasan_tolak should pass, got %v", err)
	}

	// Invalid kondisi.
	badKondisi := CreatePenerimaanInput{
		Items: []PenerimaanItem{
			{POItemID: 1, QtyDiterima: 5, QtyDitolak: 0, Kondisi: "baiknya", FotoBerkasID: 1},
		},
	}
	if err := ValidateCreatePenerimaanInput(badKondisi); err == nil {
		t.Fatal("expected error for invalid kondisi")
	}

	// Invalid tgl_kedaluwarsa format.
	badDate := CreatePenerimaanInput{
		Items: []PenerimaanItem{
			{POItemID: 1, QtyDiterima: 5, QtyDitolak: 0, Kondisi: KondisiBaik, FotoBerkasID: 1, TglKedaluwarsa: strPtr("27-10-2026")},
		},
	}
	if err := ValidateCreatePenerimaanInput(badDate); err == nil {
		t.Fatal("expected error for invalid tgl_kedaluwarsa format")
	}

	// Valid tgl_kedaluwarsa.
	goodDate := CreatePenerimaanInput{
		Items: []PenerimaanItem{
			{POItemID: 1, QtyDiterima: 5, QtyDitolak: 0, Kondisi: KondisiBaik, FotoBerkasID: 1, TglKedaluwarsa: strPtr(nowStr)},
		},
	}
	if err := ValidateCreatePenerimaanInput(goodDate); err != nil {
		t.Fatalf("valid tgl_kedaluwarsa should pass, got %v", err)
	}

	// Missing foto_berkas_id.
	noFoto := CreatePenerimaanInput{
		Items: []PenerimaanItem{
			{POItemID: 1, QtyDiterima: 5, QtyDitolak: 0, Kondisi: KondisiBaik},
		},
	}
	if err := ValidateCreatePenerimaanInput(noFoto); err == nil {
		t.Fatal("expected error for missing foto_berkas_id")
	}

	// 51 items (over max).
	manyItems := CreatePenerimaanInput{
		ClientRef: "550e8400-e29b-41d4-a716-446655440001",
		Items:     make([]PenerimaanItem, 51),
	}
	for i := range manyItems.Items {
		manyItems.Items[i] = PenerimaanItem{POItemID: int64(i), QtyDiterima: 1, QtyDitolak: 0, Kondisi: KondisiBaik, FotoBerkasID: int64(i)}
	}
	if err := ValidateCreatePenerimaanInput(manyItems); err == nil {
		t.Fatal("expected error for 51 items")
	}

	// 50 items (at max) -> valid.
	maxItems := CreatePenerimaanInput{
		ClientRef: "550e8400-e29b-41d4-a716-446655440002",
		Items:     make([]PenerimaanItem, 50),
	}
	for i := range maxItems.Items {
		maxItems.Items[i] = PenerimaanItem{POItemID: int64(i + 1), QtyDiterima: 1, QtyDitolak: 0, Kondisi: KondisiBaik, FotoBerkasID: int64(i + 1)}
	}
	if err := ValidateCreatePenerimaanInput(maxItems); err != nil {
		t.Fatalf("50 items should pass, got %v", err)
	}
}

func TestValidatePenerimaanFilter(t *testing.T) {
	// Valid empty filter.
	if err := ValidatePenerimaanFilter(PenerimaanFilter{}); err != nil {
		t.Fatalf("empty filter should pass, got %v", err)
	}

	// Valid kondisi.
	f := PenerimaanFilter{Kondisi: strPtr(KondisiBaik)}
	if err := ValidatePenerimaanFilter(f); err != nil {
		t.Fatalf("valid kondisi should pass, got %v", err)
	}

	// Invalid kondisi.
	f = PenerimaanFilter{Kondisi: strPtr("busuk")}
	if err := ValidatePenerimaanFilter(f); err == nil {
		t.Fatal("expected error for invalid kondisi")
	}

	// Valid date range (<= 31 days).
	dari := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sampai := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	f = PenerimaanFilter{Dari: &dari, Sampai: &sampai}
	if err := ValidatePenerimaanFilter(f); err != nil {
		t.Fatalf("31-day range should pass, got %v", err)
	}

	// Invalid date range (> 31 days): Oct 1 to Nov 2 = 32 days.
	sampai = time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	f = PenerimaanFilter{Dari: &dari, Sampai: &sampai}
	if err := ValidatePenerimaanFilter(f); err == nil {
		t.Fatal("expected error for range > 31 days")
	}

	// Tanggal terbalik.
	sampai = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	f = PenerimaanFilter{Dari: &dari, Sampai: &sampai}
	if err := ValidatePenerimaanFilter(f); err == nil {
		t.Fatal("expected error for reversed date range")
	}
}

func TestValidateKoreksiPenerimaanInput(t *testing.T) {
	// Valid: alasan + qty_diterima.
	valid := KoreksiPenerimaanInput{
		Alasan:      strPtr("perbaikan qty karena salah ketik"),
		QtyDiterima: ptrFloat64(10),
	}
	if err := ValidateKoreksiPenerimaanInput(valid); err != nil {
		t.Fatalf("valid input should pass, got %v", err)
	}

	// Valid: alasan only with other fields nil (but hasField will be false... wait)
	// Actually alasan alone without any other field should fail because hasField is false.
	alasanOnly := KoreksiPenerimaanInput{
		Alasan: strPtr("perbaikan qty karena salah ketik"),
	}
	if err := ValidateKoreksiPenerimaanInput(alasanOnly); err == nil {
		t.Fatal("expected error for alasan only without any correctable field")
	}

	// Missing alasan.
	noAlasan := KoreksiPenerimaanInput{
		QtyDiterima: ptrFloat64(10),
	}
	if err := ValidateKoreksiPenerimaanInput(noAlasan); err == nil {
		t.Fatal("expected error for missing alasan")
	}

	// Alasan too short (< 10 chars).
	shortAlasan := KoreksiPenerimaanInput{
		Alasan:      strPtr("fix"),
		QtyDiterima: ptrFloat64(10),
	}
	if err := ValidateKoreksiPenerimaanInput(shortAlasan); err == nil {
		t.Fatal("expected error for short alasan")
	}

	// Alasan too long (> 500 chars).
	longAlasan := KoreksiPenerimaanInput{
		Alasan:      strPtr(strings.Repeat("a", 501)),
		QtyDiterima: ptrFloat64(10),
	}
	if err := ValidateKoreksiPenerimaanInput(longAlasan); err == nil {
		t.Fatal("expected error for long alasan")
	}

	// qty_diterima negative.
	negQty := KoreksiPenerimaanInput{
		Alasan:      strPtr("perbaikan qty"),
		QtyDiterima: ptrFloat64(-5),
	}
	if err := ValidateKoreksiPenerimaanInput(negQty); err == nil {
		t.Fatal("expected error for negative qty_diterima")
	}

	// qty_ditolak > 0 without alasan_tolak.
	noAlasanTolak := KoreksiPenerimaanInput{
		Alasan:     strPtr("perbaikan qty"),
		QtyDitolak: ptrFloat64(2),
	}
	if err := ValidateKoreksiPenerimaanInput(noAlasanTolak); err == nil {
		t.Fatal("expected error for qty_ditolak without alasan_tolak")
	}

	// qty_ditolak > 0 with alasan_tolak -> valid.
	withAlasanTolak := KoreksiPenerimaanInput{
		Alasan:      strPtr("perbaikan qty"),
		QtyDitolak:  ptrFloat64(2),
		AlasanTolak: strPtr("barang rusak saat kirim"),
	}
	if err := ValidateKoreksiPenerimaanInput(withAlasanTolak); err != nil {
		t.Fatalf("valid input with alasan_tolak should pass, got %v", err)
	}

	// kondisi rusak with qty_diterima > 0.
	rusakDiterima := KoreksiPenerimaanInput{
		Alasan:      strPtr("perbaikan kondisi"),
		Kondisi:     strPtr(KondisiRusak),
		QtyDiterima: ptrFloat64(5),
	}
	if err := ValidateKoreksiPenerimaanInput(rusakDiterima); err == nil {
		t.Fatal("expected error for rusak with qty_diterima > 0")
	}

	// kondisi kurang without catatan.
	kurangNoCatatan := KoreksiPenerimaanInput{
		Alasan:  strPtr("perbaikan kondisi"),
		Kondisi: strPtr(KondisiKurang),
	}
	if err := ValidateKoreksiPenerimaanInput(kurangNoCatatan); err == nil {
		t.Fatal("expected error for kurang without catatan")
	}

	// Invalid kondisi.
	badKondisi := KoreksiPenerimaanInput{
		Alasan:  strPtr("perbaikan"),
		Kondisi: strPtr("busuk"),
	}
	if err := ValidateKoreksiPenerimaanInput(badKondisi); err == nil {
		t.Fatal("expected error for invalid kondisi")
	}

	// Invalid tgl_kedaluwarsa format.
	badDate := KoreksiPenerimaanInput{
		Alasan:         strPtr("perbaikan tanggal"),
		TglKedaluwarsa: strPtr("27-10-2026"),
	}
	if err := ValidateKoreksiPenerimaanInput(badDate); err == nil {
		t.Fatal("expected error for invalid tgl_kedaluwarsa format")
	}
}

// ptrFloat64 returns a pointer to a float64.
func ptrFloat64(v float64) *float64 {
	return &v
}

package domain

import (
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

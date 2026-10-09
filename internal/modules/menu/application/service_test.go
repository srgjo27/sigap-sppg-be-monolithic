package application

import (
	"context"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/infrastructure"
)

func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int) *int           { return &v }
func ptrInt64(v int64) *int64     { return &v }
func ptrBool(v bool) *bool        { return &v }
func ptrStr(v string) *string     { return &v }

func newFixture(now time.Time) (*Service, *infrastructure.MemoryStores) {
	stores := infrastructure.NewMemoryStores()
	cap := 100
	stores.SeedSPPG(1, &cap, 250)
	svc := New(Deps{Bahan: stores, Menus: stores, SPPG: stores, Sekolah: stores, Audits: stores, Clock: func() time.Time { return now }})
	return svc, stores
}

func adminClaims() *authdomain.Claims {
	return &authdomain.Claims{UserID: 1, Role: authdomain.RoleAdmin, SPPGID: ptrInt64(1)}
}

func giziClaims() *authdomain.Claims {
	return &authdomain.Claims{UserID: 2, Role: authdomain.RoleAhliGizi, SPPGID: ptrInt64(1)}
}

func TestCreateBahanDuplicateCaseInsensitive(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, _ := newFixture(now)
	ctx := context.Background()
	if _, err := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Beras", Kategori: "karbohidrat", Satuan: "kg"}, nil); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "beras", Kategori: "karbohidrat", Satuan: "kg"}, nil); err != domain.ErrBahanExists {
		t.Fatalf("expected ErrBahanExists, got %v", err)
	}
}

func TestCreateBahanPerishableAndRole(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, stores := newFixture(now)
	ctx := context.Background()
	if _, err := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Ayam", Kategori: "protein_hewani", Satuan: "kg", MudahRusak: true}, nil); err == nil {
		t.Fatal("perishable without suhu must fail")
	}
	// Forbidden role.
	akuntan := &authdomain.Claims{UserID: 9, Role: authdomain.RoleAkuntan, SPPGID: ptrInt64(1)}
	if _, err := svc.CreateBahan(ctx, akuntan, domain.CreateBahanInput{Nama: "Gula", Kategori: "lainnya", Satuan: "kg"}, nil); err != domain.ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if stores.AuditCount() != 0 {
		t.Fatalf("failed creates must not audit, got %d", stores.AuditCount())
	}
	suhu := 4.0
	b, err := svc.CreateBahan(ctx, adminClaims(), domain.CreateBahanInput{Nama: "Ayam", Kategori: "protein_hewani", Satuan: "kg", MudahRusak: true, SuhuSimpanMaks: &suhu}, nil)
	if err != nil {
		t.Fatalf("create perishable: %v", err)
	}
	if b.GramPerSatuan != 1000 || !b.Aktif {
		t.Fatalf("defaults wrong: %+v", b)
	}
	if stores.AuditCount() != 1 {
		t.Fatalf("expected 1 audit, got %d", stores.AuditCount())
	}
}

func TestDeactivatedBahanHiddenFromNewMenu(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, _ := newFixture(now)
	ctx := context.Background()
	b, err := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Telur", Kategori: "protein_hewani", Satuan: "butir", GramPerSatuan: ptrFloat(60)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Old menu with the bahan succeeds.
	tomorrow := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	res, err := svc.CreateMenu(ctx, giziClaims(), domain.CreateMenuInput{
		Tanggal: tomorrow, NamaMenu: "Nasi Telur",
		EnergiKkal: 500, ProteinG: 20, KarbohidratG: 60, LemakG: 15,
		Items: []domain.MenuBahanInput{{BahanID: b.ID, GramPerPorsi: 50}},
	}, nil)
	if err != nil {
		t.Fatalf("old menu: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].BahanNama != "Telur" {
		t.Fatalf("old menu items wrong: %+v", res.Items)
	}
	// Deactivate.
	f := false
	if _, err := svc.UpdateBahan(ctx, giziClaims(), b.ID, domain.UpdateBahanInput{Aktif: &f}, nil); err != nil {
		t.Fatal(err)
	}
	// New menu with inactive bahan is rejected with 422-style sentinel.
	dayAfter := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	if _, err := svc.CreateMenu(ctx, giziClaims(), domain.CreateMenuInput{
		Tanggal: dayAfter, NamaMenu: "Nasi Telur Lagi",
		EnergiKkal: 500, ProteinG: 20, KarbohidratG: 60, LemakG: 15,
		Items: []domain.MenuBahanInput{{BahanID: b.ID, GramPerPorsi: 50}},
	}, nil); err != domain.ErrBahanInactive {
		t.Fatalf("expected ErrBahanInactive, got %v", err)
	}
}

func TestCreateMenuDefaultsWarningsAtomicity(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, stores := newFixture(now)
	ctx := context.Background()
	beras, err := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Beras", Kategori: "karbohidrat", Satuan: "kg"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	telur, err := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Telur", Kategori: "protein_hewani", Satuan: "butir", GramPerSatuan: ptrFloat(60)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tomorrow := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	res, err := svc.CreateMenu(ctx, giziClaims(), domain.CreateMenuInput{
		Tanggal: tomorrow, NamaMenu: "Nasi Ayam Bergizi",
		EnergiKkal: 500, ProteinG: 20, KarbohidratG: 60, LemakG: 15,
		Items: []domain.MenuBahanInput{{BahanID: beras.ID, GramPerPorsi: 100}, {BahanID: telur.ID, GramPerPorsi: 50}},
	}, nil)
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}
	if res.Menu.Status != domain.MenuStatusDraf {
		t.Fatalf("status = %q", res.Menu.Status)
	}
	if res.Menu.TargetPorsi != 250 {
		t.Fatalf("default target = %d, want 250", res.Menu.TargetPorsi)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("expected capacity warning, got %v", res.Warnings)
	}
	if len(res.Items) != 2 || res.Menu.DibuatOleh != 2 {
		t.Fatalf("items/owner wrong: %+v", res)
	}
	// Second menu same date same SPPG -> 409, no partial data.
	if _, err := svc.CreateMenu(ctx, giziClaims(), domain.CreateMenuInput{
		Tanggal: tomorrow, NamaMenu: "Menu Kedua",
		EnergiKkal: 400, ProteinG: 15, KarbohidratG: 50, LemakG: 10,
		TargetPorsi: ptrInt(50),
		Items:       []domain.MenuBahanInput{{BahanID: beras.ID, GramPerPorsi: 80}},
	}, nil); err != domain.ErrMenuExists {
		t.Fatalf("expected ErrMenuExists, got %v", err)
	}
	// Unknown bahan -> 404 and nothing persisted for that date.
	other := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	if _, err := svc.CreateMenu(ctx, giziClaims(), domain.CreateMenuInput{
		Tanggal: other, NamaMenu: "Menu Gagal",
		EnergiKkal: 400, ProteinG: 15, KarbohidratG: 50, LemakG: 10,
		Items: []domain.MenuBahanInput{{BahanID: 9999, GramPerPorsi: 10}},
	}, nil); err != domain.ErrBahanNotFound {
		t.Fatalf("expected ErrBahanNotFound, got %v", err)
	}
	if exists, _ := stores.ExistsBySPPGDate(ctx, 1, other); exists {
		t.Fatal("failed menu must not leave partial data")
	}
	// Non-ahli_gizi forbidden.
	if _, err := svc.CreateMenu(ctx, adminClaims(), domain.CreateMenuInput{
		Tanggal: other, NamaMenu: "Menu Admin",
		EnergiKkal: 400, ProteinG: 15, KarbohidratG: 50, LemakG: 10,
		Items: []domain.MenuBahanInput{{BahanID: beras.ID, GramPerPorsi: 10}},
	}, nil); err != domain.ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

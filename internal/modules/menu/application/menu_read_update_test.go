package application

import (
	"context"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
)

func mustCreateMenu(t *testing.T, svc *Service, ctx context.Context, actor *authdomain.Claims, tanggal time.Time, nama string, bahanIDs ...int64) int64 {
	t.Helper()
	items := make([]domain.MenuBahanInput, 0, len(bahanIDs))
	for _, id := range bahanIDs {
		items = append(items, domain.MenuBahanInput{BahanID: id, GramPerPorsi: 50})
	}
	res, err := svc.CreateMenu(ctx, actor, domain.CreateMenuInput{
		Tanggal: tanggal, NamaMenu: nama,
		EnergiKkal: 500, ProteinG: 20, KarbohidratG: 60, LemakG: 15,
		Items: items,
	}, nil)
	if err != nil {
		t.Fatalf("mustCreateMenu: %v", err)
	}
	return res.Menu.ID
}

func mustCreateBahan(t *testing.T, svc *Service, ctx context.Context, nama string) int64 {
	t.Helper()
	b, err := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: nama, Kategori: "karbohidrat", Satuan: "kg"}, nil)
	if err != nil {
		t.Fatalf("mustCreateBahan: %v", err)
	}
	return b.ID
}

func TestListMenusScopingAndMissing(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, stores := newFixture(now)
	stores.SeedSPPG(2, nil, 50)
	ctx := context.Background()
	beras := mustCreateBahan(t, svc, ctx, "Beras")
	mustCreateMenu(t, svc, ctx, giziClaims(), time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), "Menu Sepuluh", beras)
	mustCreateMenu(t, svc, ctx, giziClaims(), time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), "Menu Duabelas", beras)

	res, err := svc.ListMenus(ctx, giziClaims(), domain.MenuFilter{Dari: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Sampai: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if res.Total != 2 || len(res.Missing) != 1 || res.Missing[0] != "2026-10-11" {
		t.Fatalf("total/missing wrong: %+v", res)
	}
	if !res.Items[0].Tanggal.Before(res.Items[1].Tanggal) {
		t.Fatal("must be tanggal ASC")
	}
	// Cross-SPPG filter by non-privileged is 404.
	other := int64(2)
	if _, err := svc.ListMenus(ctx, giziClaims(), domain.MenuFilter{Dari: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Sampai: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), SPPGID: &other}); err != domain.ErrNotFound {
		t.Fatalf("cross-sppg must be 404, got %v", err)
	}
	// Admin may filter another SPPG (empty, not error).
	admin := &authdomain.Claims{UserID: 1, Role: authdomain.RoleAdmin}
	ares, err := svc.ListMenus(ctx, admin, domain.MenuFilter{Dari: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Sampai: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), SPPGID: &other})
	if err != nil || ares.Total != 0 {
		t.Fatalf("admin cross filter: %v %+v", err, ares)
	}
	// Range over 31 days is 400.
	if _, err := svc.ListMenus(ctx, giziClaims(), domain.MenuFilter{Dari: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Sampai: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)}); err == nil {
		t.Fatal("range >31 must fail")
	}
}

func TestGetMenuDetailAndPICRedaction(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, stores := newFixture(now)
	stores.SeedSekolah(20, 1)
	ctx := context.Background()
	beras := mustCreateBahan(t, svc, ctx, "Beras")
	id := mustCreateMenu(t, svc, ctx, giziClaims(), time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), "Menu Detail", beras)

	det, err := svc.GetMenu(ctx, giziClaims(), id)
	if err != nil || len(det.Items) != 1 || det.Items[0].GramPerPorsi != 50 {
		t.Fatalf("detail: %v %+v", err, det)
	}
	// Cross-SPPG detail is 404.
	otherGizi := &authdomain.Claims{UserID: 5, Role: authdomain.RoleAhliGizi, SPPGID: ptrInt64(2)}
	if _, err := svc.GetMenu(ctx, otherGizi, id); err != domain.ErrMenuNotFound {
		t.Fatalf("cross detail must be 404, got %v", err)
	}
	// pic_sekolah sees draft? No: 404 until approved.
	pic := &authdomain.Claims{UserID: 7, Role: authdomain.RolePICsekolah, SPPGID: ptrInt64(1), SekolahID: ptrInt64(20)}
	if _, err := svc.GetMenu(ctx, pic, id); err != domain.ErrMenuNotFound {
		t.Fatalf("pic draft must be 404, got %v", err)
	}
	// Approve directly in store, then pic sees it without grams.
	m, _ := stores.FindMenuByID(ctx, id)
	m.Status = domain.MenuStatusDisetujui
	if _, _, err := stores.UpdateMenu(ctx, m, nil); err != nil {
		t.Fatal(err)
	}
	pdet, err := svc.GetMenu(ctx, pic, id)
	if err != nil || pdet.Redacted != true || pdet.Items[0].GramPerPorsi != 0 {
		t.Fatalf("pic approved redacted: %v %+v", err, pdet)
	}
	// pic list only approved.
	lres, err := svc.ListMenus(ctx, pic, domain.MenuFilter{Dari: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Sampai: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)})
	if err != nil || lres.Total != 1 {
		t.Fatalf("pic list: %v %+v", err, lres)
	}
}

func TestUpdateMenuDraftOnlyAndAtomic(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, stores := newFixture(now)
	ctx := context.Background()
	beras := mustCreateBahan(t, svc, ctx, "Beras")
	telurID := func() int64 {
		b, err := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Telur", Kategori: "protein_hewani", Satuan: "butir", GramPerSatuan: ptrFloat(60)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return b.ID
	}()
	id := mustCreateMenu(t, svc, ctx, giziClaims(), time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), "Menu Update", beras)

	newNama := "Menu Update Baru"
	newTarget := 120
	ures, err := svc.UpdateMenu(ctx, giziClaims(), id, domain.UpdateMenuInput{
		NamaMenu: &newNama, TargetPorsi: &newTarget,
		Items: &[]domain.MenuBahanInput{{BahanID: beras, GramPerPorsi: 80}, {BahanID: telurID, GramPerPorsi: 40}},
	}, nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if ures.Menu.NamaMenu != newNama || ures.Menu.TargetPorsi != 120 || len(ures.Items) != 2 {
		t.Fatalf("updated wrong: %+v", ures.Menu)
	}
	if len(ures.Warnings) != 1 {
		t.Fatalf("expected capacity warning, got %v", ures.Warnings)
	}
	// Failed bahan replace leaves old composition intact (atomic).
	if _, err := svc.UpdateMenu(ctx, giziClaims(), id, domain.UpdateMenuInput{
		Items: &[]domain.MenuBahanInput{{BahanID: 9999, GramPerPorsi: 10}},
	}, nil); err != domain.ErrBahanNotFound {
		t.Fatalf("bad bahan must be 404, got %v", err)
	}
	det, _ := svc.GetMenu(ctx, giziClaims(), id)
	if len(det.Items) != 2 || det.Menu.NamaMenu != newNama {
		t.Fatalf("failed update must not change data: %+v", det)
	}
	// Non-gizi forbidden.
	if _, err := svc.UpdateMenu(ctx, adminClaims(), id, domain.UpdateMenuInput{NamaMenu: &newNama}, nil); err != domain.ErrForbidden {
		t.Fatalf("non-gizi must be 403, got %v", err)
	}
	// Approved menu is 409.
	m, _ := stores.FindMenuByID(ctx, id)
	m.Status = domain.MenuStatusDisetujui
	_, _, _ = stores.UpdateMenu(ctx, m, nil)
	if _, err := svc.UpdateMenu(ctx, giziClaims(), id, domain.UpdateMenuInput{NamaMenu: &newNama}, nil); err != domain.ErrMenuApproved {
		t.Fatalf("approved must be 409, got %v", err)
	}
	// Used menu is 409 (reset to draf first, mark used).
	m.Status = domain.MenuStatusDraf
	_, _, _ = stores.UpdateMenu(ctx, m, nil)
	stores.MarkMenuUsed(id)
	if _, err := svc.UpdateMenu(ctx, giziClaims(), id, domain.UpdateMenuInput{NamaMenu: &newNama}, nil); err != domain.ErrMenuInUse {
		t.Fatalf("used must be 409, got %v", err)
	}
}

func TestDeleteMenuDraftOnly(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, stores := newFixture(now)
	ctx := context.Background()
	beras := mustCreateBahan(t, svc, ctx, "Beras")
	id := mustCreateMenu(t, svc, ctx, giziClaims(), time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), "Menu Hapus", beras)

	before := stores.AuditCount()
	if err := svc.DeleteMenu(ctx, giziClaims(), id, nil); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetMenu(ctx, giziClaims(), id); err != domain.ErrMenuNotFound {
		t.Fatalf("deleted must be 404, got %v", err)
	}
	if stores.AuditCount() != before+1 {
		t.Fatal("delete must audit")
	}
	// Deleting again is 404.
	if err := svc.DeleteMenu(ctx, giziClaims(), id, nil); err != domain.ErrMenuNotFound {
		t.Fatalf("second delete must be 404, got %v", err)
	}
	// Approved cannot be deleted.
	id2 := mustCreateMenu(t, svc, ctx, giziClaims(), time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC), "Menu Approve", beras)
	m, _ := stores.FindMenuByID(ctx, id2)
	m.Status = domain.MenuStatusDisetujui
	_, _, _ = stores.UpdateMenu(ctx, m, nil)
	if err := svc.DeleteMenu(ctx, giziClaims(), id2, nil); err != domain.ErrMenuApproved {
		t.Fatalf("approved delete must be 409, got %v", err)
	}
}

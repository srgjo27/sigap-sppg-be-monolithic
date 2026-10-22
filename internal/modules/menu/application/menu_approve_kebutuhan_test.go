package application

import (
	"context"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/infrastructure"
)

func kepalaClaims() *authdomain.Claims {
	return &authdomain.Claims{UserID: 3, Role: authdomain.RoleKepalaSPPG, SPPGID: ptrInt64(1)}
}

func akuntanClaims() *authdomain.Claims {
	return &authdomain.Claims{UserID: 4, Role: authdomain.RoleAkuntan, SPPGID: ptrInt64(1)}
}

// newApprovalFixture wires notifications too (unlike legacy newFixture).
func newApprovalFixture(now time.Time) (*Service, *infrastructure.MemoryStores) {
	stores := infrastructure.NewMemoryStores()
	cap := 100
	stores.SeedSPPG(1, &cap, 250)
	svc := New(Deps{Bahan: stores, Menus: stores, SPPG: stores, Sekolah: stores, Audits: stores, Notifs: stores, Clock: func() time.Time { return now }})
	return svc, stores
}

func seedDraftMenu(t *testing.T, svc *Service, ctx context.Context, tanggal time.Time, nama string, bahanIDs ...int64) int64 {
	t.Helper()
	items := make([]domain.MenuBahanInput, 0, len(bahanIDs))
	for _, id := range bahanIDs {
		items = append(items, domain.MenuBahanInput{BahanID: id, GramPerPorsi: 50})
	}
	res, err := svc.CreateMenu(ctx, giziClaims(), domain.CreateMenuInput{
		Tanggal: tanggal, NamaMenu: nama,
		EnergiKkal: 500, ProteinG: 20, KarbohidratG: 60, LemakG: 15,
		Items: items,
	}, nil)
	if err != nil {
		t.Fatalf("seed menu: %v", err)
	}
	return res.Menu.ID
}

func TestApproveMenuHappyPath(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, stores := newApprovalFixture(now)
	ctx := context.Background()
	beras, err := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Beras", Kategori: "karbohidrat", Satuan: "kg"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Notification recipients.
	stores.SeedUser(ptrInt64(1), string(authdomain.RoleAkuntan))
	stores.SeedUser(ptrInt64(1), string(authdomain.RoleAhliGizi))
	stores.SeedUser(ptrInt64(2), string(authdomain.RoleAkuntan)) // other SPPG, must not notify
	id := seedDraftMenu(t, svc, ctx, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), "Menu Setuju", beras.ID)

	res, err := svc.ApproveMenu(ctx, kepalaClaims(), id, nil)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if res.Menu.Status != domain.MenuStatusDisetujui {
		t.Fatalf("status = %q", res.Menu.Status)
	}
	if res.Menu.DisetujuiOleh == nil || *res.Menu.DisetujuiOleh != 3 || res.Menu.DisetujuiAt == nil {
		t.Fatalf("approver/at missing: %+v", res.Menu)
	}
	notifs := stores.Notifications()
	if len(notifs) != 2 {
		t.Fatalf("expected 2 notifications, got %d: %+v", len(notifs), notifs)
	}
	for _, n := range notifs {
		if n.RefTabel != "menu" || n.RefID != id || n.Jenis == "" {
			t.Fatalf("bad notif: %+v", n)
		}
	}
	// Double approve is 409.
	if _, err := svc.ApproveMenu(ctx, kepalaClaims(), id, nil); err != domain.ErrStatusConflict {
		t.Fatalf("double approve must be 409, got %v", err)
	}
	// Non-kepala forbidden.
	if _, err := svc.ApproveMenu(ctx, giziClaims(), id, nil); err != domain.ErrForbidden {
		t.Fatalf("non-kepala must be 403, got %v", err)
	}
	// Cross-SPPG is 404.
	other := &authdomain.Claims{UserID: 8, Role: authdomain.RoleKepalaSPPG, SPPGID: ptrInt64(2)}
	if _, err := svc.ApproveMenu(ctx, other, id, nil); err != domain.ErrMenuNotFound {
		t.Fatalf("cross-sppg must be 404, got %v", err)
	}
}

func TestApproveIncompleteMenu(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, stores := newApprovalFixture(now)
	ctx := context.Background()
	beras, _ := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Beras", Kategori: "karbohidrat", Satuan: "kg"}, nil)
	id := seedDraftMenu(t, svc, ctx, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), "Menu Kosong?", beras.ID)
	// Empty the composition directly to simulate incompleteness.
	m, _ := stores.FindMenuByID(ctx, id)
	empty := []domain.MenuBahan{}
	if _, _, err := stores.UpdateMenu(ctx, m, &empty); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveMenu(ctx, kepalaClaims(), id, nil); err != domain.ErrMenuIncomplete {
		t.Fatalf("empty composition must be 422, got %v", err)
	}
}

func TestRevertMenu(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, stores := newApprovalFixture(now)
	ctx := context.Background()
	beras, _ := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Beras", Kategori: "karbohidrat", Satuan: "kg"}, nil)
	id := seedDraftMenu(t, svc, ctx, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), "Menu Revert", beras.ID)
	if _, err := svc.ApproveMenu(ctx, kepalaClaims(), id, nil); err != nil {
		t.Fatal(err)
	}
	// Empty reason is 400.
	if _, err := svc.RevertMenu(ctx, kepalaClaims(), id, "  ", nil); err == nil {
		t.Fatal("blank alasan must fail")
	}
	before := stores.AuditCount()
	res, err := svc.RevertMenu(ctx, kepalaClaims(), id, "protein kurang, tambah telur", nil)
	if err != nil {
		t.Fatalf("revert: %v", err)
	}
	if res.Menu.Status != domain.MenuStatusDraf || res.Menu.DisetujuiOleh != nil || res.Menu.DisetujuiAt != nil {
		t.Fatalf("reverted wrong: %+v", res.Menu)
	}
	if stores.AuditCount() != before+1 {
		t.Fatal("revert must audit")
	}
	// Reverting a draft is 409.
	if _, err := svc.RevertMenu(ctx, kepalaClaims(), id, "lagi", nil); err != domain.ErrStatusConflict {
		t.Fatalf("revert draft must be 409, got %v", err)
	}
	// Used menu cannot be reverted.
	if _, err := svc.ApproveMenu(ctx, kepalaClaims(), id, nil); err != nil {
		t.Fatal(err)
	}
	stores.MarkMenuUsed(id)
	if _, err := svc.RevertMenu(ctx, kepalaClaims(), id, "batal", nil); err != domain.ErrMenuInUse {
		t.Fatalf("used revert must be 409, got %v", err)
	}
}

func TestGetKebutuhanFormula(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	svc, _ := newApprovalFixture(now)
	ctx := context.Background()
	beras, _ := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Beras", Kategori: "karbohidrat", Satuan: "kg"}, nil)
	telur, _ := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Telur", Kategori: "protein_hewani", Satuan: "butir", GramPerSatuan: ptrFloat(60)}, nil)
	id := seedDraftMenu(t, svc, ctx, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), "Menu Hitung", beras.ID, telur.ID)
	// Fix grams: beras 100, telur 50 (seedDraftMenu used 50 for both; adjust via update).
	items := []domain.MenuBahanInput{{BahanID: beras.ID, GramPerPorsi: 100}, {BahanID: telur.ID, GramPerPorsi: 50}}
	if _, err := svc.UpdateMenu(ctx, giziClaims(), id, domain.UpdateMenuInput{Items: &items}, nil); err != nil {
		t.Fatal(err)
	}
	// Default target = 250 recipients, no buffer.
	res, err := svc.GetKebutuhan(ctx, akuntanClaims(), id, nil, nil)
	if err != nil {
		t.Fatalf("kebutuhan: %v", err)
	}
	if res.TargetPorsi != 250 || res.CadanganPersen != 0 || len(res.Items) != 2 {
		t.Fatalf("header wrong: %+v", res)
	}
	byID := map[int64]KebutuhanItem{}
	for _, it := range res.Items {
		byID[it.BahanID] = it
	}
	if b := byID[beras.ID]; b.TotalGram != 25000 || b.Qty != 25 {
		t.Fatalf("beras wrong: %+v", b)
	}
	if e := byID[telur.ID]; e.TotalGram != 12500 || e.Qty != 209 {
		t.Fatalf("telur wrong: %+v", e)
	}
	// Simulation params do not mutate stored menu.
	simTarget := 100
	simCad := 10.0
	sim, err := svc.GetKebutuhan(ctx, akuntanClaims(), id, &simTarget, &simCad)
	if err != nil {
		t.Fatal(err)
	}
	if sim.TargetPorsi != 100 || sim.CadanganPersen != 10 {
		t.Fatalf("sim header: %+v", sim)
	}
	if b := sim.Items[0]; b.TotalGram == 0 || b.Qty == 0 {
		t.Fatalf("sim items: %+v", sim.Items)
	}
	det, _ := svc.GetMenu(ctx, giziClaims(), id)
	if det.Menu.TargetPorsi != 250 {
		t.Fatalf("simulation must not mutate, target=%d", det.Menu.TargetPorsi)
	}
	// Validation: bad target, bad cadangan.
	bad := 0
	if _, err := svc.GetKebutuhan(ctx, akuntanClaims(), id, &bad, nil); err == nil {
		t.Fatal("target 0 must fail")
	}
	over := 25.0
	if _, err := svc.GetKebutuhan(ctx, akuntanClaims(), id, nil, &over); err == nil {
		t.Fatal("cadangan >20 must fail")
	}
	// RBAC: dapur forbidden, cross-sppg 404.
	dapur := &authdomain.Claims{UserID: 9, Role: authdomain.RolePetugasDapur, SPPGID: ptrInt64(1)}
	if _, err := svc.GetKebutuhan(ctx, dapur, id, nil, nil); err != domain.ErrForbidden {
		t.Fatalf("dapur must be 403, got %v", err)
	}
	otherKepala := &authdomain.Claims{UserID: 8, Role: authdomain.RoleKepalaSPPG, SPPGID: ptrInt64(2)}
	if _, err := svc.GetKebutuhan(ctx, otherKepala, id, nil, nil); err != domain.ErrMenuNotFound {
		t.Fatalf("cross must be 404, got %v", err)
	}
}

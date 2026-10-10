package application

import (
	"context"
	"strings"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
)

func copyDay(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestCopyMenuHappyPath(t *testing.T) {
	now := copyDay("2026-10-09")
	svc, stores := newApprovalFixture(now)
	_ = stores
	ctx := context.Background()
	beras := mustCreateBahan(t, svc, ctx, "Beras Copy")
	telur, err := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Telur Copy", Kategori: "protein_hewani", Satuan: "butir", GramPerSatuan: ptrFloat(60)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	src := seedDraftMenu(t, svc, ctx, copyDay("2026-10-10"), "Menu Mingguan", beras, telur.ID)

	before := stores.AuditCount()
	res, err := svc.CopyMenu(ctx, giziClaims(), src, domain.CopyMenuInput{
		TanggalTujuan: []time.Time{copyDay("2026-10-11"), copyDay("2026-10-12")},
	}, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if len(res.Menus) != 2 || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "capacity") {
		t.Fatalf("menus/warnings wrong: %d menus %v", len(res.Menus), res.Warnings)
	}
	for i, m := range res.Menus {
		if m.Status != domain.MenuStatusDraf {
			t.Fatalf("copy %d status = %q", i, m.Status)
		}
		if m.DibuatOleh != 2 {
			t.Fatalf("copy %d dibuat_oleh = %d", i, m.DibuatOleh)
		}
		if m.TargetPorsi != 250 {
			t.Fatalf("copy %d target = %d, want recalculated 250", i, m.TargetPorsi)
		}
		if len(res.Items[i]) != 2 {
			t.Fatalf("copy %d items = %d", i, len(res.Items[i]))
		}
	}
	if got := res.Menus[0].Tanggal.Format("2006-01-02"); got != "2026-10-11" {
		t.Fatalf("first copy tanggal = %s", got)
	}
	if stores.AuditCount() != before+2 {
		t.Fatalf("expected 2 audit rows, got %d", stores.AuditCount()-before)
	}
}

func TestCopyMenuSkipsInactiveBahan(t *testing.T) {
	now := copyDay("2026-10-09")
	svc, _ := newApprovalFixture(now)
	ctx := context.Background()
	beras := mustCreateBahan(t, svc, ctx, "Beras Skip")
	telur, _ := svc.CreateBahan(ctx, giziClaims(), domain.CreateBahanInput{Nama: "Telur Skip", Kategori: "protein_hewani", Satuan: "butir", GramPerSatuan: ptrFloat(60)}, nil)
	src := seedDraftMenu(t, svc, ctx, copyDay("2026-10-10"), "Menu Skip", beras, telur.ID)
	off := false
	if _, err := svc.UpdateBahan(ctx, giziClaims(), telur.ID, domain.UpdateBahanInput{Aktif: &off}, nil); err != nil {
		t.Fatal(err)
	}
	res, err := svc.CopyMenu(ctx, giziClaims(), src, domain.CopyMenuInput{TanggalTujuan: []time.Time{copyDay("2026-10-11")}}, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if len(res.Items[0]) != 1 || res.Items[0][0].BahanID != beras {
		t.Fatalf("inactive bahan must be skipped: %+v", res.Items[0])
	}
	if len(res.Warnings) != 2 || !strings.Contains(res.Warnings[0], "Telur Skip") {
		t.Fatalf("warnings must name skipped bahan: %v", res.Warnings)
	}
}

func TestCopyMenuConflictAtomic(t *testing.T) {
	now := copyDay("2026-10-09")
	svc, stores := newApprovalFixture(now)
	ctx := context.Background()
	beras := mustCreateBahan(t, svc, ctx, "Beras Atom")
	src := seedDraftMenu(t, svc, ctx, copyDay("2026-10-10"), "Menu Atom", beras)
	// Occupy the second target date.
	seedDraftMenu(t, svc, ctx, copyDay("2026-10-12"), "Menu Penghuni", beras)

	_, err := svc.CopyMenu(ctx, giziClaims(), src, domain.CopyMenuInput{
		TanggalTujuan: []time.Time{copyDay("2026-10-11"), copyDay("2026-10-12")},
	}, nil)
	conflict, ok := err.(*domain.MenuDateConflictError)
	if !ok {
		t.Fatalf("expected date conflict, got %T %v", err, err)
	}
	if conflict.Tanggal != "2026-10-12" {
		t.Fatalf("conflict must name 2026-10-12, got %q", conflict.Tanggal)
	}
	// Atomicity: the free date must stay free.
	if exists, _ := stores.ExistsBySPPGDate(ctx, 1, copyDay("2026-10-11")); exists {
		t.Fatal("failed copy must not leave partial menus")
	}
}

func TestCopyMenuValidationAndRBAC(t *testing.T) {
	now := copyDay("2026-10-09")
	svc, _ := newApprovalFixture(now)
	ctx := context.Background()
	beras := mustCreateBahan(t, svc, ctx, "Beras RBAC")
	src := seedDraftMenu(t, svc, ctx, copyDay("2026-10-10"), "Menu RBAC", beras)

	many := make([]time.Time, 0, 8)
	for i := 1; i <= 8; i++ {
		many = append(many, copyDay("2026-10-09").AddDate(0, 0, i))
	}
	if _, err := svc.CopyMenu(ctx, giziClaims(), src, domain.CopyMenuInput{TanggalTujuan: many}, nil); err == nil {
		t.Fatal(">7 dates must fail")
	}
	if _, err := svc.CopyMenu(ctx, giziClaims(), src, domain.CopyMenuInput{TanggalTujuan: []time.Time{copyDay("2026-10-08")}}, nil); err == nil {
		t.Fatal("past date must fail")
	}
	d := copyDay("2026-10-11")
	if _, err := svc.CopyMenu(ctx, giziClaims(), src, domain.CopyMenuInput{TanggalTujuan: []time.Time{d, d}}, nil); err == nil {
		t.Fatal("duplicate dates must fail")
	}
	if _, err := svc.CopyMenu(ctx, adminClaims(), src, domain.CopyMenuInput{TanggalTujuan: []time.Time{d}}, nil); err != domain.ErrForbidden {
		t.Fatalf("non-gizi must be 403, got %v", err)
	}
	otherGizi := &authdomain.Claims{UserID: 9, Role: authdomain.RoleAhliGizi, SPPGID: ptrInt64(2)}
	if _, err := svc.CopyMenu(ctx, otherGizi, src, domain.CopyMenuInput{TanggalTujuan: []time.Time{d}}, nil); err != domain.ErrMenuNotFound {
		t.Fatalf("cross-sppg must be 404, got %v", err)
	}
	if _, err := svc.CopyMenu(ctx, giziClaims(), 999999, domain.CopyMenuInput{TanggalTujuan: []time.Time{d}}, nil); err != domain.ErrMenuNotFound {
		t.Fatalf("missing source must be 404, got %v", err)
	}
}

func TestCopyMenuApprovedSource(t *testing.T) {
	now := copyDay("2026-10-09")
	svc, stores := newApprovalFixture(now)
	ctx := context.Background()
	beras := mustCreateBahan(t, svc, ctx, "Beras Appr")
	src := seedDraftMenu(t, svc, ctx, copyDay("2026-10-10"), "Menu Appr", beras)
	uid := int64(3)
	at := now
	if _, err := stores.SetApproval(ctx, src, domain.MenuStatusDisetujui, &uid, &at); err != nil {
		t.Fatal(err)
	}
	res, err := svc.CopyMenu(ctx, giziClaims(), src, domain.CopyMenuInput{TanggalTujuan: []time.Time{copyDay("2026-10-11")}}, nil)
	if err != nil {
		t.Fatalf("approved source must be copyable: %v", err)
	}
	if res.Menus[0].Status != domain.MenuStatusDraf || res.Menus[0].DisetujuiOleh != nil {
		t.Fatalf("copies must be fresh drafts: %+v", res.Menus[0])
	}
}

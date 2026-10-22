package application

import (
	"context"
	"testing"
	"time"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

func kepalaClaims(sppg int64) *domain.Claims {
	return &domain.Claims{UserID: 777, Role: domain.RoleKepalaSPPG, SPPGID: &sppg}
}

func mustCreateListUser(t *testing.T, f *testFixture, nama, email string, peran domain.Role, sppg *int64) int64 {
	t.Helper()
	pw := "user1234"
	created, err := f.svc.CreateUser(context.Background(), adminClaims(nil), domain.CreateUserInput{
		Nama: nama, Email: email, Peran: peran, SPPGID: sppg, PasswordAwal: &pw,
	}, nil)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return created.User.ID
}

func TestListUsersScoping(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	mustCreateListUser(t, f, "Admin Pusat", "admin@x.com", domain.RoleAdmin, nil)
	mustCreateListUser(t, f, "Kepala Satu", "kepala1@x.com", domain.RoleKepalaSPPG, ptr(int64(1)))
	mustCreateListUser(t, f, "Dapur Satu", "dapur1@x.com", domain.RolePetugasDapur, ptr(int64(1)))
	mustCreateListUser(t, f, "Akun Dua", "akun2@x.com", domain.RoleAkuntan, ptr(int64(2)))
	mustCreateListUser(t, f, "Wawas Lintas", "wawas@x.com", domain.RolePengawas, nil)

	admin := adminClaims(nil)
	views, total, err := f.svc.ListUsers(ctx, admin, domain.UserFilter{})
	if err != nil {
		t.Fatalf("admin list: %v", err)
	}
	if total != 5 {
		t.Fatalf("admin must see all, got %d", total)
	}
	if len(views) != 5 || views[0].User.Nama != "Admin Pusat" {
		t.Fatalf("default sort nama wrong: %+v", views)
	}

	kepala := kepalaClaims(1)
	kviews, ktotal, err := f.svc.ListUsers(ctx, kepala, domain.UserFilter{})
	if err != nil {
		t.Fatalf("kepala list: %v", err)
	}
	if ktotal != 2 {
		t.Fatalf("kepala must see 2 (own sppg, no admin/pengawas), got %d", ktotal)
	}
	for _, v := range kviews {
		if v.User.Peran == domain.RoleAdmin || v.User.Peran == domain.RolePengawas {
			t.Fatalf("privileged leaked: %+v", v.User)
		}
		if v.User.SPPGID == nil || *v.User.SPPGID != 1 {
			t.Fatalf("cross-sppg leaked: %+v", v.User)
		}
	}

	// Other roles are forbidden.
	peon := &domain.Claims{UserID: 5, Role: domain.RoleAkuntan, SPPGID: ptr(int64(1))}
	if _, _, err := f.svc.ListUsers(ctx, peon, domain.UserFilter{}); err != domain.ErrForbidden {
		t.Fatalf("akuntan must be 403, got %v", err)
	}
	wawas := &domain.Claims{UserID: 6, Role: domain.RolePengawas}
	if _, _, err := f.svc.ListUsers(ctx, wawas, domain.UserFilter{}); err != domain.ErrForbidden {
		t.Fatalf("pengawas must be 403, got %v", err)
	}
	if _, _, err := f.svc.ListUsers(ctx, nil, domain.UserFilter{}); err != domain.ErrUnauthorized {
		t.Fatalf("nil actor must be 401, got %v", err)
	}
}

func TestListUsersFilters(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	mustCreateListUser(t, f, "Dapur Pagi", "pagi@x.com", domain.RolePetugasDapur, ptr(int64(1)))
	mustCreateListUser(t, f, "Dapur Sore", "sore@x.com", domain.RolePetugasDistribusi, ptr(int64(1)))
	akunID := mustCreateListUser(t, f, "Akun Audit", "audit@x.com", domain.RoleAkuntan, ptr(int64(1)))
	admin := adminClaims(nil)

	// q is case-insensitive partial on nama/email.
	views, total, err := f.svc.ListUsers(ctx, admin, domain.UserFilter{Q: ptr("DAPUR")})
	if err != nil || total != 2 {
		t.Fatalf("q filter: %v %d", err, total)
	}
	views, total, err = f.svc.ListUsers(ctx, admin, domain.UserFilter{Q: ptr("audit@x")})
	if err != nil || total != 1 || views[0].User.ID != akunID {
		t.Fatalf("q email: %v %d", err, total)
	}
	if _, _, err := f.svc.ListUsers(ctx, admin, domain.UserFilter{Q: ptr("a")}); err == nil {
		t.Fatal("short q must fail")
	}

	// peran multi.
	views, total, err = f.svc.ListUsers(ctx, admin, domain.UserFilter{Peran: []domain.Role{domain.RolePetugasDapur, domain.RolePetugasDistribusi}})
	if err != nil || total != 2 {
		t.Fatalf("peran multi: %v %d", err, total)
	}
	_ = views
	if _, _, err := f.svc.ListUsers(ctx, admin, domain.UserFilter{Peran: []domain.Role{"siluman"}}); err == nil {
		t.Fatal("bad peran must fail")
	}

	// aktif filter.
	u, _ := f.stores.FindByID(ctx, akunID)
	u.Aktif = false
	if _, err := f.stores.Update(ctx, u); err != nil {
		t.Fatal(err)
	}
	_, total, err = f.svc.ListUsers(ctx, admin, domain.UserFilter{Aktif: ptr(true)})
	if err != nil || total != 2 {
		t.Fatalf("aktif filter: %v %d", err, total)
	}

	// sppg_id and sppg_id=null.
	two := int64(2)
	_, total, err = f.svc.ListUsers(ctx, admin, domain.UserFilter{SPPGID: &two})
	if err != nil || total != 0 {
		t.Fatalf("sppg 2 filter: %v %d", err, total)
	}
	mustCreateListUser(t, f, "Admin Nol", "admin0@x.com", domain.RoleAdmin, nil)
	views, total, err = f.svc.ListUsers(ctx, admin, domain.UserFilter{NullSPPG: true})
	if err != nil || total != 1 || views[0].User.Email != "admin0@x.com" {
		t.Fatalf("null sppg: %v %d %+v", err, total, views)
	}

	// sekolah_id scoping.
	_, total, err = f.svc.ListUsers(ctx, admin, domain.UserFilter{SekolahID: ptr(int64(10))})
	if err != nil || total != 0 {
		t.Fatalf("sekolah filter (none assigned): %v %d", err, total)
	}
	if _, _, err := f.svc.ListUsers(ctx, admin, domain.UserFilter{SekolahID: ptr(int64(999))}); err != domain.ErrSekolahNotFound {
		t.Fatalf("missing sekolah must be 404, got %v", err)
	}

	// Sort and pagination.
	_, _, err = f.svc.ListUsers(ctx, admin, domain.UserFilter{Sort: "email"})
	if err == nil {
		t.Fatal("bad sort must fail")
	}
	if _, _, err := f.svc.ListUsers(ctx, admin, domain.UserFilter{Limit: 101}); err == nil {
		t.Fatal("per_page > 100 must fail")
	}
	paged, total, err := f.svc.ListUsers(ctx, admin, domain.UserFilter{Page: 2, Limit: 2, Sort: domain.UserSortNama})
	if err != nil || total != 4 || len(paged) != 2 {
		t.Fatalf("pagination: %v total=%d len=%d", err, total, len(paged))
	}

	// kepala_sppg sppg_id filter is ignored.
	kepala := kepalaClaims(1)
	_, ktotal, err := f.svc.ListUsers(ctx, kepala, domain.UserFilter{SPPGID: &two})
	if err != nil || ktotal != 3 {
		t.Fatalf("kepala ignores sppg_id: %v %d", err, ktotal)
	}
}

func TestGetUserDetail(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	now := f.now
	id := mustCreateListUser(t, f, "Detail Me", "detail@x.com", domain.RoleAkuntan, ptr(int64(1)))

	admin := adminClaims(nil)
	view, err := f.svc.GetUser(ctx, admin, id)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if view.User.Email != "detail@x.com" || view.Terkunci || view.JumlahSesiAktif != 0 {
		t.Fatalf("detail wrong: %+v", view)
	}
	if view.SPPG == nil || view.SPPG.Nama == "" {
		t.Fatalf("sppg ref missing: %+v", view)
	}

	// Sessions: two logins → 2 active; logout-all → 0.
	if _, err := f.svc.Login(ctx, "detail@x.com", "user1234", nil, nil); err != nil {
		t.Fatalf("login1: %v", err)
	}
	sess, err := f.svc.Login(ctx, "detail@x.com", "user1234", nil, nil)
	if err != nil {
		t.Fatalf("login2: %v", err)
	}
	_ = sess
	view, _ = f.svc.GetUser(ctx, admin, id)
	if view.JumlahSesiAktif != 2 {
		t.Fatalf("sessions = %d, want 2", view.JumlahSesiAktif)
	}
	if err := f.svc.LogoutAll(ctx, id, nil); err != nil {
		t.Fatal(err)
	}
	view, _ = f.svc.GetUser(ctx, admin, id)
	if view.JumlahSesiAktif != 0 {
		t.Fatalf("sessions after logout-all = %d", view.JumlahSesiAktif)
	}

	// Lock derivation.
	u, _ := f.stores.FindByID(ctx, id)
	until := now.Add(time.Hour)
	u.TerkunciSampai = &until
	if _, err := f.stores.Update(ctx, u); err != nil {
		t.Fatal(err)
	}
	view, _ = f.svc.GetUser(ctx, admin, id)
	if !view.Terkunci || view.User.TerkunciSampai == nil {
		t.Fatalf("locked wrong: %+v", view)
	}

	// Cross-SPPG detail is 404; kepala never resolves admin accounts.
	kepala := kepalaClaims(1)
	otherID := mustCreateListUser(t, f, "Luar", "luar@x.com", domain.RoleAkuntan, ptr(int64(2)))
	if _, err := f.svc.GetUser(ctx, kepala, otherID); err != domain.ErrNotFound {
		t.Fatalf("cross detail must be 404, got %v", err)
	}
	bossID := mustCreateListUser(t, f, "Bos", "bos@x.com", domain.RoleAdmin, ptr(int64(1)))
	if _, err := f.svc.GetUser(ctx, kepala, bossID); err != domain.ErrNotFound {
		t.Fatalf("kepala/admin detail must be 404, got %v", err)
	}
	if _, err := f.svc.GetUser(ctx, kepala, 999999); err != domain.ErrNotFound {
		t.Fatalf("missing must be 404, got %v", err)
	}
	peon := &domain.Claims{UserID: 5, Role: domain.RolePetugasDapur, SPPGID: ptr(int64(1))}
	if _, err := f.svc.GetUser(ctx, peon, id); err != domain.ErrForbidden {
		t.Fatalf("dapur must be 403, got %v", err)
	}
}

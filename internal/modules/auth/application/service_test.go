package application

import (
	"context"
	"testing"
	"time"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/infrastructure"
)

type testFixture struct {
	svc    *Service
	stores *infrastructure.MemoryStores
	now    time.Time
}

func newTestFixture() *testFixture {
	now := time.Now().Truncate(time.Second)
	stores := infrastructure.NewMemoryStores()
	stores.SeedSPPG(1)
	stores.SeedSPPG(2)
	stores.SeedSekolah(10, 1)
	current := now
	svc := New(Deps{
		Users:   stores,
		Refresh: stores,
		Audits:  stores,
		SPPG:    stores,
		Sekolah: stores,
		Tokens:  infrastructure.NewJWTIssuer("test-secret"),
		Hasher:  infrastructure.NewBcryptHasher(4),
		Clock:   func() time.Time { return current },
	})
	f := &testFixture{svc: svc, stores: stores, now: now}
	return f
}

func (f *testFixture) advance(d time.Duration) {
	f.now = f.now.Add(d)
}

// clearWajib clears wajib_ganti_password so refresh flows can be tested
// independently of the first-login gate (MVP-001.6).
func (f *testFixture) clearWajib(t *testing.T, userID int64) {
	t.Helper()
	u, err := f.stores.FindByID(context.Background(), userID)
	if err != nil || u == nil {
		t.Fatalf("clearWajib: %v", err)
	}
	u.WajibGantiPassword = false
	if _, err := f.stores.Update(context.Background(), u); err != nil {
		t.Fatalf("clearWajib update: %v", err)
	}
}

func adminClaims(sppg *int64) *domain.Claims {
	return &domain.Claims{UserID: 999, Role: domain.RoleAdmin, SPPGID: sppg}
}

func ptr[T any](v T) *T { return &v }

func TestCreateUserSuccessAndDuplicate(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg := int64(1)
	res, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Petugas Dapur", Email: "DAPUR@Example.com", Peran: domain.RolePetugasDapur, SPPGID: &sppg, PasswordAwal: ptr("dapur123"),
	}, ptr("127.0.0.1"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res.User.Email != "dapur@example.com" {
		t.Fatalf("email should be lowercase, got %q", res.User.Email)
	}
	if !res.User.WajibGantiPassword {
		t.Fatal("new accounts must require password change")
	}
	if res.GeneratedPassword != nil {
		t.Fatal("explicit password should not generate one")
	}
	// Duplicate (case-insensitive) -> 409 sentinel.
	_, err = f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Lain", Email: "dapur@example.com", Peran: domain.RoleAkuntan, SPPGID: &sppg, PasswordAwal: ptr("akun1234"),
	}, nil)
	if err != domain.ErrEmailExists {
		t.Fatalf("expected ErrEmailExists, got %v", err)
	}
}

func TestCreateUserGeneratesPassword(t *testing.T) {
	f := newTestFixture()
	sppg := int64(1)
	res, err := f.svc.CreateUser(context.Background(), adminClaims(nil), domain.CreateUserInput{
		Nama: "Akuntan Baru", Email: "akun@x.com", Peran: domain.RoleAkuntan, SPPGID: &sppg,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.GeneratedPassword == nil || !domain.ValidatePasswordPolicy(*res.GeneratedPassword) {
		t.Fatalf("generated password must satisfy policy, got %v", res.GeneratedPassword)
	}
}

func TestKepalaScopeRules(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg1 := int64(1)
	sppg2 := int64(2)
	kepala := &domain.Claims{UserID: 1, Role: domain.RoleKepalaSPPG, SPPGID: &sppg1}

	// Cannot create outside own SPPG.
	_, err := f.svc.CreateUser(ctx, kepala, domain.CreateUserInput{
		Nama: "Luar", Email: "luar@x.com", Peran: domain.RolePetugasDapur, SPPGID: &sppg2, PasswordAwal: ptr("luar1234"),
	}, nil)
	if err != domain.ErrForbidden {
		t.Fatalf("expected forbidden for other SPPG, got %v", err)
	}
	// Cannot create admin/pengawas.
	_, err = f.svc.CreateUser(ctx, kepala, domain.CreateUserInput{
		Nama: "Admin", Email: "admin2@x.com", Peran: domain.RoleAdmin, SPPGID: &sppg1, PasswordAwal: ptr("admin123"),
	}, nil)
	if err != domain.ErrForbidden {
		t.Fatalf("expected forbidden for admin role, got %v", err)
	}
	// Can create within SPPG.
	_, err = f.svc.CreateUser(ctx, kepala, domain.CreateUserInput{
		Nama: "Dapur Satu", Email: "dapur1@x.com", Peran: domain.RolePetugasDapur, SPPGID: &sppg1, PasswordAwal: ptr("dapur123"),
	}, nil)
	if err != nil {
		t.Fatalf("kepala create within sppg: %v", err)
	}
}

func TestLoginLockoutAndAudit(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg := int64(1)
	_, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Login User", Email: "login@x.com", Peran: domain.RoleAkuntan, SPPGID: &sppg, PasswordAwal: ptr("benar123"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 5 wrong attempts -> still 401 each.
	for i := 0; i < 5; i++ {
		_, err := f.svc.Login(ctx, "login@x.com", "salah123", ptr("1.1.1.1"), nil)
		if err != domain.ErrInvalidCredentials {
			t.Fatalf("attempt %d: expected invalid credentials, got %v", i, err)
		}
	}
	// 6th -> locked.
	_, err = f.svc.Login(ctx, "login@x.com", "salah123", ptr("1.1.1.1"), nil)
	if _, ok := err.(*AccountLockedError); !ok {
		t.Fatalf("expected locked, got %v", err)
	}
	// Correct password while locked -> still locked.
	_, err = f.svc.Login(ctx, "login@x.com", "benar123", ptr("1.1.1.1"), nil)
	if _, ok := err.(*AccountLockedError); !ok {
		t.Fatalf("expected locked even with correct pw, got %v", err)
	}
	// Audit should contain failed logins.
	entries, _, err := f.svc.ListAuditLogs(ctx, adminClaims(nil), domain.AuditFilter{Tabel: ptr("auth")})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 5 {
		t.Fatalf("expected >=5 login audits, got %d", len(entries))
	}
}

func TestLoginInactiveAndUnknownEmail(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	// Unknown email and wrong password must give same message sentinel.
	_, errUnknown := f.svc.Login(ctx, "nope@x.com", "whatever1", nil, nil)
	sppg := int64(1)
	created, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Aktif User", Email: "aktif@x.com", Peran: domain.RoleAkuntan, SPPGID: &sppg, PasswordAwal: ptr("aktif123"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, errWrong := f.svc.Login(ctx, "aktif@x.com", "salah123", nil, nil)
	if (errUnknown == nil) != (errWrong == nil) || errUnknown != errWrong {
		t.Fatalf("unknown vs wrong password must match: %v vs %v", errUnknown, errWrong)
	}
	// Deactivate then login -> inactive.
	deact := false
	_, err = f.svc.UpdateUser(ctx, adminClaims(nil), created.User.ID, domain.UpdateUserInput{Aktif: &deact}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Login(ctx, "aktif@x.com", "aktif123", nil, nil)
	if err != domain.ErrAccountInactive {
		t.Fatalf("expected inactive, got %v", err)
	}
}

func TestRefreshRotationAndLogout(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg := int64(1)
	created, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Refresh User", Email: "refresh@x.com", Peran: domain.RoleAkuntan, SPPGID: &sppg, PasswordAwal: ptr("refresh1"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.clearWajib(t, created.User.ID)
	login, err := f.svc.Login(ctx, "refresh@x.com", "refresh1", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := f.svc.Refresh(ctx, login.RefreshToken, nil, nil)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if rotated.RefreshToken == login.RefreshToken {
		t.Fatal("refresh token must rotate")
	}
	// Logout new token then refresh fails.
	if err := f.svc.Logout(ctx, rotated.RefreshToken, nil); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Refresh(ctx, rotated.RefreshToken, nil, nil)
	if err != domain.ErrRefreshRevoked {
		t.Fatalf("expected revoked after logout, got %v", err)
	}
}

func TestChangePasswordAndMe(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg := int64(1)
	created, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Ganti User", Email: "ganti@x.com", Peran: domain.RoleAkuntan, SPPGID: &sppg, PasswordAwal: ptr("lama1234"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	login, err := f.svc.Login(ctx, "ganti@x.com", "lama1234", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !login.WajibGantiPassword {
		t.Fatal("new user must have wajib_ganti_password=true")
	}
	// Wrong old password is rejected with a dedicated sentinel.
	if err := f.svc.ChangePassword(ctx, created.User.ID, "salah123", "baru1234", nil); err != domain.ErrOldPasswordMismatch {
		t.Fatalf("expected old mismatch, got %v", err)
	}
	// Weak and reused passwords are rejected.
	if err := f.svc.ChangePassword(ctx, created.User.ID, "lama1234", "lemah", nil); err == nil {
		t.Fatal("expected weak rejection")
	}
	if err := f.svc.ChangePassword(ctx, created.User.ID, "lama1234", "lama1234", nil); err == nil {
		t.Fatal("expected reuse rejection")
	}
	// Second session: only other sessions are revoked, current survives.
	second, err := f.svc.Login(ctx, "ganti@x.com", "lama1234", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ChangePassword(ctx, created.User.ID, "lama1234", "baru1234", nil); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if _, err := f.svc.Refresh(ctx, second.RefreshToken, nil, nil); err != nil {
		t.Fatalf("current session must survive, got %v", err)
	}
	if _, err := f.svc.Refresh(ctx, login.RefreshToken, nil, nil); err != domain.ErrRefreshRevoked {
		t.Fatalf("expected other session revoked, got %v", err)
	}
	profile, err := f.svc.Me(ctx, created.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.User.WajibGantiPassword {
		t.Fatal("wajib_ganti_password should be false after change")
	}
	if len(profile.Permissions) == 0 {
		t.Fatal("expected permissions")
	}
	if profile.SPPG == nil || profile.SPPG.ID != 1 {
		t.Fatalf("expected sppg ref, got %+v", profile.SPPG)
	}
}

func TestRefreshReuseRevokesAllSessions(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg := int64(1)
	created, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Theft User", Email: "theft@x.com", Peran: domain.RoleAkuntan, SPPGID: &sppg, PasswordAwal: ptr("theft123"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.clearWajib(t, created.User.ID)
	first, err := f.svc.Login(ctx, "theft@x.com", "theft123", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.svc.Refresh(ctx, first.RefreshToken, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Attacker reuses the already-rotated first token: theft detection
	// must revoke every session, so the second token dies too.
	_, err = f.svc.Refresh(ctx, first.RefreshToken, nil, nil)
	if err != domain.ErrRefreshRevoked {
		t.Fatalf("expected revoked on reuse, got %v", err)
	}
	_, err = f.svc.Refresh(ctx, second.RefreshToken, nil, nil)
	if err != domain.ErrRefreshRevoked {
		t.Fatalf("expected all sessions revoked after reuse, got %v", err)
	}
}

func TestLogoutAllRevokesEverySession(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg := int64(1)
	created, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Multi User", Email: "multi@x.com", Peran: domain.RoleAkuntan, SPPGID: &sppg, PasswordAwal: ptr("multi123"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.clearWajib(t, created.User.ID)
	a, err := f.svc.Login(ctx, "multi@x.com", "multi123", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.svc.Login(ctx, "multi@x.com", "multi123", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.LogoutAll(ctx, created.User.ID, nil); err != nil {
		t.Fatal(err)
	}
	for i, tok := range []string{a.RefreshToken, b.RefreshToken} {
		if _, err := f.svc.Refresh(ctx, tok, nil, nil); err != domain.ErrRefreshRevoked {
			t.Fatalf("session %d: expected revoked after logout-all, got %v", i, err)
		}
	}
}

func TestMeEnrichedWithNames(t *testing.T) {
	f := newTestFixture()
	f.stores.SeedSPPGWithName(1, "SPPG Cempaka")
	f.stores.SeedSekolahWithName(10, 1, "SDN 01")
	ctx := context.Background()
	sppg := int64(1)
	sekolah := int64(10)
	created, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "PIC Sekolah", Email: "pics@x.com", Peran: domain.RolePICsekolah, SPPGID: &sppg, SekolahID: &sekolah, PasswordAwal: ptr("pics1234"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := f.svc.Me(ctx, created.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.SPPG == nil || profile.SPPG.Nama != "SPPG Cempaka" {
		t.Fatalf("expected sppg name, got %+v", profile.SPPG)
	}
	if profile.Sekolah == nil || profile.Sekolah.Nama != "SDN 01" {
		t.Fatalf("expected sekolah name, got %+v", profile.Sekolah)
	}
	if !profile.User.WajibGantiPassword {
		t.Fatal("new user must require password change")
	}
	if len(profile.Permissions) == 0 {
		t.Fatal("expected permissions")
	}
}

func TestUpdateUserResetPassword(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg := int64(1)
	created, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Reset User", Email: "reset@x.com", Peran: domain.RoleAkuntan, SPPGID: &sppg, PasswordAwal: ptr("reset123"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.clearWajib(t, created.User.ID)
	before, err := f.svc.Login(ctx, "reset@x.com", "reset123", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.svc.UpdateUser(ctx, adminClaims(nil), created.User.ID, domain.UpdateUserInput{ResetPassword: ptr(true)}, nil)
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if res.TempPassword == nil || !domain.ValidatePasswordPolicy(*res.TempPassword) {
		t.Fatalf("expected policy-compliant temp password, got %v", res.TempPassword)
	}
	if !res.User.WajibGantiPassword {
		t.Fatal("reset must set wajib_ganti_password")
	}
	// Old password stops working, temp password works, old session dies.
	if _, err := f.svc.Login(ctx, "reset@x.com", "reset123", nil, nil); err != domain.ErrInvalidCredentials {
		t.Fatalf("old pw must fail, got %v", err)
	}
	login, err := f.svc.Login(ctx, "reset@x.com", *res.TempPassword, nil, nil)
	if err != nil {
		t.Fatalf("temp pw login: %v", err)
	}
	if !login.WajibGantiPassword {
		t.Fatal("temp login must require password change")
	}
	if _, err := f.svc.Refresh(ctx, before.RefreshToken, nil, nil); err != domain.ErrRefreshRevoked {
		t.Fatalf("old session must die, got %v", err)
	}
}

func TestUpdateUserSelfGuard(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg := int64(1)
	created, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Kepala", Email: "kepala@x.com", Peran: domain.RoleKepalaSPPG, SPPGID: &sppg, PasswordAwal: ptr("kepala12"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	self := &domain.Claims{UserID: created.User.ID, Role: domain.RoleKepalaSPPG, SPPGID: &sppg}
	if _, err := f.svc.UpdateUser(ctx, self, created.User.ID, domain.UpdateUserInput{Aktif: ptr(false)}, nil); err != domain.ErrSelfModification {
		t.Fatalf("self deactivate: got %v", err)
	}
	demote := domain.RoleAkuntan
	if _, err := f.svc.UpdateUser(ctx, self, created.User.ID, domain.UpdateUserInput{Peran: &demote}, nil); err != domain.ErrSelfModification {
		t.Fatalf("self demote: got %v", err)
	}
	// Editing own name is allowed.
	res, err := f.svc.UpdateUser(ctx, self, created.User.ID, domain.UpdateUserInput{Nama: ptr("Kepala Baru")}, nil)
	if err != nil {
		t.Fatalf("self rename: %v", err)
	}
	if res.User.Nama != "Kepala Baru" {
		t.Fatalf("nama %q", res.User.Nama)
	}
}

func TestUpdateUserLastKepalaGuard(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg := int64(1)
	k1, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Kepala Satu", Email: "k1@x.com", Peran: domain.RoleKepalaSPPG, SPPGID: &sppg, PasswordAwal: ptr("kepala12"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Kepala Dua", Email: "k2@x.com", Peran: domain.RoleKepalaSPPG, SPPGID: &sppg, PasswordAwal: ptr("kepala12"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Deactivating one of two is fine.
	if _, err := f.svc.UpdateUser(ctx, adminClaims(nil), k1.User.ID, domain.UpdateUserInput{Aktif: ptr(false)}, nil); err != nil {
		t.Fatalf("first deactivate: %v", err)
	}
	// The last one cannot go.
	if _, err := f.svc.UpdateUser(ctx, adminClaims(nil), k2.User.ID, domain.UpdateUserInput{Aktif: ptr(false)}, nil); err != domain.ErrLastKepalaRequired {
		t.Fatalf("last deactivate: got %v", err)
	}
	demote := domain.RoleAkuntan
	if _, err := f.svc.UpdateUser(ctx, adminClaims(nil), k2.User.ID, domain.UpdateUserInput{Peran: &demote}, nil); err != domain.ErrLastKepalaRequired {
		t.Fatalf("last demote: got %v", err)
	}
}

func TestRefreshBlockedWhenWajib(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg := int64(1)
	_, err := f.svc.CreateUser(ctx, adminClaims(nil), domain.CreateUserInput{
		Nama: "Fresh User", Email: "fresh@x.com", Peran: domain.RoleAkuntan, SPPGID: &sppg, PasswordAwal: ptr("fresh123"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	login, err := f.svc.Login(ctx, "fresh@x.com", "fresh123", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Refresh(ctx, login.RefreshToken, nil, nil); err != domain.ErrMustChangePassword {
		t.Fatalf("expected must-change, got %v", err)
	}
}

func TestAuditRBAC(t *testing.T) {
	f := newTestFixture()
	ctx := context.Background()
	sppg := int64(1)
	// Non-privileged role cannot list.
	user := &domain.Claims{UserID: 5, Role: domain.RoleAkuntan, SPPGID: &sppg}
	if _, _, err := f.svc.ListAuditLogs(ctx, user, domain.AuditFilter{}); err != domain.ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
	// Kepala without SPPG cannot list (defensive).
	headless := &domain.Claims{UserID: 6, Role: domain.RoleKepalaSPPG}
	if _, _, err := f.svc.ListAuditLogs(ctx, headless, domain.AuditFilter{}); err != domain.ErrForbidden {
		t.Fatalf("expected forbidden for headless kepala, got %v", err)
	}
}

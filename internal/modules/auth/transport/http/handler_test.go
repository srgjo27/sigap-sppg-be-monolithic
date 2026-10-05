package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/infrastructure"
)

func init() { gin.SetMode(gin.TestMode) }

type httpFixture struct {
	router *gin.Engine
	svc    *application.Service
	stores *infrastructure.MemoryStores
}

func newHTTPFixture() *httpFixture {
	stores := infrastructure.NewMemoryStores()
	stores.SeedSPPGWithName(1, "SPPG Cempaka")
	stores.SeedSekolahWithName(10, 1, "SDN 01")
	issuer := infrastructure.NewJWTIssuer("test-secret")
	svc := application.New(application.Deps{
		Users: stores, Refresh: stores, Audits: stores, SPPG: stores, Sekolah: stores,
		Tokens: issuer, Hasher: infrastructure.NewBcryptHasher(4),
	})
	h := NewHandler(svc)
	mw := NewMiddleware(issuer, svc)
	r := gin.New()
	v1 := r.Group("/api/v1")
	RegisterRoutes(v1, h, mw)
	return &httpFixture{router: r, svc: svc, stores: stores}
}

func doRequest(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	// Fixed IP so rate limiter is deterministic per test router.
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func seedAdminAndLogin(t *testing.T, f *httpFixture, sppg *int64, email, password string, role domain.Role) (adminToken string) {
	t.Helper()
	ctx := context.Background()
	adminClaims := &domain.Claims{UserID: 999, Role: domain.RoleAdmin}
	created, err := f.svc.CreateUser(ctx, adminClaims, domain.CreateUserInput{
		Nama: "Seed User", Email: email, Peran: role, SPPGID: sppg, PasswordAwal: &password,
	}, nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Seed admins simulate users who already changed their initial password,
	// otherwise the wajib_ganti_password gate (MVP-001.6) blocks management APIs.
	clearWajib(t, f, created.User.ID)
	login, err := f.svc.Login(ctx, email, password, nil, nil)
	if err != nil {
		t.Fatalf("seed login: %v", err)
	}
	return login.AccessToken
}

// clearWajib clears the wajib_ganti_password flag directly in the store.
func clearWajib(t *testing.T, f *httpFixture, userID int64) {
	t.Helper()
	ctx := context.Background()
	u, err := f.stores.FindByID(ctx, userID)
	if err != nil || u == nil {
		t.Fatalf("clearWajib: %v", err)
	}
	u.WajibGantiPassword = false
	if _, err := f.stores.Update(ctx, u); err != nil {
		t.Fatalf("clearWajib update: %v", err)
	}
}

// clearWajibByEmail clears the flag for a user looked up by email.
func clearWajibByEmail(t *testing.T, f *httpFixture, email string) {
	t.Helper()
	ctx := context.Background()
	u, err := f.stores.FindByEmail(ctx, email)
	if err != nil || u == nil {
		t.Fatalf("clearWajibByEmail: %v", err)
	}
	clearWajib(t, f, u.ID)
}

func TestCreateUserEndpoint(t *testing.T) {
	f := newHTTPFixture()
	adminToken := seedAdminAndLogin(t, f, ptrInt(1), "admin@x.com", "admin1234", domain.RoleAdmin)

	w := doRequest(t, f.router, "POST", "/api/v1/users", adminToken, map[string]any{
		"nama": "Dapur A", "email": "dapur@x.com", "peran": "petugas_dapur", "sppg_id": 1, "password_awal": "dapur123",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: got %d body %s", w.Code, w.Body.String())
	}
	var created CreateUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.User.Email != "dapur@x.com" {
		t.Fatalf("email %q", created.User.Email)
	}

	dup := doRequest(t, f.router, "POST", "/api/v1/users", adminToken, map[string]any{
		"nama": "Dapur B", "email": "DAPUR@x.com", "peran": "akuntan", "sppg_id": 1, "password_awal": "akun1234",
	})
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate: got %d %s", dup.Code, dup.Body.String())
	}

	unauth := doRequest(t, f.router, "POST", "/api/v1/users", "", map[string]any{
		"nama": "X", "email": "x@x.com", "peran": "akuntan", "sppg_id": 1,
	})
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: got %d", unauth.Code)
	}
}

func TestLoginMeRefreshLogoutFlow(t *testing.T) {
	f := newHTTPFixture()
	_ = seedAdminAndLogin(t, f, ptrInt(1), "boss@x.com", "boss1234", domain.RoleAdmin)
	// Register a user via API then login via API.
	adminLogin := doRequest(t, f.router, "POST", "/api/v1/auth/login", "", map[string]any{"email": "boss@x.com", "password": "boss1234"})
	if adminLogin.Code != http.StatusOK {
		t.Fatalf("admin login: %d %s", adminLogin.Code, adminLogin.Body.String())
	}
	var adminSess LoginResponse
	_ = json.Unmarshal(adminLogin.Body.Bytes(), &adminSess)

	create := doRequest(t, f.router, "POST", "/api/v1/users", adminSess.AccessToken, map[string]any{
		"nama": "Kasir", "email": "kasir@x.com", "peran": "akuntan", "sppg_id": 1, "password_awal": "kasir123",
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create kasir: %d %s", create.Code, create.Body.String())
	}

	login := doRequest(t, f.router, "POST", "/api/v1/auth/login", "", map[string]any{"email": "kasir@x.com", "password": "kasir123"})
	if login.Code != http.StatusOK {
		t.Fatalf("login: %d %s", login.Code, login.Body.String())
	}
	var sess LoginResponse
	if err := json.Unmarshal(login.Body.Bytes(), &sess); err != nil {
		t.Fatal(err)
	}
	if sess.AccessToken == "" || sess.RefreshToken == "" || !sess.WajibGantiPassword {
		t.Fatalf("login payload incomplete: %+v", sess)
	}

	me := doRequest(t, f.router, "GET", "/api/v1/auth/me", sess.AccessToken, nil)
	if me.Code != http.StatusOK {
		t.Fatalf("me: %d %s", me.Code, me.Body.String())
	}
	var profile MeResponse
	if err := json.Unmarshal(me.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	if profile.Email != "kasir@x.com" || !profile.WajibGantiPassword || len(profile.Permissions) == 0 {
		t.Fatalf("me payload incomplete: %+v", profile)
	}
	if profile.SPPG == nil || profile.SPPG.Nama != "SPPG Cempaka" {
		t.Fatalf("me sppg missing: %+v", profile.SPPG)
	}

	badLogin := doRequest(t, f.router, "POST", "/api/v1/auth/login", "", map[string]any{"email": "kasir@x.com", "password": "wrong123"})
	if badLogin.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: got %d", badLogin.Code)
	}

	// Fresh accounts must change password first: refresh is gated (403).
	gated := doRequest(t, f.router, "POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": sess.RefreshToken})
	if gated.Code != http.StatusForbidden {
		t.Fatalf("gated refresh: got %d %s", gated.Code, gated.Body.String())
	}
	gatedManage := doRequest(t, f.router, "POST", "/api/v1/users", sess.AccessToken, map[string]any{
		"nama": "X", "email": "x@x.com", "peran": "akuntan", "sppg_id": 1,
	})
	if gatedManage.Code != http.StatusForbidden {
		t.Fatalf("gated manage: got %d", gatedManage.Code)
	}

	pw := doRequest(t, f.router, "PUT", "/api/v1/auth/password", sess.AccessToken, map[string]any{"password_lama": "kasir123", "password_baru": "kasir456"})
	if pw.Code != http.StatusNoContent {
		t.Fatalf("change pw: %d %s", pw.Code, pw.Body.String())
	}

	refresh := doRequest(t, f.router, "POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": sess.RefreshToken})
	if refresh.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", refresh.Code, refresh.Body.String())
	}
	var rotated RefreshResponse
	_ = json.Unmarshal(refresh.Body.Bytes(), &rotated)

	logout := doRequest(t, f.router, "POST", "/api/v1/auth/logout", rotated.AccessToken, map[string]any{"refresh_token": rotated.RefreshToken})
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout: got %d %s", logout.Code, logout.Body.String())
	}
	if logout.Body.Len() != 0 {
		t.Fatalf("logout must have no body, got %q", logout.Body.String())
	}
	reuse := doRequest(t, f.router, "POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": rotated.RefreshToken})
	if reuse.Code != http.StatusUnauthorized {
		t.Fatalf("reuse after logout: got %d", reuse.Code)
	}
}

func TestRefreshReuseTriggersTheftDetection(t *testing.T) {
	f := newHTTPFixture()
	_ = seedAdminAndLogin(t, f, ptrInt(1), "boss2@x.com", "boss1234", domain.RoleAdmin)
	adminLogin := doRequest(t, f.router, "POST", "/api/v1/auth/login", "", map[string]any{"email": "boss2@x.com", "password": "boss1234"})
	var adminSess LoginResponse
	_ = json.Unmarshal(adminLogin.Body.Bytes(), &adminSess)
	_ = doRequest(t, f.router, "POST", "/api/v1/users", adminSess.AccessToken, map[string]any{
		"nama": "Korban", "email": "korban@x.com", "peran": "akuntan", "sppg_id": 1, "password_awal": "korban12",
	})
	clearWajibByEmail(t, f, "korban@x.com")
	login := doRequest(t, f.router, "POST", "/api/v1/auth/login", "", map[string]any{"email": "korban@x.com", "password": "korban12"})
	var first LoginResponse
	_ = json.Unmarshal(login.Body.Bytes(), &first)
	rotatedReq := doRequest(t, f.router, "POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": first.RefreshToken})
	var second RefreshResponse
	_ = json.Unmarshal(rotatedReq.Body.Bytes(), &second)
	// Attacker replays the old token.
	replay := doRequest(t, f.router, "POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": first.RefreshToken})
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replay: got %d", replay.Code)
	}
	// Theft detection revokes everything: the rotated token dies too.
	after := doRequest(t, f.router, "POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": second.RefreshToken})
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("post-theft refresh: got %d", after.Code)
	}
}

func TestLogoutAllRevokesEverySession(t *testing.T) {
	f := newHTTPFixture()
	_ = seedAdminAndLogin(t, f, ptrInt(1), "boss3@x.com", "boss1234", domain.RoleAdmin)
	adminLogin := doRequest(t, f.router, "POST", "/api/v1/auth/login", "", map[string]any{"email": "boss3@x.com", "password": "boss1234"})
	var adminSess LoginResponse
	_ = json.Unmarshal(adminLogin.Body.Bytes(), &adminSess)
	_ = doRequest(t, f.router, "POST", "/api/v1/users", adminSess.AccessToken, map[string]any{
		"nama": "Multi", "email": "multi@x.com", "peran": "akuntan", "sppg_id": 1, "password_awal": "multi123",
	})
	clearWajibByEmail(t, f, "multi@x.com")
	l1 := doRequest(t, f.router, "POST", "/api/v1/auth/login", "", map[string]any{"email": "multi@x.com", "password": "multi123"})
	var s1 LoginResponse
	_ = json.Unmarshal(l1.Body.Bytes(), &s1)
	l2 := doRequest(t, f.router, "POST", "/api/v1/auth/login", "", map[string]any{"email": "multi@x.com", "password": "multi123"})
	var s2 LoginResponse
	_ = json.Unmarshal(l2.Body.Bytes(), &s2)
	all := doRequest(t, f.router, "POST", "/api/v1/auth/logout?all=true", s1.AccessToken, nil)
	if all.Code != http.StatusNoContent {
		t.Fatalf("logout all: got %d %s", all.Code, all.Body.String())
	}
	for i, tok := range []string{s1.RefreshToken, s2.RefreshToken} {
		w := doRequest(t, f.router, "POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": tok})
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("session %d alive after logout-all: %d", i, w.Code)
		}
	}
}

func TestUpdatePasswordAndAuditRBAC(t *testing.T) {
	f := newHTTPFixture()
	adminToken := seedAdminAndLogin(t, f, ptrInt(1), "admin2@x.com", "admin1234", domain.RoleAdmin)

	created := doRequest(t, f.router, "POST", "/api/v1/users", adminToken, map[string]any{
		"nama": "Dapur B", "email": "dapurb@x.com", "peran": "petugas_dapur", "sppg_id": 1, "password_awal": "dapur123",
	})
	var cu CreateUserResponse
	_ = json.Unmarshal(created.Body.Bytes(), &cu)

	patch := doRequest(t, f.router, "PATCH", "/api/v1/users/"+itoa(cu.User.ID), adminToken, map[string]any{"nama": "Dapur B Updated"})
	if patch.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", patch.Code, patch.Body.String())
	}

	// Login as the user and change own password.
	login := doRequest(t, f.router, "POST", "/api/v1/auth/login", "", map[string]any{"email": "dapurb@x.com", "password": "dapur123"})
	var sess LoginResponse
	_ = json.Unmarshal(login.Body.Bytes(), &sess)
	weak := doRequest(t, f.router, "PUT", "/api/v1/auth/password", sess.AccessToken, map[string]any{"password_lama": "dapur123", "password_baru": "lemah"})
	if weak.Code != http.StatusBadRequest {
		t.Fatalf("weak pw: got %d", weak.Code)
	}
	same := doRequest(t, f.router, "PUT", "/api/v1/auth/password", sess.AccessToken, map[string]any{"password_lama": "dapur123", "password_baru": "dapur123"})
	if same.Code != http.StatusBadRequest {
		t.Fatalf("reused pw: got %d", same.Code)
	}
	wrongOld := doRequest(t, f.router, "PUT", "/api/v1/auth/password", sess.AccessToken, map[string]any{"password_lama": "salah123", "password_baru": "baru1234"})
	if wrongOld.Code != http.StatusUnauthorized {
		t.Fatalf("wrong old pw: got %d", wrongOld.Code)
	}
	pw := doRequest(t, f.router, "PUT", "/api/v1/auth/password", sess.AccessToken, map[string]any{"password_lama": "dapur123", "password_baru": "baru1234"})
	if pw.Code != http.StatusNoContent {
		t.Fatalf("change pw: %d %s", pw.Code, pw.Body.String())
	}
	if pw.Body.Len() != 0 {
		t.Fatalf("change pw must have no body, got %q", pw.Body.String())
	}

	// Non-privileged cannot read audit logs.
	forbidden := doRequest(t, f.router, "GET", "/api/v1/audit-logs", sess.AccessToken, nil)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("audit forbidden: got %d", forbidden.Code)
	}
	allowed := doRequest(t, f.router, "GET", "/api/v1/audit-logs", adminToken, nil)
	if allowed.Code != http.StatusOK {
		t.Fatalf("audit allowed: %d %s", allowed.Code, allowed.Body.String())
	}
	var list AuditLogListResponse
	if err := json.Unmarshal(allowed.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Meta.Total == 0 || len(list.Data) == 0 {
		t.Fatal("expected audit rows")
	}
}

func TestUpdateUserResetSelfAndLastKepala(t *testing.T) {
	f := newHTTPFixture()
	adminToken := seedAdminAndLogin(t, f, ptrInt(1), "admin4@x.com", "admin1234", domain.RoleAdmin)

	// Admin cannot deactivate itself.
	adminMe := doRequest(t, f.router, "GET", "/api/v1/auth/me", adminToken, nil)
	var adminProfile MeResponse
	_ = json.Unmarshal(adminMe.Body.Bytes(), &adminProfile)
	selfOff := doRequest(t, f.router, "PATCH", "/api/v1/users/"+itoa(adminProfile.ID), adminToken, map[string]any{"aktif": false})
	if selfOff.Code != http.StatusForbidden {
		t.Fatalf("self deactivate: got %d %s", selfOff.Code, selfOff.Body.String())
	}

	// Create two kepala in SPPG 1: deactivating one is fine, the last is 409.
	k1 := doRequest(t, f.router, "POST", "/api/v1/users", adminToken, map[string]any{
		"nama": "Kepala Satu", "email": "kepala1@x.com", "peran": "kepala_sppg", "sppg_id": 1, "password_awal": "kepala123",
	})
	var k1Res CreateUserResponse
	_ = json.Unmarshal(k1.Body.Bytes(), &k1Res)
	k2 := doRequest(t, f.router, "POST", "/api/v1/users", adminToken, map[string]any{
		"nama": "Kepala Dua", "email": "kepala2@x.com", "peran": "kepala_sppg", "sppg_id": 1, "password_awal": "kepala123",
	})
	var k2Res CreateUserResponse
	_ = json.Unmarshal(k2.Body.Bytes(), &k2Res)

	off1 := doRequest(t, f.router, "PATCH", "/api/v1/users/"+itoa(k1Res.User.ID), adminToken, map[string]any{"aktif": false})
	if off1.Code != http.StatusOK {
		t.Fatalf("deactivate first kepala: %d %s", off1.Code, off1.Body.String())
	}
	offLast := doRequest(t, f.router, "PATCH", "/api/v1/users/"+itoa(k2Res.User.ID), adminToken, map[string]any{"aktif": false})
	if offLast.Code != http.StatusConflict {
		t.Fatalf("deactivate last kepala: got %d %s", offLast.Code, offLast.Body.String())
	}
	demoteLast := doRequest(t, f.router, "PATCH", "/api/v1/users/"+itoa(k2Res.User.ID), adminToken, map[string]any{"peran": "akuntan"})
	if demoteLast.Code != http.StatusConflict {
		t.Fatalf("demote last kepala: got %d %s", demoteLast.Code, demoteLast.Body.String())
	}

	// Reset password returns a temporary password once.
	reset := doRequest(t, f.router, "PATCH", "/api/v1/users/"+itoa(k2Res.User.ID), adminToken, map[string]any{"reset_password": true})
	if reset.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", reset.Code, reset.Body.String())
	}
	var resetRes UpdateUserResponse
	if err := json.Unmarshal(reset.Body.Bytes(), &resetRes); err != nil {
		t.Fatal(err)
	}
	if resetRes.PasswordSementara == nil || *resetRes.PasswordSementara == "" {
		t.Fatal("expected password_sementara")
	}
	// Temp password works for login and forces wajib flag.
	login := doRequest(t, f.router, "POST", "/api/v1/auth/login", "", map[string]any{"email": "kepala2@x.com", "password": *resetRes.PasswordSementara})
	if login.Code != http.StatusOK {
		t.Fatalf("login with temp pw: %d %s", login.Code, login.Body.String())
	}
	var sess LoginResponse
	_ = json.Unmarshal(login.Body.Bytes(), &sess)
	if !sess.WajibGantiPassword {
		t.Fatal("reset must set wajib_ganti_password")
	}
}

func TestRoleMatrixTable(t *testing.T) {
	tests := []struct {
		name       string
		role       domain.Role
		sppg       *int64
		method     string
		path       string
		withBody   bool
		wantStatus int
	}{
		{"admin creates user", domain.RoleAdmin, nil, "POST", "/api/v1/users", true, http.StatusCreated},
		{"kepala creates own-sppg user", domain.RoleKepalaSPPG, ptrInt(1), "POST", "/api/v1/users", true, http.StatusCreated},
		{"kepala cannot create other-sppg user", domain.RoleKepalaSPPG, ptrInt(1), "POST", "/api/v1/users-other", true, http.StatusForbidden},
		{"akuntan cannot manage users", domain.RoleAkuntan, ptrInt(1), "POST", "/api/v1/users", true, http.StatusForbidden},
		{"pengawas cannot manage users", domain.RolePengawas, ptrInt(1), "POST", "/api/v1/users", true, http.StatusForbidden},
		{"admin reads audit", domain.RoleAdmin, nil, "GET", "/api/v1/audit-logs", false, http.StatusOK},
		{"kepala reads audit", domain.RoleKepalaSPPG, ptrInt(1), "GET", "/api/v1/audit-logs", false, http.StatusOK},
		{"pengawas reads audit", domain.RolePengawas, ptrInt(1), "GET", "/api/v1/audit-logs", false, http.StatusOK},
		{"akuntan cannot read audit", domain.RoleAkuntan, ptrInt(1), "GET", "/api/v1/audit-logs", false, http.StatusForbidden},
		{"petugas_dapur cannot read audit", domain.RolePetugasDapur, ptrInt(1), "GET", "/api/v1/audit-logs", false, http.StatusForbidden},
		{"pengawas cannot PATCH users", domain.RolePengawas, ptrInt(1), "PATCH", "/api/v1/users/1", true, http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newHTTPFixture()
			email := "matrix-tok@example.com"
			pw := "matrix12"
			ctx := context.Background()
			created, err := f.svc.CreateUser(ctx, &domain.Claims{UserID: 999, Role: domain.RoleAdmin}, domain.CreateUserInput{
				Nama: "Matrix T", Email: email, Peran: tc.role, SPPGID: tc.sppg, PasswordAwal: &pw,
			}, nil)
			if err != nil {
				t.Fatalf("seed: %v", err)
			}
			clearWajib(t, f, created.User.ID)
			login, err := f.svc.Login(ctx, email, pw, nil, nil)
			if err != nil {
				t.Fatalf("login: %v", err)
			}
			path := tc.path
			var body any
			if tc.withBody {
				body = map[string]any{
					"nama": "Matrix User", "email": "matrix-new@x.com", "peran": "akuntan",
					"sppg_id": 1, "password_awal": "matrix12",
				}
				if path == "/api/v1/users-other" {
					path = "/api/v1/users"
					body = map[string]any{
						"nama": "Matrix User", "email": "matrix-new@x.com", "peran": "akuntan",
						"sppg_id": 2, "password_awal": "matrix12",
					}
				}
			}
			w := doRequest(t, f.router, tc.method, path, login.AccessToken, body)
			if w.Code != tc.wantStatus {
				t.Fatalf("got %d want %d: %s", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}

func TestAuditLogEnrichedShapeAndFilters(t *testing.T) {
	f := newHTTPFixture()
	adminToken := seedAdminAndLogin(t, f, ptrInt(1), "admin5@x.com", "admin1234", domain.RoleAdmin)

	created := doRequest(t, f.router, "POST", "/api/v1/users", adminToken, map[string]any{
		"nama": "Audit Me", "email": "auditme@x.com", "peran": "akuntan", "sppg_id": 1, "password_awal": "audit123",
	})
	var cu CreateUserResponse
	_ = json.Unmarshal(created.Body.Bytes(), &cu)

	// Enriched shape: nested user + data fields.
	list := doRequest(t, f.router, "GET", "/api/v1/audit-logs", adminToken, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list: %d %s", list.Code, list.Body.String())
	}
	var res AuditLogListResponse
	if err := json.Unmarshal(list.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Data) == 0 {
		t.Fatal("expected audit rows")
	}
	// Entries attributed to deleted/unknown users may lack a nested user,
	// but present ones must be complete.
	seenUser := false
	for _, item := range res.Data {
		if item.User == nil {
			continue
		}
		seenUser = true
		if item.User.Nama == "" || item.User.Peran == "" {
			t.Fatalf("incomplete nested user: %+v", item)
		}
	}
	// A real login produces an entry with its actor enriched.
	_ = doRequest(t, f.router, "POST", "/api/v1/auth/login", "", map[string]any{"email": "auditme@x.com", "password": "audit123"})
	logins := doRequest(t, f.router, "GET", "/api/v1/audit-logs?aksi=login", adminToken, nil)
	var loginRes AuditLogListResponse
	_ = json.Unmarshal(logins.Body.Bytes(), &loginRes)
	if len(loginRes.Data) == 0 {
		t.Fatal("expected login entries")
	}
	for _, item := range loginRes.Data {
		if item.User == nil || item.User.Nama == "" {
			t.Fatalf("login entry missing actor: %+v", item)
		}
	}
	if !seenUser {
		t.Fatal("expected at least one entry with nested user")
	}
	// record_id filter.
	byRecord := doRequest(t, f.router, "GET", "/api/v1/audit-logs?record_id="+itoa(cu.User.ID), adminToken, nil)
	var byRecRes AuditLogListResponse
	_ = json.Unmarshal(byRecord.Body.Bytes(), &byRecRes)
	if byRecord.Code != http.StatusOK || len(byRecRes.Data) == 0 {
		t.Fatalf("record filter: %d %s", byRecord.Code, byRecord.Body.String())
	}
	// Date range filter (wide) + reversed range 400 + invalid 400.
	wide := doRequest(t, f.router, "GET", "/api/v1/audit-logs?dari=2000-01-01&sampai=2100-01-01", adminToken, nil)
	if wide.Code != http.StatusOK {
		t.Fatalf("wide range: %d", wide.Code)
	}
	reversed := doRequest(t, f.router, "GET", "/api/v1/audit-logs?dari=2100-01-01&sampai=2000-01-01", adminToken, nil)
	if reversed.Code != http.StatusBadRequest {
		t.Fatalf("reversed range: got %d", reversed.Code)
	}
	bad := doRequest(t, f.router, "GET", "/api/v1/audit-logs?dari=not-a-date", adminToken, nil)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad date: got %d", bad.Code)
	}
	badID := doRequest(t, f.router, "GET", "/api/v1/audit-logs?user_id=abc", adminToken, nil)
	if badID.Code != http.StatusBadRequest {
		t.Fatalf("bad user_id: got %d", badID.Code)
	}
}

func TestUnauthenticatedAndDenyAudit(t *testing.T) {
	f := newHTTPFixture()
	_ = seedAdminAndLogin(t, f, ptrInt(1), "admin6@x.com", "admin1234", domain.RoleAdmin)
	protected := []struct {
		method, path string
		body         any
	}{
		{"POST", "/api/v1/users", map[string]any{}},
		{"PATCH", "/api/v1/users/1", map[string]any{}},
		{"POST", "/api/v1/auth/logout", map[string]any{}},
		{"GET", "/api/v1/auth/me", nil},
		{"PUT", "/api/v1/auth/password", map[string]any{}},
		{"GET", "/api/v1/audit-logs", nil},
	}
	for _, p := range protected {
		w := doRequest(t, f.router, p.method, p.path, "", p.body)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: got %d want 401", p.method, p.path, w.Code)
		}
	}
	// A forbidden role action leaves a DENY entry readable via aksi=deny.
	ctx := context.Background()
	created, err := f.svc.CreateUser(ctx, &domain.Claims{UserID: 999, Role: domain.RoleAdmin}, domain.CreateUserInput{
		Nama: "Peon", Email: "peon@x.com", Peran: domain.RoleAkuntan, SPPGID: ptrInt(1), PasswordAwal: ptrStr("peon1234"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	clearWajib(t, f, created.User.ID)
	login, err := f.svc.Login(ctx, "peon@x.com", "peon1234", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := doRequest(t, f.router, "GET", "/api/v1/audit-logs", login.AccessToken, nil)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden: got %d", forbidden.Code)
	}
	adminToken := seedAdminAndLogin(t, f, nil, "admin7@x.com", "admin1234", domain.RoleAdmin)
	deny := doRequest(t, f.router, "GET", "/api/v1/audit-logs?aksi=DENY", adminToken, nil)
	var denyRes AuditLogListResponse
	_ = json.Unmarshal(deny.Body.Bytes(), &denyRes)
	if deny.Code != http.StatusOK || len(denyRes.Data) == 0 {
		t.Fatalf("deny audit: %d %s", deny.Code, deny.Body.String())
	}
}

func ptrInt(v int64) *int64 { return &v }

func ptrStr(v string) *string { return &v }

func itoa(v int64) string {
	return json.Number(itoaStr(v)).String()
}

func itoaStr(v int64) string {
	// Avoid strconv import cycle confusion; simple format.
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

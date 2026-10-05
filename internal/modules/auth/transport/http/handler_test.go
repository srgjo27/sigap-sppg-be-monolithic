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
	mw := NewMiddleware(issuer)
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
	_, err := f.svc.CreateUser(ctx, adminClaims, domain.CreateUserInput{
		Nama: "Seed User", Email: email, Peran: role, SPPGID: sppg, PasswordAwal: &password,
	}, nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	login, err := f.svc.Login(ctx, email, password, nil, nil)
	if err != nil {
		t.Fatalf("seed login: %v", err)
	}
	return login.AccessToken
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
	pw := doRequest(t, f.router, "PUT", "/api/v1/auth/password", sess.AccessToken, map[string]any{"password_lama": "dapur123", "password_baru": "baru1234"})
	if pw.Code != http.StatusOK {
		t.Fatalf("change pw: %d %s", pw.Code, pw.Body.String())
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

func ptrInt(v int64) *int64 { return &v }

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

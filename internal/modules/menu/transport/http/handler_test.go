package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	authapplication "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/application"
	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	authinfra "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/infrastructure"
	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
	menuapplication "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/application"
	menuinfra "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/infrastructure"
)

func init() { gin.SetMode(gin.TestMode) }

type fixture struct {
	router    *gin.Engine
	authSvc   *authapplication.Service
	authStore *authinfra.MemoryStores
	menuStore *menuinfra.MemoryStores
}

func newFixture() *fixture {
	authStore := authinfra.NewMemoryStores()
	authStore.SeedSPPGWithName(1, "SPPG Cempaka")
	authStore.SeedSekolahWithName(10, 1, "SDN 01")
	issuer := authinfra.NewJWTIssuer("test-secret")
	authSvc := authapplication.New(authapplication.Deps{
		Users: authStore, Refresh: authStore, Audits: authStore, SPPG: authStore, Sekolah: authStore,
		Tokens: issuer, Hasher: authinfra.NewBcryptHasher(4),
	})
	authHandler := authhttp.NewHandler(authSvc)
	mw := authhttp.NewMiddleware(issuer, authSvc)

	menuStore := menuinfra.NewMemoryStores()
	cap := 100
	menuStore.SeedSPPG(1, &cap, 250)
	menuSvc := menuapplication.New(menuapplication.Deps{
		Bahan: menuStore, Menus: menuStore, SPPG: menuStore, Sekolah: menuStore, Audits: menuStore,
	})
	menuHandler := NewHandler(menuSvc)

	r := gin.New()
	v1 := r.Group("/api/v1")
	authhttp.RegisterRoutes(v1, authHandler, mw)
	RegisterRoutes(v1, menuHandler, mw)
	return &fixture{router: r, authSvc: authSvc, authStore: authStore, menuStore: menuStore}
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
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func seedUser(t *testing.T, f *fixture, email, password string, role authdomain.Role, sppg *int64) string {
	t.Helper()
	ctx := context.Background()
	created, err := f.authSvc.CreateUser(ctx, &authdomain.Claims{UserID: 999, Role: authdomain.RoleAdmin}, authdomain.CreateUserInput{
		Nama: "Seed " + string(role), Email: email, Peran: role, SPPGID: sppg, PasswordAwal: &password,
	}, nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	u, _ := f.authStore.FindByID(ctx, created.User.ID)
	u.WajibGantiPassword = false
	if _, err := f.authStore.Update(ctx, u); err != nil {
		t.Fatal(err)
	}
	login, err := f.authSvc.Login(ctx, email, password, nil, nil)
	if err != nil {
		t.Fatalf("seed login: %v", err)
	}
	return login.AccessToken
}

func ptrInt64(v int64) *int64 { return &v }

func TestBahanCRUDAndRBAC(t *testing.T) {
	f := newFixture()
	adminToken := seedUser(t, f, "admin@x.com", "admin1234", authdomain.RoleAdmin, nil)
	giziToken := seedUser(t, f, "gizi@x.com", "gizi1234", authdomain.RoleAhliGizi, ptrInt64(1))
	akuntanToken := seedUser(t, f, "akun@x.com", "akun1234", authdomain.RoleAkuntan, ptrInt64(1))

	created := doRequest(t, f.router, "POST", "/api/v1/bahan", giziToken, map[string]any{
		"nama": "Beras", "kategori": "karbohidrat", "satuan": "kg",
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var b BahanResponse
	if err := json.Unmarshal(created.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if b.GramPerSatuan != 1000 || !b.Aktif {
		t.Fatalf("defaults wrong: %+v", b)
	}

	dup := doRequest(t, f.router, "POST", "/api/v1/bahan", adminToken, map[string]any{
		"nama": "BERAS", "kategori": "karbohidrat", "satuan": "kg",
	})
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate: got %d %s", dup.Code, dup.Body.String())
	}

	perishable := doRequest(t, f.router, "POST", "/api/v1/bahan", giziToken, map[string]any{
		"nama": "Ayam", "kategori": "protein_hewani", "satuan": "kg", "mudah_rusak": true,
	})
	if perishable.Code != http.StatusBadRequest {
		t.Fatalf("perishable: got %d %s", perishable.Code, perishable.Body.String())
	}

	forbidden := doRequest(t, f.router, "POST", "/api/v1/bahan", akuntanToken, map[string]any{
		"nama": "Gula", "kategori": "lainnya", "satuan": "kg",
	})
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("write forbidden: got %d", forbidden.Code)
	}

	list := doRequest(t, f.router, "GET", "/api/v1/bahan", akuntanToken, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list: %d %s", list.Code, list.Body.String())
	}
	var lr BahanListResponse
	if err := json.Unmarshal(list.Body.Bytes(), &lr); err != nil {
		t.Fatal(err)
	}
	if lr.Meta.Total == 0 || len(lr.Data) == 0 {
		t.Fatal("expected bahan rows")
	}

	patch := doRequest(t, f.router, "PATCH", "/api/v1/bahan/"+itoa(b.ID), adminToken, map[string]any{"aktif": false})
	if patch.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", patch.Code, patch.Body.String())
	}
	filtered := doRequest(t, f.router, "GET", "/api/v1/bahan?aktif=true", giziToken, nil)
	var fr BahanListResponse
	_ = json.Unmarshal(filtered.Body.Bytes(), &fr)
	for _, item := range fr.Data {
		if item.ID == b.ID {
			t.Fatal("deactivated bahan must be hidden with aktif=true")
		}
	}

	missing := doRequest(t, f.router, "PATCH", "/api/v1/bahan/9999", adminToken, map[string]any{"nama": "Xyz"})
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing: got %d", missing.Code)
	}
	unauth := doRequest(t, f.router, "GET", "/api/v1/bahan", "", nil)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: got %d", unauth.Code)
	}
}

func TestCreateMenuEndpoint(t *testing.T) {
	f := newFixture()
	giziToken := seedUser(t, f, "gizi2@x.com", "gizi1234", authdomain.RoleAhliGizi, ptrInt64(1))
	adminToken := seedUser(t, f, "admin2@x.com", "admin1234", authdomain.RoleAdmin, nil)

	beras := doRequest(t, f.router, "POST", "/api/v1/bahan", giziToken, map[string]any{
		"nama": "Beras", "kategori": "karbohidrat", "satuan": "kg",
	})
	var berasRes BahanResponse
	_ = json.Unmarshal(beras.Body.Bytes(), &berasRes)
	telur := doRequest(t, f.router, "POST", "/api/v1/bahan", giziToken, map[string]any{
		"nama": "Telur", "kategori": "protein_hewani", "satuan": "butir", "gram_per_satuan": 60,
	})
	var telurRes BahanResponse
	_ = json.Unmarshal(telur.Body.Bytes(), &telurRes)

	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	dayAfter := time.Now().Add(48 * time.Hour).Format("2006-01-02")
	payload := map[string]any{
		"tanggal": tomorrow, "nama_menu": "Nasi Ayam Bergizi",
		"energi_kkal": 500, "protein_g": 20, "karbohidrat_g": 60, "lemak_g": 15,
		"bahan": []map[string]any{
			{"bahan_id": berasRes.ID, "gram_per_porsi": 100},
			{"bahan_id": telurRes.ID, "gram_per_porsi": 50},
		},
	}
	created := doRequest(t, f.router, "POST", "/api/v1/menus", giziToken, payload)
	if created.Code != http.StatusCreated {
		t.Fatalf("create menu: %d %s", created.Code, created.Body.String())
	}
	var menu CreateMenuResponse
	if err := json.Unmarshal(created.Body.Bytes(), &menu); err != nil {
		t.Fatal(err)
	}
	if menu.Status != "draf" || menu.TargetPorsi != 250 || len(menu.Bahan) != 2 || len(menu.Warnings) != 1 {
		t.Fatalf("menu payload wrong: %+v", menu)
	}
	if menu.Gizi.EnergiKkal != 500 || menu.DibuatOleh == 0 {
		t.Fatalf("gizi/owner wrong: %+v", menu)
	}

	second := doRequest(t, f.router, "POST", "/api/v1/menus", giziToken, payload)
	if second.Code != http.StatusConflict {
		t.Fatalf("second same date: got %d %s", second.Code, second.Body.String())
	}

	forbidden := doRequest(t, f.router, "POST", "/api/v1/menus", adminToken, payload)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-gizi: got %d", forbidden.Code)
	}

	badBahan := map[string]any{
		"tanggal": dayAfter, "nama_menu": "Menu Rusak",
		"energi_kkal": 400, "protein_g": 15, "karbohidrat_g": 50, "lemak_g": 10,
		"bahan": []map[string]any{{"bahan_id": 9999, "gram_per_porsi": 10}},
	}
	missing := doRequest(t, f.router, "POST", "/api/v1/menus", giziToken, badBahan)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown bahan: got %d %s", missing.Code, missing.Body.String())
	}

	past := map[string]any{
		"tanggal": "2000-01-01", "nama_menu": "Menu Lama",
		"energi_kkal": 400, "protein_g": 15, "karbohidrat_g": 50, "lemak_g": 10,
		"bahan": []map[string]any{{"bahan_id": berasRes.ID, "gram_per_porsi": 10}},
	}
	pastRes := doRequest(t, f.router, "POST", "/api/v1/menus", giziToken, past)
	if pastRes.Code != http.StatusBadRequest {
		t.Fatalf("past date: got %d", pastRes.Code)
	}
}

func TestBahanPicSekolahDenied(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	f.authStore.SeedSekolahWithName(20, 1, "SDN 02")
	created, err := f.authSvc.CreateUser(ctx, &authdomain.Claims{UserID: 999, Role: authdomain.RoleAdmin}, authdomain.CreateUserInput{
		Nama: "PIC", Email: "pic@x.com", Peran: authdomain.RolePICsekolah, SPPGID: ptrInt64(1), SekolahID: ptrInt64(20), PasswordAwal: ptrStr("pic12345"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := f.authStore.FindByID(ctx, created.User.ID)
	u.WajibGantiPassword = false
	_, _ = f.authStore.Update(ctx, u)
	login, err := f.authSvc.Login(ctx, "pic@x.com", "pic12345", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := doRequest(t, f.router, "GET", "/api/v1/bahan", login.AccessToken, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("pic_sekolah read: got %d", w.Code)
	}
}

func itoa(v int64) string { return json.Number(itoaStr(v)).String() }

func itoaStr(v int64) string {
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

func ptrStr(v string) *string { return &v }

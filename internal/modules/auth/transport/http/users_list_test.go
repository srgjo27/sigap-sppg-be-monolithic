package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

func seedListUser(t *testing.T, f *httpFixture, nama, email string, role domain.Role, sppg *int64) int64 {
	t.Helper()
	pw := "user1234"
	created, err := f.svc.CreateUser(context.Background(), &domain.Claims{UserID: 999, Role: domain.RoleAdmin}, domain.CreateUserInput{
		Nama: nama, Email: email, Peran: role, SPPGID: sppg, PasswordAwal: &pw,
	}, nil)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return created.User.ID
}

func TestListUsersEndpoint(t *testing.T) {
	f := newHTTPFixture()
	f.stores.SeedSPPG(2)
	adminToken := seedAdminAndLogin(t, f, nil, "admin@x.com", "admin1234", domain.RoleAdmin)
	kepalaToken := seedAdminAndLogin(t, f, ptrInt(1), "kepala@x.com", "kepala1234", domain.RoleKepalaSPPG)
	seedListUser(t, f, "Dapur Pagi", "pagi@x.com", domain.RolePetugasDapur, ptrInt(1))
	seedListUser(t, f, "Akun Audit", "audit@x.com", domain.RoleAkuntan, ptrInt(1))
	seedListUser(t, f, "Luar Dua", "luar@x.com", domain.RoleAkuntan, ptrInt(2))

	list := doRequest(t, f.router, "GET", "/api/v1/users", adminToken, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list: %d %s", list.Code, list.Body.String())
	}
	var res UserListResponse
	if err := json.Unmarshal(list.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Meta.Total != 5 || len(res.Data) != 5 {
		t.Fatalf("admin must see 5, got %+v", res.Meta)
	}
	if res.Data[0].Nama != "Akun Audit" {
		t.Fatalf("default sort nama wrong, first = %q", res.Data[0].Nama)
	}

	// List rows carry exactly the display columns.
	var raw map[string]any
	if err := json.Unmarshal(list.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	wantKeys := map[string]bool{"nama": true, "email": true, "peran": true, "penempatan": true, "status": true, "log_terakhir": true}
	rows, ok := raw["data"].([]any)
	if !ok || len(rows) != 5 {
		t.Fatalf("data rows wrong: %v", raw["data"])
	}
	for _, item := range rows {
		obj := item.(map[string]any)
		if len(obj) != len(wantKeys) {
			t.Fatalf("row must have exactly %d keys, got %+v", len(wantKeys), obj)
		}
		for k := range obj {
			if !wantKeys[k] {
				t.Fatalf("unexpected key %q in %+v", k, obj)
			}
		}
	}
	// penempatan resolves the SPPG name; accounts without SPPG are null.
	found := false
	for _, item := range rows {
		obj := item.(map[string]any)
		if obj["email"] == "audit@x.com" {
			found = true
			if obj["penempatan"] != "SPPG Cempaka" {
				t.Fatalf("penempatan wrong: %+v", obj)
			}
			if obj["status"] != true {
				t.Fatalf("status wrong: %+v", obj)
			}
		}
		if obj["email"] == "admin@x.com" && obj["penempatan"] != nil {
			t.Fatalf("sppg-less account penempatan must be null: %+v", obj)
		}
	}
	if !found {
		t.Fatal("expected audit@x.com row")
	}

	// kepala_sppg sees own SPPG only; sppg_id filter is ignored.
	klist := doRequest(t, f.router, "GET", "/api/v1/users", kepalaToken, nil)
	var kres UserListResponse
	_ = json.Unmarshal(klist.Body.Bytes(), &kres)
	if klist.Code != http.StatusOK || kres.Meta.Total != 3 {
		t.Fatalf("kepala must see 3, got %d %+v", klist.Code, kres.Meta)
	}
	ignored := doRequest(t, f.router, "GET", "/api/v1/users?sppg_id=2", kepalaToken, nil)
	var ires UserListResponse
	_ = json.Unmarshal(ignored.Body.Bytes(), &ires)
	if ignored.Code != http.StatusOK || ires.Meta.Total != 3 {
		t.Fatalf("kepala sppg_id ignored: %d %+v", ignored.Code, ires.Meta)
	}

	// Filters.
	q := doRequest(t, f.router, "GET", "/api/v1/users?q=DAPUR", adminToken, nil)
	var qres UserListResponse
	_ = json.Unmarshal(q.Body.Bytes(), &qres)
	if q.Code != http.StatusOK || qres.Meta.Total != 1 {
		t.Fatalf("q filter: %d %+v", q.Code, qres.Meta)
	}
	multi := doRequest(t, f.router, "GET", "/api/v1/users?peran=petugas_dapur,akuntan", adminToken, nil)
	var mres UserListResponse
	_ = json.Unmarshal(multi.Body.Bytes(), &mres)
	if multi.Code != http.StatusOK || mres.Meta.Total != 3 {
		t.Fatalf("peran multi: %d %+v", multi.Code, mres.Meta)
	}
	paged := doRequest(t, f.router, "GET", "/api/v1/users?page=2&per_page=2", adminToken, nil)
	var pres UserListResponse
	_ = json.Unmarshal(paged.Body.Bytes(), &pres)
	if paged.Code != http.StatusOK || pres.Meta.Total != 5 || len(pres.Data) != 2 || pres.Meta.Page != 2 {
		t.Fatalf("pagination: %d %+v", paged.Code, pres.Meta)
	}

	// Invalid inputs are 400.
	for _, path := range []string{
		"/api/v1/users?q=x",
		"/api/v1/users?peran=siluman",
		"/api/v1/users?per_page=101",
		"/api/v1/users?aktif=bukan",
		"/api/v1/users?sppg_id=abc",
		"/api/v1/users?sort=email",
	} {
		if w := doRequest(t, f.router, "GET", path, adminToken, nil); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: got %d %s", path, w.Code, w.Body.String())
		}
	}

	// Forbidden roles, including pengawas.
	akunToken := seedAdminAndLogin(t, f, ptrInt(1), "akun@x.com", "akun1234", domain.RoleAkuntan)
	if w := doRequest(t, f.router, "GET", "/api/v1/users", akunToken, nil); w.Code != http.StatusForbidden {
		t.Fatalf("akuntan: got %d", w.Code)
	}
	wawasToken := seedAdminAndLogin(t, f, nil, "wawas@x.com", "wawas1234", domain.RolePengawas)
	if w := doRequest(t, f.router, "GET", "/api/v1/users", wawasToken, nil); w.Code != http.StatusForbidden {
		t.Fatalf("pengawas: got %d", w.Code)
	}
	if w := doRequest(t, f.router, "GET", "/api/v1/users", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: got %d", w.Code)
	}
}

func TestGetUserEndpoint(t *testing.T) {
	f := newHTTPFixture()
	adminToken := seedAdminAndLogin(t, f, nil, "admin@x.com", "admin1234", domain.RoleAdmin)
	kepalaToken := seedAdminAndLogin(t, f, ptrInt(1), "kepala@x.com", "kepala1234", domain.RoleKepalaSPPG)
	target := seedListUser(t, f, "Target User", "target@x.com", domain.RoleAkuntan, ptrInt(1))

	detail := doRequest(t, f.router, "GET", "/api/v1/users/"+itoa(target), adminToken, nil)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", detail.Code, detail.Body.String())
	}
	var res UserDetailResponse
	if err := json.Unmarshal(detail.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Email != "target@x.com" || res.SPPG == nil || res.SPPG.Nama != "SPPG Cempaka" {
		t.Fatalf("detail wrong: %+v", res)
	}
	if res.Terkunci || res.JumlahSesiAktif != 0 {
		t.Fatalf("fresh user flags wrong: %+v", res)
	}
	if !strings.Contains(detail.Body.String(), "updated_at") {
		t.Fatalf("detail must include updated_at: %s", detail.Body.String())
	}

	// kepala sees own-SPPG user; cross-SPPG and admin targets are 404.
	if w := doRequest(t, f.router, "GET", "/api/v1/users/"+itoa(target), kepalaToken, nil); w.Code != http.StatusOK {
		t.Fatalf("kepala own detail: got %d", w.Code)
	}
	f.stores.SeedSPPG(2)
	other := seedListUser(t, f, "Orang Luar", "luar@x.com", domain.RoleAkuntan, ptrInt(2))
	if w := doRequest(t, f.router, "GET", "/api/v1/users/"+itoa(other), kepalaToken, nil); w.Code != http.StatusNotFound {
		t.Fatalf("cross detail: got %d", w.Code)
	}
	boss := seedListUser(t, f, "Bos Admin", "bos@x.com", domain.RoleAdmin, ptrInt(1))
	if w := doRequest(t, f.router, "GET", "/api/v1/users/"+itoa(boss), kepalaToken, nil); w.Code != http.StatusNotFound {
		t.Fatalf("kepala/admin detail: got %d", w.Code)
	}
	if w := doRequest(t, f.router, "GET", "/api/v1/users/999999", adminToken, nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing: got %d", w.Code)
	}
	akunToken := seedAdminAndLogin(t, f, ptrInt(1), "akun@x.com", "akun1234", domain.RoleAkuntan)
	if w := doRequest(t, f.router, "GET", "/api/v1/users/"+itoa(target), akunToken, nil); w.Code != http.StatusForbidden {
		t.Fatalf("akuntan detail: got %d", w.Code)
	}
}

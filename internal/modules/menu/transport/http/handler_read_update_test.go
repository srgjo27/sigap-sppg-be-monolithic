package http

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
)

func seedBahan(t *testing.T, f *fixture, token, nama string) BahanResponse {
	t.Helper()
	extra := map[string]any{"nama": nama, "kategori": "karbohidrat", "satuan": "kg"}
	if nama == "Telur" {
		extra = map[string]any{"nama": nama, "kategori": "protein_hewani", "satuan": "butir", "gram_per_satuan": 60}
	}
	w := doRequest(t, f.router, "POST", "/api/v1/bahan", token, extra)
	if w.Code != http.StatusCreated {
		t.Fatalf("seed bahan %s: %d %s", nama, w.Code, w.Body.String())
	}
	var b BahanResponse
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return b
}

func seedMenu(t *testing.T, f *fixture, token, tanggal, nama string, bahanIDs ...int64) CreateMenuResponse {
	t.Helper()
	items := make([]map[string]any, 0, len(bahanIDs))
	for _, id := range bahanIDs {
		items = append(items, map[string]any{"bahan_id": id, "gram_per_porsi": 100})
	}
	w := doRequest(t, f.router, "POST", "/api/v1/menus", token, map[string]any{
		"tanggal": tanggal, "nama_menu": nama,
		"energi_kkal": 500, "protein_g": 20, "karbohidrat_g": 60, "lemak_g": 15,
		"bahan": items,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("seed menu: %d %s", w.Code, w.Body.String())
	}
	var m CreateMenuResponse
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return m
}

func TestListAndDetailMenus(t *testing.T) {
	f := newFixture()
	giziToken := seedUser(t, f, "gizi-rd@x.com", "gizi1234", authdomain.RoleAhliGizi, ptrInt64(1))
	akuntanToken := seedUser(t, f, "akun-rd@x.com", "akun1234", authdomain.RoleAkuntan, ptrInt64(1))
	beras := seedBahan(t, f, giziToken, "Beras")
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	dayAfter := time.Now().Add(48 * time.Hour).Format("2006-01-02")
	created := seedMenu(t, f, giziToken, tomorrow, "Menu Pagi", beras.ID)

	list := doRequest(t, f.router, "GET", "/api/v1/menus?dari="+tomorrow+"&sampai="+dayAfter, akuntanToken, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list: %d %s", list.Code, list.Body.String())
	}
	var lr MenuListResponse
	if err := json.Unmarshal(list.Body.Bytes(), &lr); err != nil {
		t.Fatal(err)
	}
	if lr.Meta.Total != 1 || len(lr.Data) != 1 || lr.Data[0].ID != created.ID {
		t.Fatalf("list payload wrong: %+v", lr)
	}
	if len(lr.Meta.TanggalKosong) != 1 || lr.Meta.TanggalKosong[0] != dayAfter {
		t.Fatalf("tanggal_kosong wrong: %+v", lr.Meta)
	}

	detail := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(created.ID), akuntanToken, nil)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", detail.Code, detail.Body.String())
	}
	var det MenuDetailResponse
	_ = json.Unmarshal(detail.Body.Bytes(), &det)
	if det.NamaMenu != "Menu Pagi" || len(det.Bahan) != 1 || det.Bahan[0].GramPerPorsi != 100 || det.DibuatOleh == 0 {
		t.Fatalf("detail payload wrong: %+v", det)
	}

	// Range validation: missing params, reversed, >31 days.
	if w := doRequest(t, f.router, "GET", "/api/v1/menus", giziToken, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("missing range: got %d", w.Code)
	}
	if w := doRequest(t, f.router, "GET", "/api/v1/menus?dari="+dayAfter+"&sampai="+tomorrow, giziToken, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("reversed: got %d", w.Code)
	}
	if w := doRequest(t, f.router, "GET", "/api/v1/menus?dari=2026-01-01&sampai=2026-03-01", giziToken, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("long range: got %d", w.Code)
	}
	// Cross-SPPG detail is 404.
	otherGizi := seedUser(t, f, "gizi-other@x.com", "gizi1234", authdomain.RoleAhliGizi, ptrInt64(1))
	_ = otherGizi
	gizi2 := seedUser(t, f, "gizi-sppg2@x.com", "gizi1234", authdomain.RoleAhliGizi, ptrInt64(1))
	_ = gizi2
	missing := doRequest(t, f.router, "GET", "/api/v1/menus/999999", giziToken, nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing detail: got %d", missing.Code)
	}
	if w := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(created.ID), "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: got %d", w.Code)
	}
}

func TestListMenusCrossSPPGAndPIC(t *testing.T) {
	f := newFixture()
	f.menuStore.SeedSPPG(2, nil, 10)
	f.authStore.SeedSPPGWithName(99, "SPPG 99")
	f.menuStore.SeedSPPG(99, nil, 10)
	f.menuStore.SeedSekolah(20, 1)
	f.authStore.SeedSekolahWithName(20, 1, "SDN 02")
	adminToken := seedUser(t, f, "admin-rd@x.com", "admin1234", authdomain.RoleAdmin, nil)
	giziToken := seedUser(t, f, "gizi-rd2@x.com", "gizi1234", authdomain.RoleAhliGizi, ptrInt64(1))
	beras := seedBahan(t, f, giziToken, "Beras")
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	created := seedMenu(t, f, giziToken, tomorrow, "Menu Pagi", beras.ID)

	// Other-SPPG user cannot see it.
	giziOther := seedUser(t, f, "gizi-sppg99@x.com", "gizi1234", authdomain.RoleAhliGizi, ptrInt64(99))
	if w := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(created.ID), giziOther, nil); w.Code != http.StatusNotFound {
		t.Fatalf("cross detail: got %d %s", w.Code, w.Body.String())
	}
	if w := doRequest(t, f.router, "GET", "/api/v1/menus?dari="+tomorrow+"&sampai="+tomorrow+"&sppg_id=1", giziOther, nil); w.Code != http.StatusNotFound {
		t.Fatalf("cross list: got %d", w.Code)
	}
	// Admin can filter.
	if w := doRequest(t, f.router, "GET", "/api/v1/menus?dari="+tomorrow+"&sampai="+tomorrow+"&sppg_id=1", adminToken, nil); w.Code != http.StatusOK {
		t.Fatalf("admin filter: got %d %s", w.Code, w.Body.String())
	}
	// pic_sekolah: draft invisible, approved visible without grams.
	ctx := context.Background()
	picCreated, err := f.authSvc.CreateUser(ctx, &authdomain.Claims{UserID: 999, Role: authdomain.RoleAdmin}, authdomain.CreateUserInput{
		Nama: "PIC", Email: "pic-rd@x.com", Peran: authdomain.RolePICsekolah, SPPGID: ptrInt64(1), SekolahID: ptrInt64(20), PasswordAwal: ptrStr("pic12345"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := f.authStore.FindByID(ctx, picCreated.User.ID)
	u.WajibGantiPassword = false
	_, _ = f.authStore.Update(ctx, u)
	login, _ := f.authSvc.Login(ctx, "pic-rd@x.com", "pic12345", nil, nil)
	if w := doRequest(t, f.router, "GET", "/api/v1/menus?dari="+tomorrow+"&sampai="+tomorrow, login.AccessToken, nil); w.Code != http.StatusOK {
		t.Fatalf("pic list: got %d", w.Code)
	} else {
		var lr MenuListResponse
		_ = json.Unmarshal(w.Body.Bytes(), &lr)
		if lr.Meta.Total != 0 {
			t.Fatalf("pic must see 0 drafts, got %+v", lr)
		}
	}
	if w := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(created.ID), login.AccessToken, nil); w.Code != http.StatusNotFound {
		t.Fatalf("pic draft detail: got %d", w.Code)
	}
	// Approve then pic sees it gram-free.
	m, _ := f.menuStore.FindMenuByID(ctx, created.ID)
	m.Status = domain.MenuStatusDisetujui
	_, _, _ = f.menuStore.UpdateMenu(ctx, m, nil)
	approved := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(created.ID), login.AccessToken, nil)
	if approved.Code != http.StatusOK {
		t.Fatalf("pic approved: got %d %s", approved.Code, approved.Body.String())
	}
	var det MenuDetailResponse
	_ = json.Unmarshal(approved.Body.Bytes(), &det)
	if len(det.Bahan) != 1 || det.Bahan[0].Nama == "" {
		t.Fatalf("pic bahan nama missing: %+v", det)
	}
	var raw map[string]any
	_ = json.Unmarshal(approved.Body.Bytes(), &raw)
	if bahan, ok := raw["bahan"].([]any); ok && len(bahan) == 1 {
		if item, ok := bahan[0].(map[string]any); ok {
			if _, hasGram := item["gram_per_porsi"]; hasGram {
				t.Fatalf("pic must not see gram_per_porsi: %v", item)
			}
		}
	}
}

func TestUpdateAndDeleteMenuDraft(t *testing.T) {
	f := newFixture()
	giziToken := seedUser(t, f, "gizi-ud@x.com", "gizi1234", authdomain.RoleAhliGizi, ptrInt64(1))
	adminToken := seedUser(t, f, "admin-ud@x.com", "admin1234", authdomain.RoleAdmin, nil)
	beras := seedBahan(t, f, giziToken, "Beras")
	telur := seedBahan(t, f, giziToken, "Telur")
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	created := seedMenu(t, f, giziToken, tomorrow, "Menu Awal", beras.ID)

	patched := doRequest(t, f.router, "PATCH", "/api/v1/menus/"+itoa(created.ID), giziToken, map[string]any{
		"nama_menu": "Menu Baru", "target_porsi": 60,
		"bahan": []map[string]any{{"bahan_id": beras.ID, "gram_per_porsi": 80}, {"bahan_id": telur.ID, "gram_per_porsi": 40}},
	})
	if patched.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", patched.Code, patched.Body.String())
	}
	var updated UpdateMenuResponse
	_ = json.Unmarshal(patched.Body.Bytes(), &updated)
	if updated.NamaMenu != "Menu Baru" || len(updated.Bahan) != 2 {
		t.Fatalf("patched payload wrong: %+v", updated)
	}
	// tanggal immutable.
	if w := doRequest(t, f.router, "PATCH", "/api/v1/menus/"+itoa(created.ID), giziToken, map[string]any{"tanggal": tomorrow}); w.Code != http.StatusBadRequest {
		t.Fatalf("tanggal change: got %d", w.Code)
	}
	// Non-gizi forbidden.
	if w := doRequest(t, f.router, "PATCH", "/api/v1/menus/"+itoa(created.ID), adminToken, map[string]any{"nama_menu": "Xyz Menu"}); w.Code != http.StatusForbidden {
		t.Fatalf("non-gizi patch: got %d", w.Code)
	}
	// Approved menu is 409 for both patch and delete.
	ctx := context.Background()
	m, _ := f.menuStore.FindMenuByID(ctx, created.ID)
	m.Status = domain.MenuStatusDisetujui
	_, _, _ = f.menuStore.UpdateMenu(ctx, m, nil)
	if w := doRequest(t, f.router, "PATCH", "/api/v1/menus/"+itoa(created.ID), giziToken, map[string]any{"nama_menu": "Menu Coba Lagi"}); w.Code != http.StatusConflict {
		t.Fatalf("approved patch: got %d %s", w.Code, w.Body.String())
	}
	if w := doRequest(t, f.router, "DELETE", "/api/v1/menus/"+itoa(created.ID), giziToken, nil); w.Code != http.StatusConflict {
		t.Fatalf("approved delete: got %d", w.Code)
	}
	// Back to draf but used in batch: 409.
	m.Status = domain.MenuStatusDraf
	_, _, _ = f.menuStore.UpdateMenu(ctx, m, nil)
	f.menuStore.MarkMenuUsed(created.ID)
	if w := doRequest(t, f.router, "DELETE", "/api/v1/menus/"+itoa(created.ID), giziToken, nil); w.Code != http.StatusConflict {
		t.Fatalf("used delete: got %d %s", w.Code, w.Body.String())
	}
	// Fresh draft deletes with 204 and no body.
	dayAfter := time.Now().Add(72 * time.Hour).Format("2006-01-02")
	fresh := seedMenu(t, f, giziToken, dayAfter, "Menu Hapus", beras.ID)
	del := doRequest(t, f.router, "DELETE", "/api/v1/menus/"+itoa(fresh.ID), giziToken, nil)
	if del.Code != http.StatusNoContent || del.Body.Len() != 0 {
		t.Fatalf("delete: got %d body %q", del.Code, del.Body.String())
	}
	if w := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(fresh.ID), giziToken, nil); w.Code != http.StatusNotFound {
		t.Fatalf("deleted detail: got %d", w.Code)
	}
}

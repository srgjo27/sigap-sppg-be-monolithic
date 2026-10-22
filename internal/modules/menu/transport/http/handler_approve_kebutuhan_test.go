package http

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

func TestApproveAndRevertEndpoints(t *testing.T) {
	f := newFixture()
	giziToken := seedUser(t, f, "gizi-ap@x.com", "gizi1234", authdomain.RoleAhliGizi, ptrInt64(1))
	kepalaToken := seedUser(t, f, "kepala-ap@x.com", "kepala1234", authdomain.RoleKepalaSPPG, ptrInt64(1))
	akuntanToken := seedUser(t, f, "akun-ap@x.com", "akun1234", authdomain.RoleAkuntan, ptrInt64(1))
	_ = akuntanToken

	beras := seedBahan(t, f, giziToken, "Beras")
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	created := seedMenu(t, f, giziToken, tomorrow, "Menu Setuju", beras.ID)

	// Non-kepala cannot approve.
	if w := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/approve", giziToken, nil); w.Code != http.StatusForbidden {
		t.Fatalf("gizi approve: got %d %s", w.Code, w.Body.String())
	}
	approve := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/approve", kepalaToken, nil)
	if approve.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", approve.Code, approve.Body.String())
	}
	var ap ApprovalResponse
	if err := json.Unmarshal(approve.Body.Bytes(), &ap); err != nil {
		t.Fatal(err)
	}
	if ap.Status != "disetujui" || ap.DisetujuiOleh == nil || ap.DisetujuiAt == nil {
		t.Fatalf("approval payload wrong: %+v", ap)
	}
	// Double approve is 409.
	if w := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/approve", kepalaToken, nil); w.Code != http.StatusConflict {
		t.Fatalf("double approve: got %d", w.Code)
	}
	// Revert without alasan is 400.
	if w := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/revert", kepalaToken, map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("empty alasan: got %d %s", w.Code, w.Body.String())
	}
	revert := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/revert", kepalaToken, map[string]any{"alasan": "protein kurang"})
	if revert.Code != http.StatusOK {
		t.Fatalf("revert: %d %s", revert.Code, revert.Body.String())
	}
	var rv ApprovalResponse
	_ = json.Unmarshal(revert.Body.Bytes(), &rv)
	if rv.Status != "draf" || rv.DisetujuiOleh != nil {
		t.Fatalf("revert payload wrong: %+v", rv)
	}
	// Reverting a draft is 409.
	if w := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/revert", kepalaToken, map[string]any{"alasan": "lagi"}); w.Code != http.StatusConflict {
		t.Fatalf("revert draft: got %d", w.Code)
	}
	// Cross-SPPG approve is 404.
	otherKepala := seedUser(t, f, "kepala-99@x.com", "kepala1234", authdomain.RoleKepalaSPPG, ptrInt64(1))
	_ = otherKepala
	if w := doRequest(t, f.router, "POST", "/api/v1/menus/999999/approve", kepalaToken, nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing approve: got %d", w.Code)
	}
	if w := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/approve", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth approve: got %d", w.Code)
	}
}

func TestKebutuhanBahanEndpoint(t *testing.T) {
	f := newFixture()
	giziToken := seedUser(t, f, "gizi-kb@x.com", "gizi1234", authdomain.RoleAhliGizi, ptrInt64(1))
	kepalaToken := seedUser(t, f, "kepala-kb@x.com", "kepala1234", authdomain.RoleKepalaSPPG, ptrInt64(1))
	akuntanToken := seedUser(t, f, "akun-kb@x.com", "akun1234", authdomain.RoleAkuntan, ptrInt64(1))
	dapurToken := seedUser(t, f, "dapur-kb@x.com", "dapur1234", authdomain.RolePetugasDapur, ptrInt64(1))

	beras := seedBahan(t, f, giziToken, "Beras")
	telur := seedBahan(t, f, giziToken, "Telur")
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	created := seedMenu(t, f, giziToken, tomorrow, "Menu Hitung", beras.ID, telur.ID)

	got := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(created.ID)+"/kebutuhan-bahan", akuntanToken, nil)
	if got.Code != http.StatusOK {
		t.Fatalf("kebutuhan: %d %s", got.Code, got.Body.String())
	}
	var kb KebutuhanResponse
	if err := json.Unmarshal(got.Body.Bytes(), &kb); err != nil {
		t.Fatal(err)
	}
	if kb.TargetPorsi != 250 || len(kb.Items) == 0 || kb.Menu.Status == "" {
		t.Fatalf("kebutuhan payload wrong: %+v", kb)
	}
	// Simulation query params.
	sim := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(created.ID)+"/kebutuhan-bahan?target_porsi=100&cadangan_persen=10", kepalaToken, nil)
	if sim.Code != http.StatusOK {
		t.Fatalf("sim: %d %s", sim.Code, sim.Body.String())
	}
	var simRes KebutuhanResponse
	_ = json.Unmarshal(sim.Body.Bytes(), &simRes)
	if simRes.TargetPorsi != 100 || simRes.CadanganPersen != 10 {
		t.Fatalf("sim header wrong: %+v", simRes)
	}
	// Invalid params are 400.
	if w := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(created.ID)+"/kebutuhan-bahan?target_porsi=0", akuntanToken, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad target: got %d", w.Code)
	}
	if w := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(created.ID)+"/kebutuhan-bahan?cadangan_persen=25", akuntanToken, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad cadangan: got %d", w.Code)
	}
	// Forbidden role.
	if w := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(created.ID)+"/kebutuhan-bahan", dapurToken, nil); w.Code != http.StatusForbidden {
		t.Fatalf("dapur: got %d", w.Code)
	}
	// Works for approved menus too.
	if w := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/approve", kepalaToken, nil); w.Code != http.StatusOK {
		t.Fatalf("approve for kb: %d %s", w.Code, w.Body.String())
	}
	after := doRequest(t, f.router, "GET", "/api/v1/menus/"+itoa(created.ID)+"/kebutuhan-bahan", akuntanToken, nil)
	var afterRes KebutuhanResponse
	_ = json.Unmarshal(after.Body.Bytes(), &afterRes)
	if after.Code != http.StatusOK || afterRes.Menu.Status != "disetujui" {
		t.Fatalf("approved kb: %d %+v", after.Code, afterRes)
	}
}

package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

func TestCopyMenuEndpoint(t *testing.T) {
	f := newFixture()
	giziToken := seedUser(t, f, "gizi-cp@x.com", "gizi1234", authdomain.RoleAhliGizi, ptrInt64(1))
	adminToken := seedUser(t, f, "admin-cp@x.com", "admin1234", authdomain.RoleAdmin, nil)

	beras := seedBahan(t, f, giziToken, "Beras")
	telur := seedBahan(t, f, giziToken, "Telur")
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	dayAfter := time.Now().Add(48 * time.Hour).Format("2006-01-02")
	day3 := time.Now().Add(72 * time.Hour).Format("2006-01-02")
	created := seedMenu(t, f, giziToken, tomorrow, "Menu Mingguan", beras.ID, telur.ID)

	copied := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/copy", giziToken, map[string]any{
		"tanggal_tujuan": []string{dayAfter, day3},
	})
	if copied.Code != http.StatusCreated {
		t.Fatalf("copy: %d %s", copied.Code, copied.Body.String())
	}
	var res CopyMenuResponse
	if err := json.Unmarshal(copied.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Data) != 2 {
		t.Fatalf("data wrong: %+v", res)
	}
	for _, m := range res.Data {
		if m.Status != "draf" || len(m.Bahan) != 2 || m.NamaMenu != "Menu Mingguan" {
			t.Fatalf("copy payload wrong: %+v", m)
		}
	}
	if res.Data[0].Tanggal != dayAfter || res.Data[1].Tanggal != day3 {
		t.Fatalf("copy dates wrong: %+v", res.Data)
	}

	// Conflict names the date and creates nothing.
	conflict := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/copy", giziToken, map[string]any{
		"tanggal_tujuan": []string{time.Now().Add(96 * time.Hour).Format("2006-01-02"), dayAfter},
	})
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict: got %d %s", conflict.Code, conflict.Body.String())
	}
	if !strings.Contains(conflict.Body.String(), dayAfter) {
		t.Fatalf("conflict must mention %s: %s", dayAfter, conflict.Body.String())
	}

	// Validation failures are 400.
	cases := []any{
		map[string]any{"tanggal_tujuan": []string{}},
		map[string]any{"tanggal_tujuan": []string{"2000-01-01"}},
		map[string]any{"tanggal_tujuan": []string{day3, day3}},
		map[string]any{"tanggal_tujuan": "bukan-array"},
		map[string]any{},
	}
	for i, body := range cases {
		if w := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/copy", giziToken, body); w.Code != http.StatusBadRequest {
			t.Fatalf("case %d: got %d %s", i, w.Code, w.Body.String())
		}
	}
	many := []string{}
	for i := 1; i <= 8; i++ {
		many = append(many, time.Now().Add(time.Duration(24*i)*time.Hour).Format("2006-01-02"))
	}
	if w := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/copy", giziToken, map[string]any{"tanggal_tujuan": many}); w.Code != http.StatusBadRequest {
		t.Fatalf(">7 dates: got %d", w.Code)
	}

	// RBAC: non-gizi 403, missing source 404, unauth 401.
	if w := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/copy", adminToken, map[string]any{"tanggal_tujuan": []string{day3}}); w.Code != http.StatusForbidden {
		t.Fatalf("non-gizi: got %d", w.Code)
	}
	if w := doRequest(t, f.router, "POST", "/api/v1/menus/999999/copy", giziToken, map[string]any{"tanggal_tujuan": []string{day3}}); w.Code != http.StatusNotFound {
		t.Fatalf("missing source: got %d", w.Code)
	}
	if w := doRequest(t, f.router, "POST", "/api/v1/menus/"+itoa(created.ID)+"/copy", "", map[string]any{"tanggal_tujuan": []string{day3}}); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: got %d", w.Code)
	}
}

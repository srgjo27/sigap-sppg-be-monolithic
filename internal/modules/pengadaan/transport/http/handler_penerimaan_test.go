package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	authapplication "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/application"
	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	authinfra "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/infrastructure"
	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

type penerimaanFakePORepo struct{}

func (f *penerimaanFakePORepo) ListBySPPGAndStatus(ctx context.Context, sppgID int64, statuses []string) ([]domain.PurchaseOrder, error) {
	return nil, nil
}
func (f *penerimaanFakePORepo) ListItemsByPOIDs(ctx context.Context, poIDs []int64) (map[int64][]domain.POItem, error) {
	return map[int64][]domain.POItem{1: {{ID: 1, POID: 1, BahanID: 1, QtyPesan: 100, HargaSatuan: 10000}}}, nil
}
func (f *penerimaanFakePORepo) ListPOMenus(ctx context.Context, menuIDs []int64) (map[int64][]int64, error) {
	return nil, nil
}
func (f *penerimaanFakePORepo) Create(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, menuIDs []int64) (*domain.PurchaseOrder, []domain.POItem, error) {
	return nil, nil, nil
}
func (f *penerimaanFakePORepo) FindByID(ctx context.Context, id int64) (*domain.PurchaseOrder, error) {
	return &domain.PurchaseOrder{ID: id, SPPGID: 1, Status: domain.POStatusDikirim}, nil
}
func (f *penerimaanFakePORepo) List(ctx context.Context, sppgID int64, filter domain.POFilter) ([]domain.POListItem, int, error) {
	return nil, 0, nil
}
func (f *penerimaanFakePORepo) GetDetail(ctx context.Context, id int64) (*domain.PODetail, error) {
	return nil, nil
}
func (f *penerimaanFakePORepo) GetPOItemRekap(ctx context.Context, poItemIDs []int64) (map[int64]domain.POItemRekap, error) {
	out := make(map[int64]domain.POItemRekap)
	for _, id := range poItemIDs {
		out[id] = domain.POItemRekap{POItemID: id, QtyDiterima: 0}
	}
	return out, nil
}
func (f *penerimaanFakePORepo) Update(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, replaceItems bool, menuIDs []int64, replaceMenus bool) (*domain.PurchaseOrder, []domain.POItem, error) {
	return nil, nil, nil
}
func (f *penerimaanFakePORepo) Delete(ctx context.Context, id int64) error { return nil }
func (f *penerimaanFakePORepo) ListMenuIDsForPO(ctx context.Context, poID int64) ([]int64, error) {
	return nil, nil
}
func (f *penerimaanFakePORepo) HasPenerimaan(ctx context.Context, poID int64) (bool, error) {
	return false, nil
}
func (f *penerimaanFakePORepo) SetDikirim(ctx context.Context, id int64, userID int64, at time.Time) (*domain.PurchaseOrder, error) {
	return nil, nil
}
func (f *penerimaanFakePORepo) SetStatus(ctx context.Context, id int64, fromStatus, toStatus string, userID int64, alasan *string, at time.Time) (*domain.PurchaseOrder, error) {
	return nil, nil
}
func (f *penerimaanFakePORepo) ReserveNumberSeq(ctx context.Context, sppgID int64, tahunBulan string) (int, error) {
	return 1, nil
}

type penerimaanFakeBahanRepo struct{}

func (f *penerimaanFakeBahanRepo) FindByID(ctx context.Context, id int64) (*domain.Bahan, error) {
	return &domain.Bahan{ID: id, Nama: "Bahan Test", Satuan: "kg", Aktif: true}, nil
}
func (f *penerimaanFakeBahanRepo) List(ctx context.Context, filter domain.BahanFilter) ([]domain.Bahan, int, error) {
	return nil, 0, nil
}

type penerimaanFakeSPPG struct{}

func (f *penerimaanFakeSPPG) GetKode(ctx context.Context, id int64) (string, error) {
	return "TST", nil
}

type penerimaanFakePenerimaanRepo struct {
	createErr error
}

func (f *penerimaanFakePenerimaanRepo) CreatePenerimaan(ctx context.Context, poID int64, in domain.CreatePenerimaanInput, diperiksaOleh int64, at time.Time) ([]*domain.PenerimaanBahan, *domain.PurchaseOrder, error) {
	if f.createErr != nil {
		return nil, nil, f.createErr
	}
	return []*domain.PenerimaanBahan{{
		ID:              1,
		POItemID:        in.Items[0].POItemID,
		KodeLot:         "LOT-TST-20261027-001",
		WaktuTerima:     at,
		QtyDiterima:     in.Items[0].QtyDiterima,
		QtyDitolak:      in.Items[0].QtyDitolak,
		Kondisi:         in.Items[0].Kondisi,
		DiperiksaOleh:   diperiksaOleh,
		NilaiPotongNota: in.Items[0].NilaiPotongNota,
		SPPGID:          1,
		ClientRef:       in.ClientRef,
		CreatedAt:       at,
	}}, &domain.PurchaseOrder{ID: poID, Status: domain.POStatusDiterimaSebagian}, nil
}

func (f *penerimaanFakePenerimaanRepo) List(ctx context.Context, sppgID int64, filter domain.PenerimaanFilter) ([]domain.PenerimaanListItem, int, error) {
	return []domain.PenerimaanListItem{
		{
			ID:          1,
			KodeLot:     "LOT-TST-20261027-001",
			NomorPO:     "PO/TST/202610/0001",
			PemasokNama: "PT Test",
			BahanNama:   "Bahan Test",
			WaktuTerima: time.Now(),
			QtyDiterima: 10,
			Kondisi:     domain.KondisiBaik,
		},
	}, 1, nil
}

func (f *penerimaanFakePenerimaanRepo) GetByID(ctx context.Context, id int64) (*domain.PenerimaanDetail, error) {
	return &domain.PenerimaanDetail{
		ID:            id,
		POID:          1,
		POItemID:      1,
		NomorPO:       "PO/TST/202610/0001",
		BahanNama:     "Bahan Test",
		KodeLot:       "LOT-TST-20261027-001",
		WaktuTerima:   time.Now(),
		QtyDiterima:   10,
		Kondisi:       domain.KondisiBaik,
		SPPGID:        1,
		DiperiksaOleh: 99,
		POStatus:      domain.POStatusDiterimaSebagian,
	}, nil
}

func (f *penerimaanFakePenerimaanRepo) KoreksiPenerimaan(ctx context.Context, id int64, in domain.KoreksiPenerimaanInput, disetujuiOleh int64, at time.Time) (*domain.PenerimaanDetail, *domain.PurchaseOrder, error) {
	return &domain.PenerimaanDetail{
		ID:            id,
		POID:          1,
		POItemID:      1,
		NomorPO:       "PO/TST/202610/0001",
		BahanNama:     "Bahan Test",
		KodeLot:       "LOT-TST-20261027-001",
		WaktuTerima:   time.Now(),
		QtyDiterima:   *in.QtyDiterima,
		QtyDitolak:    *in.QtyDitolak,
		Kondisi:       *in.Kondisi,
		SPPGID:        1,
		DiperiksaOleh: 99,
		POStatus:      domain.POStatusDiterimaSebagian,
		DikoreksiOleh: &disetujuiOleh,
		DikoreksiAt:   &at,
	}, &domain.PurchaseOrder{ID: 1, Status: domain.POStatusDiterimaSebagian}, nil
}

type penerimaanFixture struct {
	router  *gin.Engine
	auth    *authinfra.MemoryStores
	svc     *application.Service
	authSvc *authapplication.Service
}

func newPenerimaanFixture() *penerimaanFixture {
	gin.SetMode(gin.TestMode)
	authStore := authinfra.NewMemoryStores()
	authStore.SeedSPPGWithName(1, "SPPG Test")
	issuer := authinfra.NewJWTIssuer("test-secret")
	authSvc := authapplication.New(authapplication.Deps{
		Users: authStore, Refresh: authStore, Audits: authStore, SPPG: authStore, Sekolah: authStore,
		Tokens: issuer, Hasher: authinfra.NewBcryptHasher(4),
	})
	mw := authhttp.NewMiddleware(issuer, authSvc)

	svc := application.New(application.Deps{
		PO:         &penerimaanFakePORepo{},
		Bahan:      &penerimaanFakeBahanRepo{},
		SPPG:       &penerimaanFakeSPPG{},
		Penerimaan: &penerimaanFakePenerimaanRepo{},
		Audits:     &penerimaanTrackingAuditRepo{},
		Notifs:     &penerimaanTrackingNotifRepo{},
		Clock:      func() time.Time { return time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC) },
	})

	r := gin.New()
	v1 := r.Group("/api/v1")
	RegisterRoutes(v1, NewHandler(svc), mw)
	return &penerimaanFixture{router: r, auth: authStore, svc: svc, authSvc: authSvc}
}

var seedCounter int

func (f *penerimaanFixture) seedToken(t *testing.T, role authdomain.Role) string {
	t.Helper()
	seedCounter++
	email := fmt.Sprintf("uji%d%s@test.local", seedCounter, role)
	created, err := f.authSvc.CreateUser(context.Background(), &authdomain.Claims{UserID: int64(seedCounter), Role: authdomain.RoleAdmin}, authdomain.CreateUserInput{
		Nama: string(role), Email: email, Peran: role, SPPGID: int64p2(1), PasswordAwal: strPtrLocal("pass1234"),
	}, nil)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	u, _ := f.auth.FindByID(context.Background(), created.User.ID)
	u.WajibGantiPassword = false
	if _, err := f.auth.Update(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	login, err := f.authSvc.Login(context.Background(), email, "pass1234", nil, nil)
	if err != nil {
		t.Fatalf("seed login: %v", err)
	}
	return login.AccessToken
}

func doPenerimaanRequest(t *testing.T, r *gin.Engine, method, path, token string, idemKey string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPenerimaan_HandlerContracts(t *testing.T) {
	f := newPenerimaanFixture()
	akuntan := f.seedToken(t, authdomain.RoleAkuntan)
	petugas := f.seedToken(t, authdomain.RolePetugasDapur)
	bukanAkuntan := f.seedToken(t, authdomain.RoleAhliGizi)

	itemsBody := map[string]any{
		"items": []map[string]any{
			{"po_item_id": 1, "qty_diterima": 10, "qty_ditolak": 0, "kondisi": "baik", "foto_berkas_id": 100},
		},
	}

	t.Run("POST /penerimaan by akuntan -> 201", func(t *testing.T) {
		res := doPenerimaanRequest(t, f.router, "POST", "/api/v1/purchase-orders/1/penerimaan", akuntan, "550e8400-e29b-41d4-a716-446655440000", itemsBody)
		if res.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d %s", res.Code, res.Body.String())
		}
	})

	t.Run("POST /penerimaan by non-akuntan/dapur -> 403", func(t *testing.T) {
		res := doPenerimaanRequest(t, f.router, "POST", "/api/v1/purchase-orders/1/penerimaan", bukanAkuntan, "key-1", itemsBody)
		if res.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d %s", res.Code, res.Body.String())
		}
	})

	t.Run("POST /penerimaan without idempotency key -> 400", func(t *testing.T) {
		res := doPenerimaanRequest(t, f.router, "POST", "/api/v1/purchase-orders/1/penerimaan", akuntan, "", itemsBody)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d %s", res.Code, res.Body.String())
		}
	})

	t.Run("POST /penerimaan without token -> 401", func(t *testing.T) {
		res := doPenerimaanRequest(t, f.router, "POST", "/api/v1/purchase-orders/1/penerimaan", "", "key-1", itemsBody)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d %s", res.Code, res.Body.String())
		}
	})

	t.Run("GET /penerimaan by petugas_dapur -> 200", func(t *testing.T) {
		res := doPenerimaanRequest(t, f.router, "GET", "/api/v1/penerimaan", petugas, "", nil)
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d %s", res.Code, res.Body.String())
		}
	})

	t.Run("GET /penerimaan/:id -> 200", func(t *testing.T) {
		res := doPenerimaanRequest(t, f.router, "GET", "/api/v1/penerimaan/1", akuntan, "", nil)
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d %s", res.Code, res.Body.String())
		}
	})

	t.Run("POST /penerimaan/:id/koreksi by kepala_sppg -> 200", func(t *testing.T) {
		kepala := f.seedToken(t, authdomain.RoleKepalaSPPG)
		res := doPenerimaanRequest(t, f.router, "POST", "/api/v1/penerimaan/1/koreksi", kepala, "", map[string]any{
			"alasan":       "perbaikan qty karena salah ketik",
			"qty_diterima": 15,
			"qty_ditolak":  0,
			"kondisi":      "baik",
			"catatan":      "perbaikan catatan",
		})
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d %s", res.Code, res.Body.String())
		}
	})

	t.Run("POST /penerimaan/:id/koreksi by non-kepala -> 403", func(t *testing.T) {
		res := doPenerimaanRequest(t, f.router, "POST", "/api/v1/penerimaan/1/koreksi", akuntan, "", map[string]any{
			"alasan":       "perbaikan qty karena salah ketik",
			"qty_diterima": 15,
			"kondisi":      "baik",
		})
		if res.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d %s", res.Code, res.Body.String())
		}
	})

	t.Run("POST /penerimaan/:id/koreksi without alasan -> 400", func(t *testing.T) {
		kepala := f.seedToken(t, authdomain.RoleKepalaSPPG)
		res := doPenerimaanRequest(t, f.router, "POST", "/api/v1/penerimaan/1/koreksi", kepala, "", map[string]any{
			"qty_diterima": 15,
			"kondisi":      "baik",
		})
		if res.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d %s", res.Code, res.Body.String())
		}
	})
}

func strPtrLocal(s string) *string { return &s }

type penerimaanTrackingAuditRepo struct{}

func (f *penerimaanTrackingAuditRepo) Append(ctx context.Context, e *authdomain.AuditEntry) error {
	return nil
}

type penerimaanTrackingNotifRepo struct{}

func (f *penerimaanTrackingNotifRepo) NotifySPPGRoles(ctx context.Context, sppgID int64, roles []string, jenis, judul, isi, refTabel string, refID int64) error {
	return nil
}

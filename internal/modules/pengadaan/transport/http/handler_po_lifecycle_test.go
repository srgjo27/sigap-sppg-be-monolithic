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

// ===== Minimal in-memory fakes for the PO lifecycle endpoints =====

type fakePORepo struct {
	pos       map[int64]*domain.PurchaseOrder
	items     map[int64][]domain.POItem
	menuLinks map[int64][]int64
	receipts  map[int64]bool
	updateErr error
	nextID    int64
}

func newFakePORepo() *fakePORepo {
	return &fakePORepo{
		pos:       make(map[int64]*domain.PurchaseOrder),
		items:     make(map[int64][]domain.POItem),
		menuLinks: make(map[int64][]int64),
		receipts:  make(map[int64]bool),
	}
}

func (f *fakePORepo) ListBySPPGAndStatus(ctx context.Context, sppgID int64, statuses []string) ([]domain.PurchaseOrder, error) {
	return nil, nil
}

func (f *fakePORepo) ListItemsByPOIDs(ctx context.Context, poIDs []int64) (map[int64][]domain.POItem, error) {
	result := make(map[int64][]domain.POItem, len(poIDs))
	for _, id := range poIDs {
		result[id] = f.items[id]
	}
	return result, nil
}

func (f *fakePORepo) ListPOMenus(ctx context.Context, menuIDs []int64) (map[int64][]int64, error) {
	return map[int64][]int64{}, nil
}

func (f *fakePORepo) Create(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, menuIDs []int64) (*domain.PurchaseOrder, []domain.POItem, error) {
	po.ID = int64(len(f.pos) + 1)
	f.pos[po.ID] = po
	f.items[po.ID] = items
	return po, items, nil
}

func (f *fakePORepo) FindByID(ctx context.Context, id int64) (*domain.PurchaseOrder, error) {
	return f.pos[id], nil
}

func (f *fakePORepo) List(ctx context.Context, sppgID int64, filter domain.POFilter) ([]domain.POListItem, int, error) {
	return nil, 0, nil
}

func (f *fakePORepo) GetDetail(ctx context.Context, id int64) (*domain.PODetail, error) {
	po := f.pos[id]
	if po == nil {
		return nil, nil
	}
	detail := &domain.PODetail{PurchaseOrder: *po, PemasokNama: "PT Segar"}
	for _, item := range f.items[id] {
		detail.Items = append(detail.Items, domain.PODetailItem{
			ID: item.ID, BahanID: item.BahanID, BahanNama: "Telur", BahanSatuan: "butir",
			QtyPesan: item.QtyPesan, HargaSatuan: item.HargaSatuan,
		})
	}
	return detail, nil
}

func (f *fakePORepo) GetPOItemRekap(ctx context.Context, poItemIDs []int64) (map[int64]domain.POItemRekap, error) {
	return map[int64]domain.POItemRekap{}, nil
}

// ReserveNumberSeq returns a deterministic sequence (fake implementation).
func (f *fakePORepo) ReserveNumberSeq(ctx context.Context, sppgID int64, tahunBulan string) (int, error) {
	return 1, nil
}

func (f *fakePORepo) Update(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, replaceItems bool, menuIDs []int64, replaceMenus bool) (*domain.PurchaseOrder, []domain.POItem, error) {
	if f.updateErr != nil {
		return nil, nil, f.updateErr
	}
	current := f.pos[po.ID]
	if current == nil || current.Status != domain.POStatusDraf {
		return nil, nil, domain.ErrPOInvalidStatus
	}
	current.PemasokID = po.PemasokID
	current.SiklusID = po.SiklusID
	current.TglKirimDiminta = po.TglKirimDiminta
	current.Catatan = po.Catatan
	var updated []domain.POItem
	if replaceItems {
		replaced := make([]domain.POItem, len(items))
		copy(replaced, items)
		f.items[po.ID] = replaced
		updated = replaced
	}
	if replaceMenus {
		links := make([]int64, len(menuIDs))
		copy(links, menuIDs)
		f.menuLinks[po.ID] = links
	}
	return current, updated, nil
}

func (f *fakePORepo) Delete(ctx context.Context, id int64) error {
	po := f.pos[id]
	if po == nil || po.Status != domain.POStatusDraf {
		return domain.ErrPOInvalidStatus
	}
	delete(f.pos, id)
	return nil
}

func (f *fakePORepo) ListMenuIDsForPO(ctx context.Context, poID int64) ([]int64, error) {
	return f.menuLinks[poID], nil
}

func (f *fakePORepo) HasPenerimaan(ctx context.Context, poID int64) (bool, error) {
	return f.receipts[poID], nil
}

func (f *fakePORepo) SetDikirim(ctx context.Context, id int64, userID int64, at time.Time) (*domain.PurchaseOrder, error) {
	po := f.pos[id]
	if po == nil || po.Status != domain.POStatusDraf {
		return nil, domain.ErrPOInvalidStatus
	}
	po.Status = domain.POStatusDikirim
	po.DikirimOleh = &userID
	po.DikirimAt = &at
	return po, nil
}

func (f *fakePORepo) SetStatus(ctx context.Context, id int64, fromStatus, toStatus string, userID int64, alasan *string, at time.Time) (*domain.PurchaseOrder, error) {
	po := f.pos[id]
	if po == nil || po.Status != fromStatus {
		return nil, domain.ErrPOInvalidStatus
	}
	po.Status = toStatus
	po.DitutupOleh = &userID
	po.DitutupAt = &at
	po.AlasanTutup = alasan
	return po, nil
}

type fakePemasokRepo struct{ pemasoks map[int64]*domain.Pemasok }

func (f *fakePemasokRepo) Create(ctx context.Context, p *domain.Pemasok) (*domain.Pemasok, error) {
	return p, nil
}
func (f *fakePemasokRepo) FindByID(ctx context.Context, id int64) (*domain.Pemasok, error) {
	return f.pemasoks[id], nil
}
func (f *fakePemasokRepo) FindByNameInsensitive(ctx context.Context, sppgID int64, nama string) (*domain.Pemasok, error) {
	return nil, nil
}
func (f *fakePemasokRepo) Update(ctx context.Context, p *domain.Pemasok) (*domain.Pemasok, error) {
	return p, nil
}
func (f *fakePemasokRepo) List(ctx context.Context, sppgID int64, filter domain.PemasokFilter) ([]domain.Pemasok, int, error) {
	return nil, 0, nil
}
func (f *fakePemasokRepo) GetKinerja(ctx context.Context, sppgID, pemasokID int64) (*domain.Kinerja, error) {
	return &domain.Kinerja{PeriodeHari: 90}, nil
}

type fakeBahanRepo struct{ bahans map[int64]*domain.Bahan }

func (f *fakeBahanRepo) FindByID(ctx context.Context, id int64) (*domain.Bahan, error) {
	return f.bahans[id], nil
}
func (f *fakeBahanRepo) List(ctx context.Context, filter domain.BahanFilter) ([]domain.Bahan, int, error) {
	return nil, 0, nil
}

type fakeMenuRepo struct{}

func (f *fakeMenuRepo) ListByScope(ctx context.Context, sppgID *int64, dari, sampai time.Time, status *string) ([]domain.Menu, error) {
	return nil, nil
}
func (f *fakeMenuRepo) ListItems(ctx context.Context, menuIDs []int64) (map[int64][]domain.MenuBahan, error) {
	return map[int64][]domain.MenuBahan{}, nil
}
func (f *fakeMenuRepo) FindByID(ctx context.Context, id int64) (*domain.Menu, error) { return nil, nil }
func (f *fakeMenuRepo) FindByIDs(ctx context.Context, ids []int64) ([]*domain.Menu, error) {
	return []*domain.Menu{}, nil
}

type fakeSiklusRepo struct{}

func (f *fakeSiklusRepo) FindByID(ctx context.Context, id int64) (*domain.SiklusDana, error) {
	return nil, nil
}

type fakeSPPGProvider struct{}

func (f *fakeSPPGProvider) GetKode(ctx context.Context, id int64) (string, error) {
	return "SPPG01", nil
}

type fakeAuditRepo struct{ count int }

func (f *fakeAuditRepo) Append(ctx context.Context, e *authdomain.AuditEntry) error {
	f.count++
	return nil
}

type fakeNotifRepo struct{ jenis []string }

func (f *fakeNotifRepo) NotifySPPGRoles(ctx context.Context, sppgID int64, roles []string, jenis, judul, isi, refTabel string, refID int64) error {
	f.jenis = append(f.jenis, jenis)
	return nil
}

// ===== Fixture =====

type poFixture struct {
	router  *gin.Engine
	auth    *authinfra.MemoryStores
	po      *fakePORepo
	pemasok *fakePemasokRepo
	audits  *fakeAuditRepo
	notifs  *fakeNotifRepo
	authSvc *authapplication.Service
}

func newPOFixture() *poFixture {
	gin.SetMode(gin.TestMode)
	authStore := authinfra.NewMemoryStores()
	authStore.SeedSPPGWithName(1, "SPPG Cempaka")
	authStore.SeedSPPGWithName(2, "SPPG Nusantara")
	issuer := authinfra.NewJWTIssuer("test-secret")
	authSvc := authapplication.New(authapplication.Deps{
		Users: authStore, Refresh: authStore, Audits: authStore, SPPG: authStore, Sekolah: authStore,
		Tokens: issuer, Hasher: authinfra.NewBcryptHasher(4),
	})
	mw := authhttp.NewMiddleware(issuer, authSvc)

	poRepo := newFakePORepo()
	pemasokRepo := &fakePemasokRepo{pemasoks: map[int64]*domain.Pemasok{
		10: {ID: 10, SPPGID: 1, Nama: "PT Segar", Aktif: true},
		11: {ID: 11, SPPGID: 1, Nama: "PT Nonaktif", Aktif: false},
	}}
	auditRepo := &fakeAuditRepo{}
	notifRepo := &fakeNotifRepo{}
	svc := application.New(application.Deps{
		Pemasok: pemasokRepo,
		Bahan: &fakeBahanRepo{bahans: map[int64]*domain.Bahan{
			100: {ID: 100, Nama: "Telur", Satuan: "butir", Aktif: true},
		}},
		Menus:  &fakeMenuRepo{},
		Siklus: &fakeSiklusRepo{},
		PO:     poRepo,
		SPPG:   &fakeSPPGProvider{},
		Audits: auditRepo,
		Notifs: notifRepo,
		Clock:  func() time.Time { return time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC) },
	})

	r := gin.New()
	v1 := r.Group("/api/v1")
	RegisterRoutes(v1, NewHandler(svc), mw)
	return &poFixture{router: r, auth: authStore, po: poRepo, pemasok: pemasokRepo, audits: auditRepo, notifs: notifRepo, authSvc: authSvc}
}

func (f *poFixture) seedPO(t *testing.T, diminta time.Time, status string) *domain.PurchaseOrder {
	t.Helper()
	po := &domain.PurchaseOrder{
		SPPGID:          1,
		NomorPO:         "PO/SPPGC01/202610/0001",
		PemasokID:       10,
		TglPO:           f.poNextDay(),
		TglKirimDiminta: &diminta,
		Status:          status,
		DibuatOleh:      7,
	}
	po.ID = int64(len(f.po.pos) + 1)
	f.po.pos[po.ID] = po
	f.po.items[po.ID] = []domain.POItem{{ID: 1, POID: po.ID, BahanID: 100, QtyPesan: 10, HargaSatuan: 2000}}
	return po
}

func (f *poFixture) poNextDay() time.Time { return time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC) }

func seedUserToken(t *testing.T, f *poFixture, email, password string, role authdomain.Role, sppg *int64) string {
	t.Helper()
	created, err := f.authSvc.CreateUser(context.Background(), &authdomain.Claims{UserID: 999, Role: authdomain.RoleAdmin}, authdomain.CreateUserInput{
		Nama: string(role), Email: email, Peran: role, SPPGID: sppg, PasswordAwal: &password,
	}, nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	u, _ := f.auth.FindByID(context.Background(), created.User.ID)
	u.WajibGantiPassword = false
	if _, err := f.auth.Update(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	login, err := f.authSvc.Login(context.Background(), email, password, nil, nil)
	if err != nil {
		t.Fatalf("seed login: %v", err)
	}
	return login.AccessToken
}

func doPORequest(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req = httptest.NewRequest(method, path, bytes.NewReader(buf))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ===== HTTP contract tests =====

func TestPOLifecycle_TransportContracts(t *testing.T) {
	f := newPOFixture()
	akuntan := seedUserToken(t, f, "akun@x.com", "akun1234", authdomain.RoleAkuntan, int64p2(1))
	kepala := seedUserToken(t, f, "kepala@x.com", "kepala1234", authdomain.RoleKepalaSPPG, int64p2(1))
	nobody := seedUserToken(t, f, "petugas@x.com", "dapur1234", authdomain.RolePetugasDapur, int64p2(1))

	// PATCH by akuntan on draft: only catatan changes.
	draft := f.seedPO(t, f.poNextDay(), domain.POStatusDraf)
	res := doPORequest(t, f.router, "PATCH", poPath(draft.ID), akuntan, map[string]any{"catatan": "tambahan telur"})
	if res.Code != http.StatusOK {
		t.Fatalf("patch draft: got %d %s", res.Code, res.Body.String())
	}
	var patched PODetailResponse
	if err := json.Unmarshal(res.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Catatan != "tambahan telur" {
		t.Fatalf("catatan not applied: %+v", patched)
	}

	// PATCH wrong status -> 409 STATUS_TIDAK_SESUAI.
	draft.Status = domain.POStatusDikirim
	res = doPORequest(t, f.router, "PATCH", poPath(draft.ID), akuntan, map[string]any{"catatan": "x"})
	if res.Code != http.StatusConflict {
		t.Fatalf("patch dikirim: got %d %s", res.Code, res.Body.String())
	}

	// PATCH by petugas_dapur -> 403.
	draft.Status = domain.POStatusDraf
	res = doPORequest(t, f.router, "PATCH", poPath(draft.ID), nobody, map[string]any{"catatan": "x"})
	if res.Code != http.StatusForbidden {
		t.Fatalf("patch dapur: got %d %s", res.Code, res.Body.String())
	}

	// DELETE draft -> 204; DELETE sent -> 409.
	res = doPORequest(t, f.router, "DELETE", poPath(draft.ID), akuntan, nil)
	if res.Code != http.StatusNoContent {
		t.Fatalf("delete draft: got %d %s", res.Code, res.Body.String())
	}
	sent := f.seedPO(t, f.poNextDay(), domain.POStatusDikirim)
	res = doPORequest(t, f.router, "DELETE", poPath(sent.ID), akuntan, nil)
	if res.Code != http.StatusConflict {
		t.Fatalf("delete dikirim: got %d %s", res.Code, res.Body.String())
	}

	// kirim -> 200, po_dikirim notification to petugas_dapur + kepala_sppg.
	kirimPO := f.seedPO(t, f.poNextDay(), domain.POStatusDraf)
	res = doPORequest(t, f.router, "POST", poPath(kirimPO.ID)+"/kirim", akuntan, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("kirim: got %d %s", res.Code, res.Body.String())
	}
	if len(f.notifs.jenis) == 0 || f.notifs.jenis[len(f.notifs.jenis)-1] != "po_dikirim" {
		t.Fatalf("expected po_dikirim notification, got %v", f.notifs.jenis)
	}

	// batal by kepala_sppg -> 200 (route must allow kepala_sppg).
	batalPO := f.seedPO(t, f.poNextDay(), domain.POStatusDraf)
	f.po.receipts[batalPO.ID] = false
	res = doPORequest(t, f.router, "POST", poPath(batalPO.ID)+"/batal", kepala, map[string]any{"alasan": "pemasok tidak sanggup"})
	if res.Code != http.StatusOK {
		t.Fatalf("batal kepala: got %d %s", res.Code, res.Body.String())
	}

	// batal without alasan -> 400.
	batal2 := f.seedPO(t, f.poNextDay(), domain.POStatusDraf)
	res = doPORequest(t, f.router, "POST", poPath(batal2.ID)+"/batal", akuntan, nil)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("batal no alasan: got %d %s", res.Code, res.Body.String())
	}

	// batal with receipt -> 409 PO_SUDAH_ADA_PENERIMAAN.
	batal3 := f.seedPO(t, f.poNextDay(), domain.POStatusDikirim)
	f.po.receipts[batal3.ID] = true
	res = doPORequest(t, f.router, "POST", poPath(batal3.ID)+"/batal", akuntan, map[string]any{"alasan": "sudah ada kiriman"})
	if res.Code != http.StatusConflict {
		t.Fatalf("batal with receipt: got %d %s", res.Code, res.Body.String())
	}
	var batalErr ErrorEnvelope
	_ = json.Unmarshal(res.Body.Bytes(), &batalErr)
	if batalErr.Error.Code != "PO_SUDAH_ADA_PENERIMAAN" {
		t.Fatalf("expected PO_SUDAH_ADA_PENERIMAAN, got %s", batalErr.Error.Code)
	}

	// batal by petugas_dapur -> 403.
	batal4 := f.seedPO(t, f.poNextDay(), domain.POStatusDraf)
	res = doPORequest(t, f.router, "POST", poPath(batal4.ID)+"/batal", nobody, map[string]any{"alasan": "coba batal ya"})
	if res.Code != http.StatusForbidden {
		t.Fatalf("batal dapur: got %d %s", res.Code, res.Body.String())
	}

	// tutup by akuntan on diterima_sebagian -> 200 with item_tidak_terpenuhi.
	tutup := f.seedPO(t, f.poNextDay(), domain.POStatusDiterimaSebagian)
	res = doPORequest(t, f.router, "POST", poPath(tutup.ID)+"/tutup", akuntan, map[string]any{"alasan": "sisa tidak dikirim"})
	if res.Code != http.StatusOK {
		t.Fatalf("tutup: got %d %s", res.Code, res.Body.String())
	}
	var closed POTransitionResponse
	if err := json.Unmarshal(res.Body.Bytes(), &closed); err != nil {
		t.Fatal(err)
	}
	if closed.Status != domain.POStatusSelesai {
		t.Fatalf("expected selesai, got %s", closed.Status)
	}
	if closed.ItemTidakTerpenuhi == nil || len(closed.ItemTidakTerpenuhi) != 1 {
		t.Fatalf("expected 1 unfulfilled item, got %+v", closed.ItemTidakTerpenuhi)
	}

	// tutup on dikirim -> 409.
	tutup2 := f.seedPO(t, f.poNextDay(), domain.POStatusDikirim)
	res = doPORequest(t, f.router, "POST", poPath(tutup2.ID)+"/tutup", akuntan, map[string]any{"alasan": "belum diterima"})
	if res.Code != http.StatusConflict {
		t.Fatalf("tutup dikirim: got %d %s", res.Code, res.Body.String())
	}

	// kepala_sppg cannot tutup -> 403.
	tutup3 := f.seedPO(t, f.poNextDay(), domain.POStatusDiterimaSebagian)
	res = doPORequest(t, f.router, "POST", poPath(tutup3.ID)+"/tutup", kepala, map[string]any{"alasan": "tutup ya"})
	if res.Code != http.StatusForbidden {
		t.Fatalf("tutup kepala: got %d %s", res.Code, res.Body.String())
	}
}

func TestPOLifecycle_Transport_RBACAndOwnership(t *testing.T) {
	f := newPOFixture()
	akuntanSPPG2 := seedUserToken(t, f, "akun2@x.com", "akun2234", authdomain.RoleAkuntan, int64p2(2))

	// PO of SPPG 1 is hidden from akuntan of SPPG 2 across all lifecycle verbs.
	po := f.seedPO(t, f.poNextDay(), domain.POStatusDraf)
	for _, req := range []struct {
		method, path string
		body         any
	}{
		{"PATCH", poPath(po.ID), map[string]any{"catatan": "x"}},
		{"DELETE", poPath(po.ID), nil},
		{"POST", poPath(po.ID) + "/kirim", nil},
		{"POST", poPath(po.ID) + "/batal", map[string]any{"alasan": "coba batal ya"}},
		{"POST", poPath(po.ID) + "/tutup", map[string]any{"alasan": "tutup ya"}},
	} {
		res := doPORequest(t, f.router, req.method, req.path, akuntanSPPG2, req.body)
		if res.Code != http.StatusNotFound {
			t.Fatalf("%s %s: expected 404, got %d %s", req.method, req.path, res.Code, res.Body.String())
		}
	}
	if f.audits.count != 0 {
		t.Fatalf("no audit expected for foreign PO, got %d", f.audits.count)
	}
}

func poPath(id int64) string {
	return "/api/v1/purchase-orders/" + itoa(id)
}

// TestPOLifecycle_Transport_WrappedErrorsAndNullPemasokID verifies two
// MapError/handler contracts: wrapped domain sentinels still map to their
// intended HTTP status (errors.Is), and PATCH with pemasok_id null is a
// client error instead of a silently ignored field.
func TestPOLifecycle_Transport_WrappedErrorsAndNullPemasokID(t *testing.T) {
	f := newPOFixture()
	akuntan := seedUserToken(t, f, "akun3@x.com", "akun3234", authdomain.RoleAkuntan, int64p2(1))

	// A repo error that wraps a sentinel (fmt.Errorf %w) must map by identity,
	// not fall through to 500.
	po := f.seedPO(t, f.poNextDay(), domain.POStatusDraf)
	f.po.updateErr = fmt.Errorf("update purchase_order: %w", domain.ErrPOInvalidStatus)
	res := doPORequest(t, f.router, "PATCH", poPath(po.ID), akuntan, map[string]any{"catatan": "x"})
	if res.Code != http.StatusConflict {
		t.Fatalf("wrapped sentinel: expected 409, got %d %s", res.Code, res.Body.String())
	}
	var errBody ErrorEnvelope
	if err := json.Unmarshal(res.Body.Bytes(), &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Error.Code != "STATUS_TIDAK_SESUAI" {
		t.Fatalf("expected STATUS_TIDAK_SESUAI, got %s", errBody.Error.Code)
	}
	f.po.updateErr = nil

	// PATCH pemasok_id: null must be rejected rather than ignored.
	res = doPORequest(t, f.router, "PATCH", poPath(po.ID), akuntan, map[string]any{"pemasok_id": nil})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("pemasok_id null: expected 400, got %d %s", res.Code, res.Body.String())
	}
}

func itoa(id int64) string {
	b, _ := json.Marshal(id)
	return string(b)
}

func int64p2(v int64) *int64 { return &v }

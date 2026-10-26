package application

import (
	"context"
	"errors"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

var _ PurchaseOrderRepository = (*lifecyclePORepo)(nil)

type lifecyclePORepo struct {
	*mockPORepo
	pos           map[int64]*domain.PurchaseOrder
	items         map[int64][]domain.POItem // poID -> items
	menuLinks     map[int64][]int64         // poID -> menu_ids
	receipts      map[int64]bool            // poID -> has penerimaan
	terpenuhi     map[int64]map[int64]bool  // poID -> poItemID -> terpenuhi
	deleted       map[int64]bool
	nextID        int64
	notifications []string
	auditActions  []string
	reservations  []string         // reserved "tahun_bulan" values, in order
	numberSeqs    map[string][]int // tahun_bulan -> issued sequences
}

func newLifecyclePORepo() *lifecyclePORepo {
	return &lifecyclePORepo{
		mockPORepo: newMockPORepo(),
		pos:        make(map[int64]*domain.PurchaseOrder),
		items:      make(map[int64][]domain.POItem),
		menuLinks:  make(map[int64][]int64),
		receipts:   make(map[int64]bool),
		terpenuhi:  make(map[int64]map[int64]bool),
		deleted:    make(map[int64]bool),
		numberSeqs: make(map[string][]int),
	}
}

func (m *lifecyclePORepo) seed(po *domain.PurchaseOrder, items []domain.POItem) *domain.PurchaseOrder {
	m.nextID++
	po.ID = m.nextID
	m.pos[po.ID] = po
	m.items[po.ID] = items
	return po
}

func (m *lifecyclePORepo) FindByID(ctx context.Context, id int64) (*domain.PurchaseOrder, error) {
	if m.deleted[id] {
		return nil, nil
	}
	return m.pos[id], nil
}

func (m *lifecyclePORepo) ListItemsByPOIDs(ctx context.Context, poIDs []int64) (map[int64][]domain.POItem, error) {
	result := make(map[int64][]domain.POItem)
	for _, id := range poIDs {
		result[id] = m.items[id]
	}
	return result, nil
}

func (m *lifecyclePORepo) ListMenuIDsForPO(ctx context.Context, poID int64) ([]int64, error) {
	return m.menuLinks[poID], nil
}

func (m *lifecyclePORepo) HasPenerimaan(ctx context.Context, poID int64) (bool, error) {
	return m.receipts[poID], nil
}

// ReserveNumberSeq tracks the reservation scope and returns a monotonically
// increasing sequence per (sppgID, tahunBulan), mimicking the real repo.
func (m *lifecyclePORepo) ReserveNumberSeq(ctx context.Context, sppgID int64, tahunBulan string) (int, error) {
	m.reservations = append(m.reservations, tahunBulan)
	max := 0
	for _, seq := range m.numberSeqs[tahunBulan] {
		if seq > max {
			max = seq
		}
	}
	next := max + 1
	m.numberSeqs[tahunBulan] = append(m.numberSeqs[tahunBulan], next)
	return next, nil
}

func (m *lifecyclePORepo) Update(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, replaceItems bool, menuIDs []int64, replaceMenus bool) (*domain.PurchaseOrder, []domain.POItem, error) {
	current := m.pos[po.ID]
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
		m.items[po.ID] = replaced
		updated = replaced
	}
	if replaceMenus {
		links := make([]int64, len(menuIDs))
		copy(links, menuIDs)
		m.menuLinks[po.ID] = links
	}
	return current, updated, nil
}

func (m *lifecyclePORepo) Delete(ctx context.Context, id int64) error {
	po := m.pos[id]
	if po == nil || po.Status != domain.POStatusDraf {
		return domain.ErrPOInvalidStatus
	}
	m.deleted[id] = true
	return nil
}

func (m *lifecyclePORepo) SetDikirim(ctx context.Context, id int64, userID int64, at time.Time) (*domain.PurchaseOrder, error) {
	po := m.pos[id]
	if po == nil || po.Status != domain.POStatusDraf {
		return nil, domain.ErrPOInvalidStatus
	}
	po.Status = domain.POStatusDikirim
	po.DikirimOleh = &userID
	po.DikirimAt = &at
	updated := *po
	return &updated, nil
}

func (m *lifecyclePORepo) SetStatus(ctx context.Context, id int64, fromStatus, toStatus string, userID int64, alasan *string, at time.Time) (*domain.PurchaseOrder, error) {
	po := m.pos[id]
	if po == nil || po.Status != fromStatus {
		return nil, domain.ErrPOInvalidStatus
	}
	po.Status = toStatus
	po.DitutupOleh = &userID
	po.DitutupAt = &at
	po.AlasanTutup = alasan
	updated := *po
	return &updated, nil
}

func (m *lifecyclePORepo) GetDetail(ctx context.Context, id int64) (*domain.PODetail, error) {
	if m.deleted[id] {
		return nil, nil
	}
	po := m.pos[id]
	if po == nil {
		return nil, nil
	}
	detail := &domain.PODetail{PurchaseOrder: *po}
	for _, item := range m.items[id] {
		detail.Items = append(detail.Items, domain.PODetailItem{
			ID:        item.ID,
			BahanID:   item.BahanID,
			Terpenuhi: m.terpenuhi[po.ID][item.ID],
		})
	}
	return detail, nil
}

type mockSiklusRepo struct {
	siklus map[int64]*domain.SiklusDana
}

func (m *mockSiklusRepo) FindByID(ctx context.Context, id int64) (*domain.SiklusDana, error) {
	return m.siklus[id], nil
}

type trackingNotifRepo struct {
	jenisNotified []string
	rolesNotified [][]string
}

func (m *trackingNotifRepo) NotifySPPGRoles(ctx context.Context, sppgID int64, roles []string, jenis, judul, isi, refTabel string, refID int64) error {
	m.jenisNotified = append(m.jenisNotified, jenis)
	m.rolesNotified = append(m.rolesNotified, roles)
	return nil
}

type trackingAuditRepo struct {
	aksi []string
}

func (m *trackingAuditRepo) Append(ctx context.Context, e *authdomain.AuditEntry) error {
	m.aksi = append(m.aksi, e.Aksi)
	return nil
}

// ===== Test scaffolding =====

var lifecycleNow = time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)

type lifecycleFixture struct {
	po      *lifecyclePORepo
	pemasok *mockPemasokRepo
	bahan   *mockBahanRepo
	menus   *mockMenuRepo
	siklus  *mockSiklusRepo
	sppg    *mockSPPGProvider
	audits  *trackingAuditRepo
	notifs  *trackingNotifRepo
	svc     *Service
}

func newLifecycleFixture() *lifecycleFixture {
	f := &lifecycleFixture{
		po:      newLifecyclePORepo(),
		pemasok: newMockPemasokRepo(),
		bahan:   newMockBahanRepo(),
		menus:   newMockMenuRepo(),
		siklus:  &mockSiklusRepo{siklus: make(map[int64]*domain.SiklusDana)},
		sppg:    &mockSPPGProvider{kode: "SPPG001"},
		audits:  &trackingAuditRepo{},
		notifs:  &trackingNotifRepo{},
	}
	f.svc = New(Deps{
		Pemasok: f.pemasok,
		Bahan:   f.bahan,
		Menus:   f.menus,
		Siklus:  f.siklus,
		PO:      f.po,
		SPPG:    f.sppg,
		Audits:  f.audits,
		Notifs:  f.notifs,
		Clock:   func() time.Time { return lifecycleNow },
	})
	// Default active supplier and bahan.
	f.pemasok.pemasoks[10] = &domain.Pemasok{ID: 10, SPPGID: 1, Nama: "PT Segar", Aktif: true}
	f.pemasok.pemasoks[11] = &domain.Pemasok{ID: 11, SPPGID: 1, Nama: "PT Nonaktif", Aktif: false}
	f.pemasok.pemasoks[12] = &domain.Pemasok{ID: 12, SPPGID: 2, Nama: "PT Lain SPPG", Aktif: true}
	f.bahan.bahans[100] = &domain.Bahan{ID: 100, Nama: "Telur", Satuan: "butir", Aktif: true}
	f.bahan.bahans[101] = &domain.Bahan{ID: 101, Nama: "Wortel", Satuan: "kg", Aktif: true}
	return f
}

func akuntanActor() *authdomain.Claims {
	return &authdomain.Claims{UserID: 7, Role: authdomain.RoleAkuntan, SPPGID: int64Ptr(1)}
}

func kepalaActor() *authdomain.Claims {
	return &authdomain.Claims{UserID: 8, Role: authdomain.RoleKepalaSPPG, SPPGID: int64Ptr(1)}
}

// seedDraftPO inserts a draft PO with one item (bahan 100, qty 10).
func (f *lifecycleFixture) seedDraftPO(t *testing.T, sppgID int64, diminta time.Time) *domain.PurchaseOrder {
	t.Helper()
	po := &domain.PurchaseOrder{
		SPPGID:          sppgID,
		NomorPO:         "PO/SPPG001/202610/0001",
		PemasokID:       10,
		TglPO:           lifecycleNow.Truncate(24 * time.Hour),
		TglKirimDiminta: &diminta,
		Status:          domain.POStatusDraf,
		DibuatOleh:      7,
	}
	items := []domain.POItem{{POID: 1, BahanID: 100, QtyPesan: 10, HargaSatuan: 2000, ID: 1}}
	return f.po.seed(po, items)
}

// TestCreatePurchaseOrder_ReservesNumberForNonReuse
// "nomor PO terhapus tidak dipakai ulang": every issued number must be
// reserved transactionally at creation time via ReserveNumberSeq.
func TestCreatePurchaseOrder_ReservesNumberForNonReuse(t *testing.T) {
	f := newLifecycleFixture()
	actor := akuntanActor()
	in := domain.CreatePOInput{
		PemasokID:       10,
		TglKirimDiminta: "2026-10-27",
		Items:           []domain.POItemInput{{BahanID: 100, QtyPesan: 10, HargaSatuan: 2000}},
	}
	res, err := f.svc.CreatePurchaseOrder(context.Background(), actor, in, nil)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if res.PO.NomorPO != "PO/SPPG001/202610/0001" {
		t.Fatalf("unexpected nomor %q", res.PO.NomorPO)
	}
	if len(f.po.reservations) != 1 || f.po.reservations[0] != "202610" {
		t.Fatalf("number was not reserved for sppg scope, got %v", f.po.reservations)
	}

	// A second PO in the same month must advance the sequence, never reuse.
	res2, err := f.svc.CreatePurchaseOrder(context.Background(), actor, in, nil)
	if err != nil {
		t.Fatalf("expected success on second create, got %v", err)
	}
	if res2.PO.NomorPO != "PO/SPPG001/202610/0002" {
		t.Fatalf("second nomor must advance, got %q", res2.PO.NomorPO)
	}
	if len(f.po.reservations) != 2 {
		t.Fatalf("second create must reserve its number too, got %v", f.po.reservations)
	}
}

func TestUpdatePurchaseOrder_OnlyDraftAllowed(t *testing.T) {
	f := newLifecycleFixture()
	actor := akuntanActor()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))

	if _, err := f.svc.UpdatePurchaseOrder(context.Background(), actor, po.ID, domain.UpdatePOInput{}, nil); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	// dikirim PO cannot be patched
	po.Status = domain.POStatusDikirim
	if _, err := f.svc.UpdatePurchaseOrder(context.Background(), actor, po.ID, domain.UpdatePOInput{}, nil); !errors.Is(err, domain.ErrPOInvalidStatus) {
		t.Fatalf("expected ErrPOInvalidStatus, got %v", err)
	}
}

func TestUpdatePurchaseOrder_CrossSPPGHidden(t *testing.T) {
	f := newLifecycleFixture()
	po := f.seedDraftPO(t, 2, lifecycleNow.Add(24*time.Hour)) // other SPPG
	if _, err := f.svc.UpdatePurchaseOrder(context.Background(), akuntanActor(), po.ID, domain.UpdatePOInput{}, nil); !errors.Is(err, domain.ErrPONotFound) {
		t.Fatalf("expected ErrPONotFound, got %v", err)
	}
}

func TestUpdatePurchaseOrder_RequiresAkuntan(t *testing.T) {
	f := newLifecycleFixture()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))
	if _, err := f.svc.UpdatePurchaseOrder(context.Background(), kepalaActor(), po.ID, domain.UpdatePOInput{}, nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestUpdatePurchaseOrder_KeepsUnsentFields(t *testing.T) {
	f := newLifecycleFixture()
	actor := akuntanActor()
	diminta := lifecycleNow.Add(24 * time.Hour)
	po := f.seedDraftPO(t, 1, diminta)

	// Only catatan provided: pemasok, tgl, items must stay.
	in := domain.UpdatePOInput{Catatan: strPtr("tambahan telur")}
	res, err := f.svc.UpdatePurchaseOrder(context.Background(), actor, po.ID, in, nil)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if res.Detail.Catatan == nil || *res.Detail.Catatan != "tambahan telur" {
		t.Fatalf("catatan not applied")
	}
	if po.PemasokID != 10 || len(f.po.items[po.ID]) != 1 || f.po.items[po.ID][0].BahanID != 100 {
		t.Fatalf("other fields must not change")
	}
	if got, _ := f.po.ListMenuIDsForPO(context.Background(), po.ID); got != nil {
		t.Fatalf("menu links must not change")
	}
}

func TestUpdatePurchaseOrder_ReplacesItemsAndMenusAtomically(t *testing.T) {
	f := newLifecycleFixture()
	actor := akuntanActor()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))

	// Seed current menu link + a second item.
	f.po.menuLinks[po.ID] = []int64{50}
	f.po.items[po.ID] = append(f.po.items[po.ID], domain.POItem{ID: 2, POID: po.ID, BahanID: 101, QtyPesan: 2, HargaSatuan: 500})
	f.menus.menus[50] = &domain.Menu{ID: 50, SPPGID: 1, Status: domain.MenuStatusDisetujui, Tanggal: lifecycleNow.Add(48 * time.Hour)}

	in := domain.UpdatePOInput{
		ItemsSet:   true,
		Items:      []domain.POItemInput{{BahanID: 101, QtyPesan: 5, HargaSatuan: 800}},
		MenuIDsSet: true,
		MenuIDs:    []int64{50},
	}
	if _, err := f.svc.UpdatePurchaseOrder(context.Background(), actor, po.ID, in, nil); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	items := f.po.items[po.ID]
	if len(items) != 1 || items[0].BahanID != 101 || items[0].QtyPesan != 5 {
		t.Fatalf("items must be fully replaced, got %+v", items)
	}
	links, _ := f.po.ListMenuIDsForPO(context.Background(), po.ID)
	if len(links) != 1 || links[0] != 50 {
		t.Fatalf("menu links must be fully replaced, got %v", links)
	}
}

func TestUpdatePurchaseOrder_InvalidPemasok(t *testing.T) {
	f := newLifecycleFixture()
	actor := akuntanActor()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))

	// inactive pemasok -> 422 PemasokNonaktif
	if _, err := f.svc.UpdatePurchaseOrder(context.Background(), actor, po.ID, domain.UpdatePOInput{PemasokID: int64Ptr(11)}, nil); !errors.Is(err, domain.ErrPemasokInactive) {
		t.Fatalf("expected ErrPemasokInactive, got %v", err)
	}
	// pemasok from another SPPG -> 404
	if _, err := f.svc.UpdatePurchaseOrder(context.Background(), actor, po.ID, domain.UpdatePOInput{PemasokID: int64Ptr(12)}, nil); !errors.Is(err, domain.ErrPemasokNotFound) {
		t.Fatalf("expected ErrPemasokNotFound, got %v", err)
	}
}

func TestUpdatePurchaseOrder_PastDeliveryDateRejected(t *testing.T) {
	f := newLifecycleFixture()
	actor := akuntanActor()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))
	in := domain.UpdatePOInput{TglKirimDiminta: strPtr("2026-10-25")}
	if _, err := f.svc.UpdatePurchaseOrder(context.Background(), actor, po.ID, in, nil); !domain.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestUpdatePurchaseOrder_WritesAudit(t *testing.T) {
	f := newLifecycleFixture()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))
	if _, err := f.svc.UpdatePurchaseOrder(context.Background(), akuntanActor(), po.ID, domain.UpdatePOInput{}, nil); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(f.audits.aksi) == 0 || f.audits.aksi[0] != authdomain.AuditUpdate {
		t.Fatalf("expected audit update entry, got %v", f.audits.aksi)
	}
}

func TestDeletePurchaseOrder_DraftOnly(t *testing.T) {
	f := newLifecycleFixture()
	actor := akuntanActor()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))

	if err := f.svc.DeletePurchaseOrder(context.Background(), actor, po.ID, nil); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got, _ := f.po.FindByID(context.Background(), po.ID); got != nil {
		t.Fatalf("expected PO deleted")
	}

	again := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))
	again.Status = domain.POStatusDikirim
	if err := f.svc.DeletePurchaseOrder(context.Background(), actor, again.ID, nil); !errors.Is(err, domain.ErrPOInvalidStatus) {
		t.Fatalf("expected ErrPOInvalidStatus, got %v", err)
	}
}

func TestDeletePurchaseOrder_RoleAndOwnership(t *testing.T) {
	f := newLifecycleFixture()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))

	if err := f.svc.DeletePurchaseOrder(context.Background(), kepalaActor(), po.ID, nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for kepala_sppg, got %v", err)
	}
	other := f.seedDraftPO(t, 2, lifecycleNow.Add(24*time.Hour))
	if err := f.svc.DeletePurchaseOrder(context.Background(), akuntanActor(), other.ID, nil); !errors.Is(err, domain.ErrPONotFound) {
		t.Fatalf("expected ErrPONotFound, got %v", err)
	}
}

func TestKirimPurchaseOrder_HappyPath(t *testing.T) {
	f := newLifecycleFixture()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))

	res, err := f.svc.KirimPurchaseOrder(context.Background(), akuntanActor(), po.ID, nil)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if po.Status != domain.POStatusDikirim || po.DikirimAt == nil || po.DikirimOleh == nil {
		t.Fatalf("expected dikirim with stamps, got %+v", po)
	}
	if res.Detail.Status != domain.POStatusDikirim {
		t.Fatalf("expected detail status dikirim, got %s", res.Detail.Status)
	}
	// notifications for petugas_dapur + kepala_sppg
	if len(f.notifs.jenisNotified) != 1 || f.notifs.jenisNotified[0] != "po_dikirim" {
		t.Fatalf("expected po_dikirim notification, got %v", f.notifs.jenisNotified)
	}
	roles := f.notifs.rolesNotified[0]
	if len(roles) != 2 {
		t.Fatalf("expected 2 notified roles, got %v", roles)
	}
}

func TestKirimPurchaseOrder_Guards(t *testing.T) {
	f := newLifecycleFixture()
	actor := akuntanActor()

	// past delivery date
	done := f.seedDraftPO(t, 1, lifecycleNow.Add(-24*time.Hour))
	if _, err := f.svc.KirimPurchaseOrder(context.Background(), actor, done.ID, nil); !errors.Is(err, domain.ErrPOCannotBeSent) {
		t.Fatalf("expected ErrPOCannotBeSent, got %v", err)
	}

	// wrong status
	wrong := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))
	wrong.Status = domain.POStatusSelesai
	if _, err := f.svc.KirimPurchaseOrder(context.Background(), actor, wrong.ID, nil); !errors.Is(err, domain.ErrPOInvalidStatus) {
		t.Fatalf("expected ErrPOInvalidStatus, got %v", err)
	}

	// inactive pemasok
	inactive := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))
	inactive.PemasokID = 11
	if _, err := f.svc.KirimPurchaseOrder(context.Background(), actor, inactive.ID, nil); !errors.Is(err, domain.ErrPemasokInactive) {
		t.Fatalf("expected ErrPemasokInactive, got %v", err)
	}

	// no items
	empty := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))
	f.po.items[empty.ID] = nil
	if _, err := f.svc.KirimPurchaseOrder(context.Background(), actor, empty.ID, nil); !errors.Is(err, domain.ErrPOCannotBeSent) {
		t.Fatalf("expected ErrPOCannotBeSent, got %v", err)
	}

	// kepala_sppg cannot kirim (matrix: only batal)
	if _, err := f.svc.KirimPurchaseOrder(context.Background(), kepalaActor(), f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour)).ID, nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestBatalPurchaseOrder_DraftWithoutReceipt(t *testing.T) {
	f := newLifecycleFixture()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))

	res, err := f.svc.BatalPurchaseOrder(context.Background(), kepalaActor(), po.ID, "pemasok tidak sanggup", nil)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if po.Status != domain.POStatusBatal || po.AlasanTutup == nil {
		t.Fatalf("expected batal with alasan, got %+v", po)
	}
	if res.Detail.Status != domain.POStatusBatal {
		t.Fatalf("expected detail batal, got %s", res.Detail.Status)
	}
	if len(f.notifs.jenisNotified) != 0 {
		t.Fatalf("no notification expected for unsent PO, got %v", f.notifs.jenisNotified)
	}
	if len(f.audits.aksi) == 0 {
		t.Fatalf("expected audit entry")
	}
}

func TestBatalPurchaseOrder_SentPONotifiesPetugasDapur(t *testing.T) {
	f := newLifecycleFixture()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))
	po.Status = domain.POStatusDikirim

	if _, err := f.svc.BatalPurchaseOrder(context.Background(), akuntanActor(), po.ID, "ganti pemasok", nil); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(f.notifs.jenisNotified) != 1 || f.notifs.jenisNotified[0] != "po_batal" {
		t.Fatalf("expected po_batal notification, got %v", f.notifs.jenisNotified)
	}
	if roles := f.notifs.rolesNotified[0]; len(roles) != 1 || roles[0] != string(authdomain.RolePetugasDapur) {
		t.Fatalf("expected only petugas_dapur, got %v", roles)
	}
}

func TestBatalPurchaseOrder_WithReceiptRejected(t *testing.T) {
	f := newLifecycleFixture()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))
	po.Status = domain.POStatusDikirim
	f.po.receipts[po.ID] = true

	if _, err := f.svc.BatalPurchaseOrder(context.Background(), akuntanActor(), po.ID, "sudah datang sebagian", nil); !errors.Is(err, domain.ErrPOHasReceipt) {
		t.Fatalf("expected ErrPOHasReceipt, got %v", err)
	}
	if po.Status != domain.POStatusDikirim {
		t.Fatalf("status must not change")
	}
}

func TestBatalPurchaseOrder_InputAndRoleGuards(t *testing.T) {
	f := newLifecycleFixture()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))

	// alasan too short -> 400
	if _, err := f.svc.BatalPurchaseOrder(context.Background(), akuntanActor(), po.ID, "hm", nil); !domain.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
	// closed PO -> 409
	po.Status = domain.POStatusSelesai
	if _, err := f.svc.BatalPurchaseOrder(context.Background(), akuntanActor(), po.ID, "coba batal", nil); !errors.Is(err, domain.ErrPOInvalidStatus) {
		t.Fatalf("expected ErrPOInvalidStatus, got %v", err)
	}
	// ahli_gizi forbidden
	po.Status = domain.POStatusDraf
	other := &authdomain.Claims{UserID: 9, Role: authdomain.RoleAhliGizi, SPPGID: int64Ptr(1)}
	if _, err := f.svc.BatalPurchaseOrder(context.Background(), other, po.ID, "coba batal", nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	// other SPPG hidden
	foreign := f.seedDraftPO(t, 2, lifecycleNow.Add(24*time.Hour))
	if _, err := f.svc.BatalPurchaseOrder(context.Background(), akuntanActor(), foreign.ID, "coba batal", nil); !errors.Is(err, domain.ErrPONotFound) {
		t.Fatalf("expected ErrPONotFound, got %v", err)
	}
}

func TestTutupPurchaseOrder_ListsUnfulfilledItems(t *testing.T) {
	f := newLifecycleFixture()
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))
	po.Status = domain.POStatusDiterimaSebagian
	f.po.terpenuhi[po.ID] = map[int64]bool{1: true}
	// second item never fulfilled
	f.po.items[po.ID] = append(f.po.items[po.ID], domain.POItem{ID: 2, POID: po.ID, BahanID: 101, QtyPesan: 5, HargaSatuan: 900})

	res, err := f.svc.TutupPurchaseOrder(context.Background(), akuntanActor(), po.ID, "sisa tidak dikirim lagi", nil)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if po.Status != domain.POStatusSelesai {
		t.Fatalf("expected selesai, got %s", po.Status)
	}
	if len(res.ItemTidakTerpenuhi) != 1 || res.ItemTidakTerpenuhi[0].ID != 2 {
		t.Fatalf("expected exactly item 2 unfulfilled, got %+v", res.ItemTidakTerpenuhi)
	}
}

func TestTutupPurchaseOrder_Guards(t *testing.T) {
	f := newLifecycleFixture()
	actor := akuntanActor()

	// alasan required
	po := f.seedDraftPO(t, 1, lifecycleNow.Add(24*time.Hour))
	po.Status = domain.POStatusDiterimaSebagian
	if _, err := f.svc.TutupPurchaseOrder(context.Background(), actor, po.ID, "", nil); !domain.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}

	// wrong status -> 409
	po.Status = domain.POStatusDikirim
	if _, err := f.svc.TutupPurchaseOrder(context.Background(), actor, po.ID, "tutup dulu", nil); !errors.Is(err, domain.ErrPOInvalidStatus) {
		t.Fatalf("expected ErrPOInvalidStatus, got %v", err)
	}

	// kepala_sppg cannot tutup
	po.Status = domain.POStatusDiterimaSebagian
	if _, err := f.svc.TutupPurchaseOrder(context.Background(), kepalaActor(), po.ID, "tutup dulu", nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

package application

import (
	"context"
	"errors"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

var _ PenerimaanRepository = (*fakePenerimaanRepo)(nil)

type fakePenerimaanRepo struct {
	lots       map[int64]*domain.PenerimaanBahan
	nextID     int64
	clientRefs map[string]bool
	createErr  error
}

func newFakePenerimaanRepo() *fakePenerimaanRepo {
	return &fakePenerimaanRepo{
		lots:       make(map[int64]*domain.PenerimaanBahan),
		clientRefs: make(map[string]bool),
	}
}

func (f *fakePenerimaanRepo) CreatePenerimaan(ctx context.Context, poID int64, in domain.CreatePenerimaanInput, diperiksaOleh int64, at time.Time) ([]*domain.PenerimaanBahan, *domain.PurchaseOrder, error) {
	if f.createErr != nil {
		return nil, nil, f.createErr
	}
	if f.clientRefs[in.ClientRef] {
		var existing []*domain.PenerimaanBahan
		for _, lot := range f.lots {
			existing = append(existing, lot)
		}
		return existing, &domain.PurchaseOrder{ID: poID, Status: domain.POStatusDiterimaSebagian}, nil
	}
	f.clientRefs[in.ClientRef] = true

	var lots []*domain.PenerimaanBahan
	for _, it := range in.Items {
		f.nextID++
		lot := &domain.PenerimaanBahan{
			ID:              f.nextID,
			POItemID:        it.POItemID,
			KodeLot:         "LOT-TEST-20261027-001",
			WaktuTerima:     at,
			QtyDiterima:     it.QtyDiterima,
			QtyDitolak:      it.QtyDitolak,
			AlasanTolak:     it.AlasanTolak,
			SuhuTerima:      it.SuhuTerima,
			Kondisi:         it.Kondisi,
			TglKedaluwarsa:  nil,
			FotoURL:         "",
			DiperiksaOleh:   diperiksaOleh,
			NilaiPotongNota: it.NilaiPotongNota,
			SPPGID:          1,
			ClientRef:       in.ClientRef,
			LolosSuhu:       it.LolosSuhu,
			Catatan:         it.Catatan,
			CreatedAt:       at,
		}
		f.lots[lot.ID] = lot
		lots = append(lots, lot)
	}
	return lots, &domain.PurchaseOrder{ID: poID, Status: domain.POStatusDiterimaSebagian}, nil
}

func (f *fakePenerimaanRepo) List(ctx context.Context, sppgID int64, filter domain.PenerimaanFilter) ([]domain.PenerimaanListItem, int, error) {
	var out []domain.PenerimaanListItem
	for _, lot := range f.lots {
		out = append(out, domain.PenerimaanListItem{
			ID:          lot.ID,
			KodeLot:     lot.KodeLot,
			NomorPO:     "PO/TEST/202610/0001",
			PemasokNama: "PT Test",
			BahanID:     1,
			BahanNama:   "Bahan Test",
			WaktuTerima: lot.WaktuTerima,
			QtyDiterima: lot.QtyDiterima,
			QtyDitolak:  lot.QtyDitolak,
			Kondisi:     lot.Kondisi,
		})
	}
	return out, len(out), nil
}

func (f *fakePenerimaanRepo) GetByID(ctx context.Context, id int64) (*domain.PenerimaanDetail, error) {
	for _, lot := range f.lots {
		if lot.ID == id {
			return &domain.PenerimaanDetail{
				ID:          lot.ID,
				POItemID:    lot.POItemID,
				NomorPO:     "PO/TEST/202610/0001",
				BahanNama:   "Bahan Test",
				KodeLot:     lot.KodeLot,
				WaktuTerima: lot.WaktuTerima,
				QtyDiterima: lot.QtyDiterima,
				QtyDitolak:  lot.QtyDitolak,
				Kondisi:     lot.Kondisi,
				SPPGID:      lot.SPPGID,
				POStatus:    domain.POStatusDiterimaSebagian,
			}, nil
		}
	}
	return nil, nil
}

// ---- fakes for other deps ----

type fakePORepo struct{}

func (f *fakePORepo) ListBySPPGAndStatus(ctx context.Context, sppgID int64, statuses []string) ([]domain.PurchaseOrder, error) {
	return nil, nil
}
func (f *fakePORepo) ListItemsByPOIDs(ctx context.Context, poIDs []int64) (map[int64][]domain.POItem, error) {
	return map[int64][]domain.POItem{1: {{ID: 1, POID: 1, BahanID: 1, QtyPesan: 100, HargaSatuan: 10000}}}, nil
}
func (f *fakePORepo) ListPOMenus(ctx context.Context, menuIDs []int64) (map[int64][]int64, error) {
	return nil, nil
}
func (f *fakePORepo) Create(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, menuIDs []int64) (*domain.PurchaseOrder, []domain.POItem, error) {
	return nil, nil, nil
}
func (f *fakePORepo) FindByID(ctx context.Context, id int64) (*domain.PurchaseOrder, error) {
	return &domain.PurchaseOrder{ID: id, SPPGID: 1, Status: domain.POStatusDikirim}, nil
}
func (f *fakePORepo) List(ctx context.Context, sppgID int64, filter domain.POFilter) ([]domain.POListItem, int, error) {
	return nil, 0, nil
}
func (f *fakePORepo) GetDetail(ctx context.Context, id int64) (*domain.PODetail, error) {
	return nil, nil
}
func (f *fakePORepo) GetPOItemRekap(ctx context.Context, poItemIDs []int64) (map[int64]domain.POItemRekap, error) {
	out := make(map[int64]domain.POItemRekap)
	for _, id := range poItemIDs {
		out[id] = domain.POItemRekap{POItemID: id, QtyDiterima: 0}
	}
	return out, nil
}
func (f *fakePORepo) Update(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, replaceItems bool, menuIDs []int64, replaceMenus bool) (*domain.PurchaseOrder, []domain.POItem, error) {
	return nil, nil, nil
}
func (f *fakePORepo) Delete(ctx context.Context, id int64) error { return nil }
func (f *fakePORepo) ListMenuIDsForPO(ctx context.Context, poID int64) ([]int64, error) {
	return nil, nil
}
func (f *fakePORepo) HasPenerimaan(ctx context.Context, poID int64) (bool, error) { return false, nil }
func (f *fakePORepo) SetDikirim(ctx context.Context, id int64, userID int64, at time.Time) (*domain.PurchaseOrder, error) {
	return nil, nil
}
func (f *fakePORepo) SetStatus(ctx context.Context, id int64, fromStatus, toStatus string, userID int64, alasan *string, at time.Time) (*domain.PurchaseOrder, error) {
	return nil, nil
}
func (f *fakePORepo) ReserveNumberSeq(ctx context.Context, sppgID int64, tahunBulan string) (int, error) {
	return 1, nil
}

type fakeBahanRepo struct{}

func (f *fakeBahanRepo) FindByID(ctx context.Context, id int64) (*domain.Bahan, error) {
	return &domain.Bahan{ID: id, Nama: "Bahan Test", Satuan: "kg", Aktif: true, MudahRusak: false}, nil
}
func (f *fakeBahanRepo) List(ctx context.Context, filter domain.BahanFilter) ([]domain.Bahan, int, error) {
	return nil, 0, nil
}

type fakeSPPGProvider struct{}

func (f *fakeSPPGProvider) GetKode(ctx context.Context, id int64) (string, error) { return "TST", nil }

type fakeAuditRepo struct{}

func (f *fakeAuditRepo) Append(ctx context.Context, e *authdomain.AuditEntry) error { return nil }

type fakeNotifRepo struct{}

func (f *fakeNotifRepo) NotifySPPGRoles(ctx context.Context, sppgID int64, roles []string, jenis, judul, isi, refTabel string, refID int64) error {
	return nil
}

func TestCreatePenerimaan_Success(t *testing.T) {
	f := newFakePenerimaanRepo()
	svc := New(Deps{
		Penerimaan: f,
		PO:         &fakePORepo{},
		Bahan:      &fakeBahanRepo{},
		SPPG:       &fakeSPPGProvider{},
		Audits:     &fakeAuditRepo{},
		Notifs:     &fakeNotifRepo{},
		Clock:      func() time.Time { return time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC) },
	})

	actor := &authdomain.Claims{UserID: 7, Role: authdomain.RoleAkuntan, SPPGID: int64Ptr(1)}

	in := domain.CreatePenerimaanInput{
		ClientRef: "550e8400-e29b-41d4-a716-446655440000",
		Items: []domain.PenerimaanItem{
			{
				POItemID:        1,
				QtyDiterima:     10,
				QtyDitolak:      0,
				Kondisi:         domain.KondisiBaik,
				FotoBerkasID:    100,
				NilaiPotongNota: 0,
			},
		},
	}
	res, err := svc.CreatePenerimaan(context.Background(), actor, 1, in, nil)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(res.Lots) != 1 {
		t.Fatalf("expected 1 lot, got %d", len(res.Lots))
	}
	if res.Lots[0].KodeLot == "" {
		t.Fatal("kode_lot should be generated")
	}
	if res.PO.Status != domain.POStatusDiterimaSebagian {
		t.Fatalf("expected PO status diterima_sebagian, got %s", res.PO.Status)
	}
}

func TestCreatePenerimaan_RequiresAkuntanOrDapur(t *testing.T) {
	f := newFakePenerimaanRepo()
	svc := New(Deps{
		Penerimaan: f,
		Clock:      func() time.Time { return time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC) },
	})

	kepala := &authdomain.Claims{UserID: 8, Role: authdomain.RoleKepalaSPPG, SPPGID: int64Ptr(1)}
	_, err := svc.CreatePenerimaan(context.Background(), kepala, 1, domain.CreatePenerimaanInput{}, nil)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	admin := &authdomain.Claims{UserID: 9, Role: authdomain.RoleAdmin, SPPGID: int64Ptr(1)}
	_, err = svc.CreatePenerimaan(context.Background(), admin, 1, domain.CreatePenerimaanInput{}, nil)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestCreatePenerimaan_WrongSPPG(t *testing.T) {
	f := newFakePenerimaanRepo()
	svc := New(Deps{
		Penerimaan: f,
		PO:         &fakePORepo{},
		Bahan:      &fakeBahanRepo{},
		SPPG:       &fakeSPPGProvider{},
		Audits:     &fakeAuditRepo{},
		Notifs:     &fakeNotifRepo{},
		Clock:      func() time.Time { return time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC) },
	})

	actor := &authdomain.Claims{UserID: 7, Role: authdomain.RoleAkuntan, SPPGID: int64Ptr(2)}
	in := domain.CreatePenerimaanInput{
		ClientRef: "550e8400-e29b-41d4-a716-446655440003",
		Items: []domain.PenerimaanItem{
			{POItemID: 1, QtyDiterima: 10, QtyDitolak: 0, Kondisi: domain.KondisiBaik, FotoBerkasID: 100},
		},
	}
	_, err := svc.CreatePenerimaan(context.Background(), actor, 1, in, nil)
	if !errors.Is(err, domain.ErrPONotFound) {
		t.Fatalf("expected ErrPONotFound for wrong SPPG, got %v", err)
	}
}

func TestCreatePenerimaan_IdempotentReplay(t *testing.T) {
	f := newFakePenerimaanRepo()
	svc := New(Deps{
		Penerimaan: f,
		PO:         &fakePORepo{},
		Bahan:      &fakeBahanRepo{},
		SPPG:       &fakeSPPGProvider{},
		Audits:     &fakeAuditRepo{},
		Notifs:     &fakeNotifRepo{},
		Clock:      func() time.Time { return time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC) },
	})

	actor := &authdomain.Claims{UserID: 7, Role: authdomain.RoleAkuntan, SPPGID: int64Ptr(1)}
	in := domain.CreatePenerimaanInput{
		ClientRef: "550e8400-e29b-41d4-a716-446655440004",
		Items: []domain.PenerimaanItem{
			{POItemID: 1, QtyDiterima: 10, QtyDitolak: 0, Kondisi: domain.KondisiBaik, FotoBerkasID: 100},
		},
	}

	res1, err := svc.CreatePenerimaan(context.Background(), actor, 1, in, nil)
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	if len(res1.Lots) != 1 {
		t.Fatalf("expected 1 lot on first request, got %d", len(res1.Lots))
	}

	res2, err := svc.CreatePenerimaan(context.Background(), actor, 1, in, nil)
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	if len(res2.Lots) != 1 {
		t.Fatalf("expected 1 lot on replay, got %d", len(res2.Lots))
	}
	if res2.Lots[0].ID != res1.Lots[0].ID {
		t.Fatal("replay should return same lot ID")
	}
}

func TestListPenerimaan(t *testing.T) {
	f := newFakePenerimaanRepo()
	svc := New(Deps{
		Penerimaan: f,
		Clock:      func() time.Time { return time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC) },
	})

	actor := &authdomain.Claims{UserID: 7, Role: authdomain.RoleAkuntan, SPPGID: int64Ptr(1)}

	res, err := svc.ListPenerimaan(context.Background(), actor, domain.PenerimaanFilter{})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if res.Meta.Page != 1 || res.Meta.PerPage != 20 {
		t.Fatalf("unexpected meta: %+v", res.Meta)
	}
}

func TestGetPenerimaan_NotFound(t *testing.T) {
	f := newFakePenerimaanRepo()
	svc := New(Deps{
		Penerimaan: f,
		Clock:      func() time.Time { return time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC) },
	})

	actor := &authdomain.Claims{UserID: 7, Role: authdomain.RoleAkuntan, SPPGID: int64Ptr(1)}
	_, err := svc.GetPenerimaan(context.Background(), actor, 999999)
	if !errors.Is(err, domain.ErrLotNotFound) {
		t.Fatalf("expected ErrLotNotFound, got %v", err)
	}
}

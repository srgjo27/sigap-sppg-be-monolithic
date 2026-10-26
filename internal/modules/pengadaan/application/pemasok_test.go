package application

import (
	"context"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// Mock repositories for testing
type mockPemasokRepo struct {
	pemasoks map[int64]*domain.Pemasok
	nextID   int64
}

func newMockPemasokRepo() *mockPemasokRepo {
	return &mockPemasokRepo{
		pemasoks: make(map[int64]*domain.Pemasok),
		nextID:   1,
	}
}

func (m *mockPemasokRepo) Create(ctx context.Context, p *domain.Pemasok) (*domain.Pemasok, error) {
	p.ID = m.nextID
	m.nextID++
	m.pemasoks[p.ID] = p
	return p, nil
}

func (m *mockPemasokRepo) FindByID(ctx context.Context, id int64) (*domain.Pemasok, error) {
	return m.pemasoks[id], nil
}

func (m *mockPemasokRepo) FindByNameInsensitive(ctx context.Context, sppgID int64, nama string) (*domain.Pemasok, error) {
	for _, p := range m.pemasoks {
		if p.SPPGID == sppgID && p.Nama == nama {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockPemasokRepo) Update(ctx context.Context, p *domain.Pemasok) (*domain.Pemasok, error) {
	m.pemasoks[p.ID] = p
	return p, nil
}

func (m *mockPemasokRepo) List(ctx context.Context, sppgID int64, filter domain.PemasokFilter) ([]domain.Pemasok, int, error) {
	var result []domain.Pemasok
	for _, p := range m.pemasoks {
		if p.SPPGID == sppgID || sppgID == 0 {
			result = append(result, *p)
		}
	}
	return result, len(result), nil
}

func (m *mockPemasokRepo) GetKinerja(ctx context.Context, sppgID, pemasokID int64) (*domain.Kinerja, error) {
	return &domain.Kinerja{
		PeriodeHari:      90,
		JumlahPO:         0,
		TotalNilaiPO:     "0.00",
		TotalPotongNota:  "0.00",
		PersenDitolak:    0,
		PersenTepatWaktu: 0,
	}, nil
}

type mockBahanRepo struct {
	bahans map[int64]*domain.Bahan
}

func newMockBahanRepo() *mockBahanRepo {
	return &mockBahanRepo{bahans: make(map[int64]*domain.Bahan)}
}

func (m *mockBahanRepo) FindByID(ctx context.Context, id int64) (*domain.Bahan, error) {
	return m.bahans[id], nil
}

func (m *mockBahanRepo) List(ctx context.Context, filter domain.BahanFilter) ([]domain.Bahan, int, error) {
	var result []domain.Bahan
	for _, b := range m.bahans {
		result = append(result, *b)
	}
	return result, len(result), nil
}

type mockMenuRepo struct {
	menus map[int64]*domain.Menu
	items map[int64][]domain.MenuBahan
}

func newMockMenuRepo() *mockMenuRepo {
	return &mockMenuRepo{
		menus: make(map[int64]*domain.Menu),
		items: make(map[int64][]domain.MenuBahan),
	}
}

func (m *mockMenuRepo) ListByScope(ctx context.Context, sppgID *int64, dari, sampai time.Time, status *string) ([]domain.Menu, error) {
	var result []domain.Menu
	for _, menu := range m.menus {
		if sppgID != nil && menu.SPPGID != *sppgID {
			continue
		}
		if status != nil && menu.Status != *status {
			continue
		}
		result = append(result, *menu)
	}
	return result, nil
}

func (m *mockMenuRepo) ListItems(ctx context.Context, menuIDs []int64) (map[int64][]domain.MenuBahan, error) {
	result := make(map[int64][]domain.MenuBahan)
	for _, id := range menuIDs {
		result[id] = m.items[id]
	}
	return result, nil
}

func (m *mockMenuRepo) FindByID(ctx context.Context, id int64) (*domain.Menu, error) {
	return m.menus[id], nil
}

// FindByIDs finds multiple menus by IDs.
func (m *mockMenuRepo) FindByIDs(ctx context.Context, ids []int64) ([]*domain.Menu, error) {
	var result []*domain.Menu
	for _, id := range ids {
		if menu, ok := m.menus[id]; ok {
			result = append(result, menu)
		}
	}
	return result, nil
}

type mockPORepo struct {
	items map[int64][]domain.POItem
	menus map[int64][]int64 // menu_id -> po_ids
}

func newMockPORepo() *mockPORepo {
	return &mockPORepo{
		items: make(map[int64][]domain.POItem),
		menus: make(map[int64][]int64),
	}
}

func (m *mockPORepo) ListBySPPGAndStatus(ctx context.Context, sppgID int64, statuses []string) ([]domain.PurchaseOrder, error) {
	return nil, nil
}

func (m *mockPORepo) ListItemsByPOIDs(ctx context.Context, poIDs []int64) (map[int64][]domain.POItem, error) {
	result := make(map[int64][]domain.POItem)
	for _, id := range poIDs {
		result[id] = m.items[id]
	}
	return result, nil
}

func (m *mockPORepo) ListPOMenus(ctx context.Context, menuIDs []int64) (map[int64][]int64, error) {
	result := make(map[int64][]int64)
	for _, id := range menuIDs {
		result[id] = m.menus[id]
	}
	return result, nil
}

// Create creates a purchase order with items and menu links (mock implementation).
func (m *mockPORepo) Create(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, menuIDs []int64) (*domain.PurchaseOrder, []domain.POItem, error) {
	// Assign mock IDs
	po.ID = 1
	for i := range items {
		items[i].ID = int64(i + 1)
		items[i].POID = po.ID
	}
	return po, items, nil
}

// ReserveNumberSeq returns a deterministic sequence (mock implementation).
func (m *mockPORepo) ReserveNumberSeq(ctx context.Context, sppgID int64, tahunBulan string) (int, error) {
	return 1, nil
}

// FindByID looks up a purchase order by ID (mock implementation).
func (m *mockPORepo) FindByID(ctx context.Context, id int64) (*domain.PurchaseOrder, error) {
	return nil, nil
}

// GetDetail returns detailed PO with items, rekap, penerimaan, and menus (mock implementation).
func (m *mockPORepo) GetDetail(ctx context.Context, id int64) (*domain.PODetail, error) {
	return nil, nil
}

// GetPOItemRekap returns rekap data for PO items (mock implementation).
func (m *mockPORepo) GetPOItemRekap(ctx context.Context, poItemIDs []int64) (map[int64]domain.POItemRekap, error) {
	return nil, nil
}

// List returns filtered purchase orders (mock implementation).
func (m *mockPORepo) List(ctx context.Context, sppgID int64, filter domain.POFilter) ([]domain.POListItem, int, error) {
	return nil, 0, nil
}

// Stubs port methods (behavioral fakes live in po_lifecycle_test.go).
func (m *mockPORepo) Update(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, replaceItems bool, menuIDs []int64, replaceMenus bool) (*domain.PurchaseOrder, []domain.POItem, error) {
	return po, items, nil
}

func (m *mockPORepo) Delete(ctx context.Context, id int64) error { return nil }

func (m *mockPORepo) ListMenuIDsForPO(ctx context.Context, poID int64) ([]int64, error) {
	return nil, nil
}

func (m *mockPORepo) HasPenerimaan(ctx context.Context, poID int64) (bool, error) { return false, nil }

func (m *mockPORepo) SetDikirim(ctx context.Context, id int64, userID int64, at time.Time) (*domain.PurchaseOrder, error) {
	return nil, nil
}

func (m *mockPORepo) SetStatus(ctx context.Context, id int64, fromStatus, toStatus string, userID int64, alasan *string, at time.Time) (*domain.PurchaseOrder, error) {
	return nil, nil
}

type mockSPPGProvider struct {
	kode string
}

func (m *mockSPPGProvider) GetKode(ctx context.Context, id int64) (string, error) {
	return m.kode, nil
}

type mockAuditRepo struct{}

func (m *mockAuditRepo) Append(ctx context.Context, e *authdomain.AuditEntry) error {
	return nil
}

type mockNotifRepo struct{}

func (m *mockNotifRepo) NotifySPPGRoles(ctx context.Context, sppgID int64, roles []string, jenis, judul, isi, refTabel string, refID int64) error {
	return nil
}

func TestCreatePemasok_Success(t *testing.T) {
	pemasokRepo := newMockPemasokRepo()
	svc := New(Deps{
		Pemasok: pemasokRepo,
		SPPG:    &mockSPPGProvider{kode: "SPPG001"},
		Audits:  &mockAuditRepo{},
		Notifs:  &mockNotifRepo{},
	})

	actor := &authdomain.Claims{
		UserID: 1,
		Role:   authdomain.RoleAkuntan,
		SPPGID: int64Ptr(1),
	}

	in := domain.CreatePemasokInput{
		Nama:       "PT ABC",
		JenisUsaha: "PT",
		NoHP:       "081234567890",
		Kontak:     "Budi",
		Alamat:     "Jl. Test",
	}

	res, err := svc.CreatePemasok(context.Background(), actor, in, nil)
	if err != nil {
		t.Fatalf("CreatePemasok failed: %v", err)
	}
	if res.Pemasok.ID != 1 {
		t.Errorf("expected ID 1, got %d", res.Pemasok.ID)
	}
	if res.Pemasok.Nama != "PT ABC" {
		t.Errorf("expected name PT ABC, got %s", res.Pemasok.Nama)
	}
	if res.Pemasok.NoHP != "+6281234567890" {
		t.Errorf("expected normalized no_hp +6281234567890, got %s", res.Pemasok.NoHP)
	}
	if !res.Pemasok.Aktif {
		t.Error("expected aktif=true")
	}
}

func TestCreatePemasok_DuplicateName(t *testing.T) {
	pemasokRepo := newMockPemasokRepo()
	svc := New(Deps{
		Pemasok: pemasokRepo,
		SPPG:    &mockSPPGProvider{kode: "SPPG001"},
		Audits:  &mockAuditRepo{},
		Notifs:  &mockNotifRepo{},
	})

	actor := &authdomain.Claims{
		UserID: 1,
		Role:   authdomain.RoleAkuntan,
		SPPGID: int64Ptr(1),
	}

	in := domain.CreatePemasokInput{
		Nama:       "PT ABC",
		JenisUsaha: "PT",
		NoHP:       "081234567890",
	}

	_, err := svc.CreatePemasok(context.Background(), actor, in, nil)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = svc.CreatePemasok(context.Background(), actor, in, nil)
	if err == nil {
		t.Error("expected duplicate name error")
	}
	if err != domain.ErrPemasokExists {
		t.Errorf("expected ErrPemasokExists, got %v", err)
	}
}

func TestCreatePemasok_ForbiddenRole(t *testing.T) {
	pemasokRepo := newMockPemasokRepo()
	svc := New(Deps{
		Pemasok: pemasokRepo,
		SPPG:    &mockSPPGProvider{kode: "SPPG001"},
		Audits:  &mockAuditRepo{},
		Notifs:  &mockNotifRepo{},
	})

	actor := &authdomain.Claims{
		UserID: 1,
		Role:   authdomain.RoleAhliGizi, // not allowed to create
		SPPGID: int64Ptr(1),
	}

	in := domain.CreatePemasokInput{
		Nama:       "PT ABC",
		JenisUsaha: "PT",
		NoHP:       "081234567890",
	}

	_, err := svc.CreatePemasok(context.Background(), actor, in, nil)
	if err != domain.ErrForbidden {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestUpdatePemasok_DeactivateRequiresAlasan(t *testing.T) {
	pemasokRepo := newMockPemasokRepo()
	// Pre-create a pemasok
	p := &domain.Pemasok{
		ID:         1,
		SPPGID:     1,
		Nama:       "PT ABC",
		JenisUsaha: "PT",
		NoHP:       "+6281234567890",
		Aktif:      true,
	}
	pemasokRepo.pemasoks[1] = p

	svc := New(Deps{
		Pemasok: pemasokRepo,
		SPPG:    &mockSPPGProvider{kode: "SPPG001"},
		Audits:  &mockAuditRepo{},
		Notifs:  &mockNotifRepo{},
	})

	actor := &authdomain.Claims{
		UserID: 1,
		Role:   authdomain.RoleAkuntan,
		SPPGID: int64Ptr(1),
	}

	// Try to deactivate without alasan
	in := domain.UpdatePemasokInput{
		Aktif: boolPtr(false),
	}

	_, err := svc.UpdatePemasok(context.Background(), actor, 1, in, nil)
	if err == nil {
		t.Error("expected error when deactivating without alasan")
	}
	if ve, ok := err.(*domain.ValidationError); ok {
		if _, exists := ve.Fields["alasan"]; !exists {
			t.Errorf("expected alasan field in error")
		}
	} else {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestGetKebutuhan_Success(t *testing.T) {
	bahanRepo := newMockBahanRepo()
	menuRepo := newMockMenuRepo()
	poRepo := newMockPORepo()

	// Setup bahan
	bahan1 := &domain.Bahan{
		ID:            1,
		Nama:          "Beras",
		Kategori:      "karbohidrat",
		Satuan:        "kg",
		GramPerSatuan: 1000,
		MudahRusak:    false,
		Aktif:         true,
	}
	bahanRepo.bahans[1] = bahan1

	bahan2 := &domain.Bahan{
		ID:             2,
		Nama:           "Ayam",
		Kategori:       "protein_hewani",
		Satuan:         "kg",
		GramPerSatuan:  1000,
		MudahRusak:     true,
		SuhuSimpanMaks: float64Ptr(5.0),
		Aktif:          true,
	}
	bahanRepo.bahans[2] = bahan2

	// Setup menu
	menu1 := &domain.Menu{
		ID:          1,
		SPPGID:      1,
		Tanggal:     mustParseTime("2024-01-15"),
		NamaMenu:    "Nasi Ayam",
		TargetPorsi: 100,
		Status:      domain.MenuStatusDisetujui,
	}
	menuRepo.menus[1] = menu1
	menuRepo.items[1] = []domain.MenuBahan{
		{BahanID: 1, GramPerPorsi: 150}, // 150g beras per porsi
		{BahanID: 2, GramPerPorsi: 100}, // 100g ayam per porsi
	}

	svc := New(Deps{
		Bahan:  bahanRepo,
		Menus:  menuRepo,
		PO:     poRepo,
		SPPG:   &mockSPPGProvider{kode: "SPPG001"},
		Audits: &mockAuditRepo{},
		Notifs: &mockNotifRepo{},
	})

	actor := &authdomain.Claims{
		UserID: 1,
		Role:   authdomain.RoleAkuntan,
		SPPGID: int64Ptr(1),
	}

	filter := domain.KebutuhanFilter{
		Dari:   "2024-01-15",
		Sampai: "2024-01-15",
	}

	res, err := svc.GetKebutuhan(context.Background(), actor, filter)
	if err != nil {
		t.Fatalf("GetKebutuhan failed: %v", err)
	}

	if len(res.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(res.Items))
	}

	// Check beras: 150g * 100 porsi = 15000g = 15kg
	berasFound := false
	for _, item := range res.Items {
		if item.BahanID == 1 {
			berasFound = true
			if item.QtyKebutuhan != 15.0 {
				t.Errorf("beras qty: expected 15.0, got %v", item.QtyKebutuhan)
			}
			if item.QtySisa != 15.0 {
				t.Errorf("beras qty_sisa: expected 15.0, got %v", item.QtySisa)
			}
		}
	}
	if !berasFound {
		t.Error("beras not found in result")
	}

	// Check ayam: 100g * 100 porsi = 10000g = 10kg
	ayamFound := false
	for _, item := range res.Items {
		if item.BahanID == 2 {
			ayamFound = true
			if item.QtyKebutuhan != 10.0 {
				t.Errorf("ayam qty: expected 10.0, got %v", item.QtyKebutuhan)
			}
			if item.MudahRusak != true {
				t.Error("ayam should be mudah_rusak")
			}
		}
	}
	if !ayamFound {
		t.Error("ayam not found in result")
	}
}

func TestGetKebutuhan_WithPO(t *testing.T) {
	bahanRepo := newMockBahanRepo()
	menuRepo := newMockMenuRepo()
	poRepo := newMockPORepo()

	// Setup bahan
	bahan1 := &domain.Bahan{
		ID:            1,
		Nama:          "Beras",
		Kategori:      "karbohidrat",
		Satuan:        "kg",
		GramPerSatuan: 1000,
		MudahRusak:    false,
		Aktif:         true,
	}
	bahanRepo.bahans[1] = bahan1

	// Setup menu
	menu1 := &domain.Menu{
		ID:          1,
		SPPGID:      1,
		Tanggal:     mustParseTime("2024-01-15"),
		NamaMenu:    "Nasi",
		TargetPorsi: 100,
		Status:      domain.MenuStatusDisetujui,
	}
	menuRepo.menus[1] = menu1
	menuRepo.items[1] = []domain.MenuBahan{
		{BahanID: 1, GramPerPorsi: 150},
	}

	// PO already ordered 5kg
	poRepo.items[1] = []domain.POItem{
		{POID: 1, BahanID: 1, QtyPesan: 5.0},
	}
	poRepo.menus[1] = []int64{1} // menu 1 linked to PO 1

	svc := New(Deps{
		Bahan:  bahanRepo,
		Menus:  menuRepo,
		PO:     poRepo,
		SPPG:   &mockSPPGProvider{kode: "SPPG001"},
		Audits: &mockAuditRepo{},
		Notifs: &mockNotifRepo{},
	})

	actor := &authdomain.Claims{
		UserID: 1,
		Role:   authdomain.RoleAkuntan,
		SPPGID: int64Ptr(1),
	}

	filter := domain.KebutuhanFilter{
		Dari:   "2024-01-15",
		Sampai: "2024-01-15",
	}

	res, err := svc.GetKebutuhan(context.Background(), actor, filter)
	if err != nil {
		t.Fatalf("GetKebutuhan failed: %v", err)
	}

	// Kebutuhan = 15kg, sudah dipesan = 5kg, sisa = 10kg
	for _, item := range res.Items {
		if item.BahanID == 1 {
			if item.QtyKebutuhan != 15.0 {
				t.Errorf("qty_kebutuhan: expected 15.0, got %v", item.QtyKebutuhan)
			}
			if item.QtySudahDipesan != 5.0 {
				t.Errorf("qty_sudah_dipesan: expected 5.0, got %v", item.QtySudahDipesan)
			}
			if item.QtySisa != 10.0 {
				t.Errorf("qty_sisa: expected 10.0, got %v", item.QtySisa)
			}
		}
	}
}

func int64Ptr(i int64) *int64       { return &i }
func boolPtr(b bool) *bool          { return &b }
func float64Ptr(f float64) *float64 { return &f }

func mustParseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

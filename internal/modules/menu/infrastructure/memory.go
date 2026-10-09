package infrastructure

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
)

// MemoryStores implements menu ports in memory for tests and local dev.
type MemoryStores struct {
	mu              sync.Mutex
	bahan           map[int64]*domain.Bahan
	menus           map[int64]*domain.Menu
	menuItems       map[int64][]domain.MenuBahan
	menuDateIndex   map[string]int64
	nextBahanID     int64
	nextMenuID      int64
	nextMenuBahanID int64
	capacities      map[int64]*int
	sppgExists      map[int64]bool
	recipients      map[int64]int
	audits          []authdomain.AuditEntry
	nextAuditID     int64
}

// NewMemoryStores builds empty stores.
func NewMemoryStores() *MemoryStores {
	return &MemoryStores{
		bahan:           map[int64]*domain.Bahan{},
		menus:           map[int64]*domain.Menu{},
		menuItems:       map[int64][]domain.MenuBahan{},
		menuDateIndex:   map[string]int64{},
		nextBahanID:     1,
		nextMenuID:      1,
		nextMenuBahanID: 1,
		capacities:      map[int64]*int{},
		sppgExists:      map[int64]bool{},
		recipients:      map[int64]int{},
		nextAuditID:     1,
	}
}

// SeedSPPG registers capacity and total recipients for a SPPG.
func (m *MemoryStores) SeedSPPG(id int64, capacity *int, recipients int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sppgExists[id] = true
	m.capacities[id] = capacity
	m.recipients[id] = recipients
}

// SeedBahan inserts a bahan directly (tests).
func (m *MemoryStores) SeedBahan(b domain.Bahan) domain.Bahan {
	m.mu.Lock()
	defer m.mu.Unlock()
	b.ID = m.nextBahanID
	m.nextBahanID++
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	if b.UpdatedAt.IsZero() {
		b.UpdatedAt = b.CreatedAt
	}
	cp := b
	m.bahan[cp.ID] = &cp
	return cp
}

// Create implements BahanRepository.
func (m *MemoryStores) Create(ctx context.Context, b *domain.Bahan) (*domain.Bahan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.bahan {
		if strings.EqualFold(existing.Nama, b.Nama) {
			return nil, domain.ErrBahanExists
		}
	}
	b.ID = m.nextBahanID
	m.nextBahanID++
	cp := *b
	m.bahan[cp.ID] = &cp
	out := cp
	return &out, nil
}

// FindByID implements BahanRepository.
func (m *MemoryStores) FindByID(ctx context.Context, id int64) (*domain.Bahan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.bahan[id]
	if !ok {
		return nil, nil
	}
	cp := *b
	return &cp, nil
}

// FindByNameInsensitive implements BahanRepository.
func (m *MemoryStores) FindByNameInsensitive(ctx context.Context, nama string) (*domain.Bahan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.bahan {
		if strings.EqualFold(b.Nama, nama) {
			cp := *b
			return &cp, nil
		}
	}
	return nil, nil
}

// Update implements BahanRepository.
func (m *MemoryStores) Update(ctx context.Context, b *domain.Bahan) (*domain.Bahan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.bahan[b.ID]
	if !ok {
		return nil, domain.ErrBahanNotFound
	}
	for id, other := range m.bahan {
		if id != b.ID && strings.EqualFold(other.Nama, b.Nama) {
			return nil, domain.ErrBahanExists
		}
	}
	_ = existing
	cp := *b
	m.bahan[b.ID] = &cp
	out := cp
	return &out, nil
}

// List implements BahanRepository with q/kategori/aktif filters.
func (m *MemoryStores) List(ctx context.Context, filter domain.BahanFilter) ([]domain.Bahan, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	filter.Normalize()
	var out []domain.Bahan
	for _, b := range m.bahan {
		if filter.Kategori != nil && b.Kategori != *filter.Kategori {
			continue
		}
		if filter.Aktif != nil && b.Aktif != *filter.Aktif {
			continue
		}
		if filter.Q != nil && *filter.Q != "" && !strings.Contains(strings.ToLower(b.Nama), strings.ToLower(*filter.Q)) {
			continue
		}
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	total := len(out)
	start := filter.Offset()
	if start >= total {
		return []domain.Bahan{}, total, nil
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}
	return out[start:end], total, nil
}

func menuDateKey(sppgID int64, t time.Time) string {
	y, mo, d := t.Date()
	return string(rune(sppgID)) + "|" + time.Date(y, mo, d, 0, 0, 0, 0, time.UTC).Format("2006-01-02") + "|" + itoa(sppgID)
}

func itoa(v int64) string {
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

// CreateMenu implements MenuRepository atomically.
func (m *MemoryStores) CreateMenu(ctx context.Context, menu *domain.Menu, items []domain.MenuBahan) (*domain.Menu, []domain.MenuBahan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := menuDateKey(menu.SPPGID, menu.Tanggal)
	if _, ok := m.menuDateIndex[key]; ok {
		return nil, nil, domain.ErrMenuExists
	}
	menu.ID = m.nextMenuID
	m.nextMenuID++
	cp := *menu
	m.menus[cp.ID] = &cp
	stored := make([]domain.MenuBahan, 0, len(items))
	for _, it := range items {
		it.ID = m.nextMenuBahanID
		m.nextMenuBahanID++
		it.MenuID = cp.ID
		stored = append(stored, it)
	}
	m.menuItems[cp.ID] = stored
	m.menuDateIndex[key] = cp.ID
	out := cp
	outItems := append([]domain.MenuBahan{}, stored...)
	return &out, outItems, nil
}

// ExistsBySPPGDate implements MenuRepository.
func (m *MemoryStores) ExistsBySPPGDate(ctx context.Context, sppgID int64, tanggal time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.menuDateIndex[menuDateKey(sppgID, tanggal)]
	return ok, nil
}

// GetCapacity implements SPPGProvider.
func (m *MemoryStores) GetCapacity(ctx context.Context, sppgID int64) (*int, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.sppgExists[sppgID] {
		// Unknown SPPG is treated as found with no capacity to keep memory
		// fixtures simple; postgres returns found=false.
		return nil, true, nil
	}
	return m.capacities[sppgID], true, nil
}

// SumRecipients implements SekolahProvider.
func (m *MemoryStores) SumRecipients(ctx context.Context, sppgID int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recipients[sppgID], nil
}

// Append implements AuditRepository.
func (m *MemoryStores) Append(ctx context.Context, e *authdomain.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e.ID = m.nextAuditID
	m.nextAuditID++
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	m.audits = append(m.audits, *e)
	return nil
}

// AuditCount returns stored audit rows (tests).
func (m *MemoryStores) AuditCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.audits)
}

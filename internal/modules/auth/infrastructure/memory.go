package infrastructure

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// MemoryStores bundles in-memory ports for tests and local dev without Postgres.
type MemoryStores struct {
	mu            sync.Mutex
	users         map[int64]*domain.User
	emailIndex    map[string]int64
	refresh       map[string]*domain.RefreshToken
	audits        []domain.AuditEntry
	sppg          map[int64]bool
	sekolahOwner  map[int64]int64
	nextUserID    int64
	nextRefreshID int64
	nextAuditID   int64
}

// NewMemoryStores builds empty stores.
func NewMemoryStores() *MemoryStores {
	return &MemoryStores{
		users:         map[int64]*domain.User{},
		emailIndex:    map[string]int64{},
		refresh:       map[string]*domain.RefreshToken{},
		sppg:          map[int64]bool{},
		sekolahOwner:  map[int64]int64{},
		nextUserID:    1,
		nextRefreshID: 1,
		nextAuditID:   1,
	}
}

// SeedSPPG registers an SPPG id as existing.
func (m *MemoryStores) SeedSPPG(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sppg[id] = true
}

// SeedSekolah registers sekolah -> sppg ownership.
func (m *MemoryStores) SeedSekolah(sekolahID, sppgID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sppg[sppgID] = true
	m.sekolahOwner[sekolahID] = sppgID
}

// CreateUser implements UserRepository.
func (m *MemoryStores) CreateUser(ctx context.Context, u *domain.User) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToLower(u.Email)
	if _, ok := m.emailIndex[key]; ok {
		return nil, domain.ErrEmailExists
	}
	u.ID = m.nextUserID
	m.nextUserID++
	cp := *u
	m.users[cp.ID] = &cp
	m.emailIndex[key] = cp.ID
	out := cp
	return &out, nil
}

// FindByEmail implements UserRepository (case-insensitive).
func (m *MemoryStores) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.emailIndex[strings.ToLower(email)]
	if !ok {
		return nil, nil
	}
	cp := *m.users[id]
	return &cp, nil
}

// FindByID implements UserRepository.
func (m *MemoryStores) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	cp := *u
	return &cp, nil
}

// Update implements UserRepository.
func (m *MemoryStores) Update(ctx context.Context, u *domain.User) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.users[u.ID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	// Keep email index consistent (email is immutable in MVP, but guard anyway).
	oldKey := strings.ToLower(existing.Email)
	newKey := strings.ToLower(u.Email)
	if oldKey != newKey {
		if _, taken := m.emailIndex[newKey]; taken {
			return nil, domain.ErrEmailExists
		}
		delete(m.emailIndex, oldKey)
		m.emailIndex[newKey] = u.ID
	}
	cp := *u
	m.users[u.ID] = &cp
	out := cp
	return &out, nil
}

// EmailExists implements UserRepository.
func (m *MemoryStores) EmailExists(ctx context.Context, email string, excludeID *int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.emailIndex[strings.ToLower(email)]
	if !ok {
		return false, nil
	}
	if excludeID != nil && id == *excludeID {
		return false, nil
	}
	return true, nil
}

// StoreRefresh implements RefreshTokenRepository.
func (m *MemoryStores) StoreRefresh(ctx context.Context, t *domain.RefreshToken) (*domain.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t.ID = m.nextRefreshID
	m.nextRefreshID++
	cp := *t
	m.refresh[cp.TokenHash] = &cp
	out := cp
	return &out, nil
}

// FindByHash implements RefreshTokenRepository.
func (m *MemoryStores) FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.refresh[hash]
	if !ok {
		return nil, nil
	}
	cp := *t
	return &cp, nil
}

// Revoke implements RefreshTokenRepository.
func (m *MemoryStores) Revoke(ctx context.Context, hash string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.refresh[hash]
	if !ok {
		return nil
	}
	if t.RevokedAt == nil {
		t.RevokedAt = &now
	}
	return nil
}

// RevokeAllForUser implements RefreshTokenRepository.
func (m *MemoryStores) RevokeAllForUser(ctx context.Context, userID int64, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.refresh {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
		}
	}
	return nil
}

// Append implements AuditRepository.
func (m *MemoryStores) Append(ctx context.Context, e *domain.AuditEntry) error {
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

// List implements AuditRepository with optional SPPG scoping.
func (m *MemoryStores) List(ctx context.Context, filter domain.AuditFilter, scopeSPPGID *int64) ([]domain.AuditEntry, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.AuditEntry
	for _, e := range m.audits {
		if filter.Aksi != nil && e.Aksi != *filter.Aksi {
			continue
		}
		if filter.Tabel != nil && e.Tabel != *filter.Tabel {
			continue
		}
		if filter.UserID != nil && (e.UserID == nil || *e.UserID != *filter.UserID) {
			continue
		}
		if scopeSPPGID != nil {
			if e.UserID == nil {
				continue
			}
			actor, ok := m.users[*e.UserID]
			if !ok || actor.SPPGID == nil || *actor.SPPGID != *scopeSPPGID {
				continue
			}
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	total := len(out)
	start := filter.Offset()
	if start >= total {
		return []domain.AuditEntry{}, total, nil
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}
	return out[start:end], total, nil
}

// Exists implements SPPGChecker.
func (m *MemoryStores) Exists(ctx context.Context, id int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sppg[id], nil
}

// FindOwner implements SekolahChecker.
func (m *MemoryStores) FindOwner(ctx context.Context, sekolahID int64) (bool, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	owner, ok := m.sekolahOwner[sekolahID]
	return ok, owner, nil
}

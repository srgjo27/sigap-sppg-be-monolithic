package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

type PemasokCreateResult struct {
	Pemasok *domain.Pemasok
}

type PemasokUpdateResult struct {
	Pemasok *domain.Pemasok
}

type PemasokDetailResult struct {
	Detail *domain.PemasokDetail
}

type PemasokListResult struct {
	Data []domain.Pemasok
	Meta PageMeta
}

type PageMeta struct {
	Page    int
	PerPage int
	Total   int
}

// CreatePemasok implements POST /pemasok. Writers: admin, kepala_sppg, akuntan.
func (s *Service) CreatePemasok(ctx context.Context, actor *authdomain.Claims, in domain.CreatePemasokInput, ip *string) (*PemasokCreateResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAdmin && actor.Role != authdomain.RoleKepalaSPPG && actor.Role != authdomain.RoleAkuntan {
		return nil, domain.ErrForbidden
	}

	var sppgID int64
	if actor.Role == authdomain.RoleAdmin {
		if in.SPPGID == nil {
			return nil, &domain.ValidationError{Fields: map[string]string{"sppg_id": "required for admin"}}
		}
		sppgID = *in.SPPGID
	} else {
		if actor.SPPGID == nil {
			return nil, domain.ErrForbidden
		}
		sppgID = *actor.SPPGID
	}

	// Validate input
	if err := domain.ValidateCreatePemasok(in); err != nil {
		return nil, err
	}

	// Check duplicate name (case-insensitive)
	normalized, _ := domain.ValidateNoHP(in.NoHP)
	in.NoHP = normalized
	namaTrimmed := strings.TrimSpace(in.Nama)
	existing, err := s.pemasok.FindByNameInsensitive(ctx, sppgID, namaTrimmed)
	if err != nil {
		return nil, fmt.Errorf("check pemasok name: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrPemasokExists
	}

	now := s.clock()
	p := &domain.Pemasok{
		SPPGID:     sppgID,
		Nama:       namaTrimmed,
		JenisUsaha: in.JenisUsaha,
		Kontak:     strings.TrimSpace(in.Kontak),
		NoHP:       in.NoHP,
		Alamat:     strings.TrimSpace(in.Alamat),
		Aktif:      true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	created, err := s.pemasok.Create(ctx, p)
	if err != nil {
		return nil, err
	}

	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditCreate,
		Tabel:     "pemasok",
		RecordID:  &created.ID,
		DataBaru:  strPtr(mustJSON(map[string]any{"nama": created.Nama, "jenis_usaha": created.JenisUsaha, "no_hp": created.NoHP, "aktif": created.Aktif})),
		IPAddress: ip,
		CreatedAt: now,
	})

	return &PemasokCreateResult{Pemasok: created}, nil
}

// UpdatePemasok implements PATCH /pemasok/{id}. Writers: admin, kepala_sppg, akuntan.
func (s *Service) UpdatePemasok(ctx context.Context, actor *authdomain.Claims, id int64, in domain.UpdatePemasokInput, ip *string) (*PemasokUpdateResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAdmin && actor.Role != authdomain.RoleKepalaSPPG && actor.Role != authdomain.RoleAkuntan {
		return nil, domain.ErrForbidden
	}

	if err := domain.ValidateUpdatePemasok(in); err != nil {
		return nil, err
	}

	current, err := s.pemasok.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find pemasok: %w", err)
	}
	if current == nil {
		return nil, domain.ErrPemasokNotFound
	}

	// Check SPPG access
	var sppgID int64
	if actor.Role == authdomain.RoleAdmin {
		if in.SPPGID != nil {
			sppgID = *in.SPPGID
		} else {
			sppgID = current.SPPGID
		}
	} else {
		if actor.SPPGID == nil || current.SPPGID != *actor.SPPGID {
			return nil, domain.ErrPemasokNotFound
		}
		sppgID = current.SPPGID
	}

	oldSnap := map[string]any{
		"nama": current.Nama, "jenis_usaha": current.JenisUsaha, "kontak": current.Kontak,
		"no_hp": current.NoHP, "alamat": current.Alamat, "aktif": current.Aktif,
	}

	if in.Nama != nil {
		trimmed := strings.TrimSpace(*in.Nama)
		if !strings.EqualFold(trimmed, current.Nama) {
			dup, err := s.pemasok.FindByNameInsensitive(ctx, sppgID, trimmed)
			if err != nil {
				return nil, fmt.Errorf("check pemasok name: %w", err)
			}
			if dup != nil && dup.ID != current.ID {
				return nil, domain.ErrPemasokExists
			}
		}
		current.Nama = trimmed
	}
	if in.JenisUsaha != nil {
		current.JenisUsaha = *in.JenisUsaha
	}
	if in.Kontak != nil {
		current.Kontak = strings.TrimSpace(*in.Kontak)
	}
	if in.NoHP != nil {
		normalized, ok := domain.ValidateNoHP(*in.NoHP)
		if !ok {
			return nil, &domain.ValidationError{Fields: map[string]string{"no_hp": "invalid Indonesian phone format"}}
		}
		current.NoHP = normalized
	}
	if in.Alamat != nil {
		current.Alamat = strings.TrimSpace(*in.Alamat)
	}
	if in.Aktif != nil {
		current.Aktif = *in.Aktif
	}

	now := s.clock()
	current.UpdatedAt = now
	updated, err := s.pemasok.Update(ctx, current)
	if err != nil {
		return nil, fmt.Errorf("update pemasok: %w", err)
	}

	newSnap := map[string]any{
		"nama": updated.Nama, "jenis_usaha": updated.JenisUsaha, "kontak": updated.Kontak,
		"no_hp": updated.NoHP, "alamat": updated.Alamat, "aktif": updated.Aktif,
	}

	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditUpdate,
		Tabel:     "pemasok",
		RecordID:  &updated.ID,
		DataLama:  strPtr(mustJSON(oldSnap)),
		DataBaru:  strPtr(mustJSON(newSnap)),
		IPAddress: ip,
		CreatedAt: now,
	})

	return &PemasokUpdateResult{Pemasok: updated}, nil
}

// GetPemasok implements GET /pemasok/{id}. Readers: admin, kepala_sppg, akuntan, ahli_gizi, pengawas.
func (s *Service) GetPemasok(ctx context.Context, actor *authdomain.Claims, id int64) (*PemasokDetailResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	allowed := map[authdomain.Role]bool{
		authdomain.RoleAdmin: true, authdomain.RoleKepalaSPPG: true, authdomain.RoleAkuntan: true,
		authdomain.RoleAhliGizi: true, authdomain.RolePengawas: true,
	}
	if !allowed[actor.Role] {
		return nil, domain.ErrForbidden
	}

	p, err := s.pemasok.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find pemasok: %w", err)
	}
	if p == nil {
		return nil, domain.ErrPemasokNotFound
	}

	// Check SPPG access
	if actor.Role != authdomain.RoleAdmin && actor.Role != authdomain.RolePengawas {
		if actor.SPPGID == nil || p.SPPGID != *actor.SPPGID {
			return nil, domain.ErrPemasokNotFound
		}
	}

	kinerja, err := s.pemasok.GetKinerja(ctx, p.SPPGID, p.ID)
	if err != nil {
		return nil, fmt.Errorf("get kinerja: %w", err)
	}

	detail := &domain.PemasokDetail{
		Pemasok: *p,
		Kinerja: kinerja,
	}

	return &PemasokDetailResult{Detail: detail}, nil
}

// ListPemasok implements GET /pemasok. Readers: admin, kepala_sppg, akuntan, ahli_gizi, pengawas.
func (s *Service) ListPemasok(ctx context.Context, actor *authdomain.Claims, filter domain.PemasokFilter) (*PemasokListResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	allowed := map[authdomain.Role]bool{
		authdomain.RoleAdmin: true, authdomain.RoleKepalaSPPG: true, authdomain.RoleAkuntan: true,
		authdomain.RoleAhliGizi: true, authdomain.RolePengawas: true,
	}
	if !allowed[actor.Role] {
		return nil, domain.ErrForbidden
	}

	var sppgID int64
	if actor.Role == authdomain.RoleAdmin || actor.Role == authdomain.RolePengawas {
		// Admin/pengawas can filter by sppg_id via query, but we don't have that in filter yet
		// For now, admin/pengawas see all (we'll add sppg_id filter later if needed)
		sppgID = 0 // 0 means all
	} else {
		if actor.SPPGID == nil {
			return nil, domain.ErrForbidden
		}
		sppgID = *actor.SPPGID
	}

	filter.Normalize()
	items, total, err := s.pemasok.List(ctx, sppgID, filter)
	if err != nil {
		return nil, fmt.Errorf("list pemasok: %w", err)
	}
	if items == nil {
		items = []domain.Pemasok{}
	}

	return &PemasokListResult{
		Data: items,
		Meta: PageMeta{Page: filter.Page, PerPage: filter.Limit, Total: total},
	}, nil
}

// Helper functions
func strPtr(s string) *string { return &s }

func mustJSON(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

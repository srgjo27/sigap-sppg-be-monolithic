package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
)

// Clock abstracts time for tests.
type Clock func() time.Time

// Service orchestrates MVP-002.1 and MVP-002.2 use cases.
type Service struct {
	bahan   BahanRepository
	menus   MenuRepository
	sppg    SPPGProvider
	sekolah SekolahProvider
	audits  AuditRepository
	clock   Clock
}

// Deps wires the service without a framework.
type Deps struct {
	Bahan   BahanRepository
	Menus   MenuRepository
	SPPG    SPPGProvider
	Sekolah SekolahProvider
	Audits  AuditRepository
	Clock   Clock
}

// New builds a Service with sensible defaults.
func New(d Deps) *Service {
	clock := d.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Service{bahan: d.Bahan, menus: d.Menus, sppg: d.SPPG, sekolah: d.Sekolah, audits: d.Audits, clock: clock}
}

// --- MVP-002.1 ---

// CreateBahan implements POST /bahan. Writers: admin, ahli_gizi.
func (s *Service) CreateBahan(ctx context.Context, actor *authdomain.Claims, in domain.CreateBahanInput, ip *string) (*domain.Bahan, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAdmin && actor.Role != authdomain.RoleAhliGizi {
		return nil, domain.ErrForbidden
	}
	trimmed := domain.Bahan{Nama: strings.TrimSpace(in.Nama)}
	_ = trimmed
	in.Nama = strings.TrimSpace(in.Nama)
	if err := domain.ValidateCreateBahan(in); err != nil {
		return nil, err
	}
	existing, err := s.bahan.FindByNameInsensitive(ctx, in.Nama)
	if err != nil {
		return nil, fmt.Errorf("check bahan name: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrBahanExists
	}
	gram, _ := domain.ResolveGramPerSatuan(in.Satuan, in.GramPerSatuan)
	now := s.clock()
	b := &domain.Bahan{
		Nama:           in.Nama,
		Kategori:       in.Kategori,
		Satuan:         in.Satuan,
		GramPerSatuan:  gram,
		MudahRusak:     in.MudahRusak,
		SuhuSimpanMaks: in.SuhuSimpanMaks,
		Aktif:          true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	created, err := s.bahan.Create(ctx, b)
	if err != nil {
		return nil, fmt.Errorf("create bahan: %w", err)
	}
	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditCreate,
		Tabel:     "bahan",
		RecordID:  &created.ID,
		DataBaru:  strPtr(mustJSON(map[string]any{"nama": created.Nama, "kategori": created.Kategori})),
		IPAddress: ip,
		CreatedAt: now,
	})
	return created, nil
}

// UpdateBahan implements PATCH /bahan/{id}. Writers: admin, ahli_gizi.
func (s *Service) UpdateBahan(ctx context.Context, actor *authdomain.Claims, id int64, in domain.UpdateBahanInput, ip *string) (*domain.Bahan, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAdmin && actor.Role != authdomain.RoleAhliGizi {
		return nil, domain.ErrForbidden
	}
	if err := domain.ValidateUpdateBahan(in); err != nil {
		return nil, err
	}
	current, err := s.bahan.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find bahan: %w", err)
	}
	if current == nil {
		return nil, domain.ErrBahanNotFound
	}
	oldSnap := map[string]any{
		"nama": current.Nama, "kategori": current.Kategori, "satuan": current.Satuan,
		"gram_per_satuan": current.GramPerSatuan, "mudah_rusak": current.MudahRusak,
		"suhu_simpan_maks": current.SuhuSimpanMaks, "aktif": current.Aktif,
	}
	if in.Nama != nil {
		trimmed := strings.TrimSpace(*in.Nama)
		if !strings.EqualFold(trimmed, current.Nama) {
			dup, err := s.bahan.FindByNameInsensitive(ctx, trimmed)
			if err != nil {
				return nil, fmt.Errorf("check bahan name: %w", err)
			}
			if dup != nil && dup.ID != current.ID {
				return nil, domain.ErrBahanExists
			}
		}
		current.Nama = trimmed
	}
	if in.Kategori != nil {
		current.Kategori = *in.Kategori
	}
	if in.Satuan != nil {
		current.Satuan = *in.Satuan
	}
	if in.GramPerSatuan != nil {
		current.GramPerSatuan = *in.GramPerSatuan
	}
	if in.MudahRusak != nil {
		current.MudahRusak = *in.MudahRusak
	}
	if in.ClearSuhu {
		current.SuhuSimpanMaks = nil
	} else if in.SuhuSimpanMaks != nil {
		v := *in.SuhuSimpanMaks
		current.SuhuSimpanMaks = &v
	}
	if in.Aktif != nil {
		current.Aktif = *in.Aktif
	}
	// Enforce cross-field rule on the merged entity.
	if current.MudahRusak && current.SuhuSimpanMaks == nil {
		return nil, &domain.ValidationError{Fields: map[string]string{"suhu_simpan_maks": "required when mudah_rusak is true"}}
	}
	if current.GramPerSatuan <= 0 {
		return nil, &domain.ValidationError{Fields: map[string]string{"gram_per_satuan": "must be greater than 0"}}
	}
	now := s.clock()
	current.UpdatedAt = now
	updated, err := s.bahan.Update(ctx, current)
	if err != nil {
		return nil, fmt.Errorf("update bahan: %w", err)
	}
	newSnap := map[string]any{
		"nama": updated.Nama, "kategori": updated.Kategori, "satuan": updated.Satuan,
		"gram_per_satuan": updated.GramPerSatuan, "mudah_rusak": updated.MudahRusak,
		"suhu_simpan_maks": updated.SuhuSimpanMaks, "aktif": updated.Aktif,
	}
	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditUpdate,
		Tabel:     "bahan",
		RecordID:  &updated.ID,
		DataLama:  strPtr(mustJSON(oldSnap)),
		DataBaru:  strPtr(mustJSON(newSnap)),
		IPAddress: ip,
		CreatedAt: now,
	})
	return updated, nil
}

// ListBahan implements GET /bahan. Readers: every role except pic_sekolah.
func (s *Service) ListBahan(ctx context.Context, actor *authdomain.Claims, filter domain.BahanFilter) ([]domain.Bahan, int, error) {
	if actor == nil {
		return nil, 0, domain.ErrUnauthorized
	}
	if actor.Role == authdomain.RolePICsekolah {
		return nil, 0, domain.ErrForbidden
	}
	filter.Normalize()
	if filter.Kategori != nil && !domain.IsValidKategori(*filter.Kategori) {
		return nil, 0, &domain.ValidationError{Fields: map[string]string{"kategori": "unknown kategori"}}
	}
	items, total, err := s.bahan.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list bahan: %w", err)
	}
	if items == nil {
		items = []domain.Bahan{}
	}
	return items, total, nil
}

// --- MVP-002.2 ---

// CreateMenuResult carries the created menu, its items, and warnings.
type CreateMenuResult struct {
	Menu     *domain.Menu
	Items    []domain.MenuBahan
	Warnings []string
}

// CreateMenu implements POST /menus. Actor must be ahli_gizi; sppg from token.
func (s *Service) CreateMenu(ctx context.Context, actor *authdomain.Claims, in domain.CreateMenuInput, ip *string) (*CreateMenuResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAhliGizi {
		return nil, domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return nil, domain.ErrForbidden
	}
	now := s.clock()
	if err := domain.ValidateCreateMenu(in, now); err != nil {
		return nil, err
	}
	exists, err := s.menus.ExistsBySPPGDate(ctx, *actor.SPPGID, in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("check menu date: %w", err)
	}
	if exists {
		return nil, domain.ErrMenuExists
	}
	// Resolve every bahan_id: must exist and be active.
	resolved := make([]domain.Bahan, 0, len(in.Items))
	for _, it := range in.Items {
		b, err := s.bahan.FindByID(ctx, it.BahanID)
		if err != nil {
			return nil, fmt.Errorf("find bahan: %w", err)
		}
		if b == nil {
			return nil, domain.ErrBahanNotFound
		}
		if !b.Aktif {
			return nil, domain.ErrBahanInactive
		}
		resolved = append(resolved, *b)
	}
	// Default target_porsi from total recipients of the SPPG's schools.
	target := 0
	if in.TargetPorsi != nil {
		target = *in.TargetPorsi
	} else {
		sum, err := s.sekolah.SumRecipients(ctx, *actor.SPPGID)
		if err != nil {
			return nil, fmt.Errorf("sum recipients: %w", err)
		}
		if sum <= 0 {
			return nil, &domain.ValidationError{Fields: map[string]string{"target_porsi": "no recipients found for sppg"}}
		}
		target = sum
	}
	var warnings []string
	if capPtr, found, err := s.sppg.GetCapacity(ctx, *actor.SPPGID); err != nil {
		return nil, fmt.Errorf("get sppg capacity: %w", err)
	} else if found && capPtr != nil && target > *capPtr {
		warnings = append(warnings, fmt.Sprintf("target_porsi %d exceeds sppg capacity %d", target, *capPtr))
	}
	menu := &domain.Menu{
		SPPGID:       *actor.SPPGID,
		Tanggal:      in.Tanggal,
		NamaMenu:     strings.TrimSpace(in.NamaMenu),
		EnergiKkal:   in.EnergiKkal,
		ProteinG:     in.ProteinG,
		KarbohidratG: in.KarbohidratG,
		LemakG:       in.LemakG,
		TargetPorsi:  target,
		Catatan:      in.Catatan,
		DibuatOleh:   actor.UserID,
		Status:       domain.MenuStatusDraf,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	items := make([]domain.MenuBahan, 0, len(in.Items))
	for i, it := range in.Items {
		items = append(items, domain.MenuBahan{
			BahanID:      it.BahanID,
			BahanNama:    resolved[i].Nama,
			GramPerPorsi: it.GramPerPorsi,
		})
	}
	created, createdItems, err := s.menus.CreateMenu(ctx, menu, items)
	if err != nil {
		return nil, fmt.Errorf("create menu: %w", err)
	}
	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditCreate,
		Tabel:     "menu",
		RecordID:  &created.ID,
		DataBaru:  strPtr(mustJSON(map[string]any{"tanggal": created.Tanggal.Format("2006-01-02"), "nama_menu": created.NamaMenu})),
		IPAddress: ip,
		CreatedAt: now,
	})
	if warnings == nil {
		warnings = []string{}
	}
	return &CreateMenuResult{Menu: created, Items: createdItems, Warnings: warnings}, nil
}

func strPtr(s string) *string { return &s }

func mustJSON(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

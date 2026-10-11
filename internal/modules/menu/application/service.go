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

// Service orchestrates MVP-002.1 through MVP-002.6 use cases.
type Service struct {
	bahan   BahanRepository
	menus   MenuRepository
	sppg    SPPGProvider
	sekolah SekolahProvider
	audits  AuditRepository
	notifs  NotificationRepository
	clock   Clock
}

// Deps wires the service without a framework.
type Deps struct {
	Bahan   BahanRepository
	Menus   MenuRepository
	SPPG    SPPGProvider
	Sekolah SekolahProvider
	Audits  AuditRepository
	Notifs  NotificationRepository
	Clock   Clock
}

// New builds a Service with sensible defaults.
func New(d Deps) *Service {
	clock := d.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Service{bahan: d.Bahan, menus: d.Menus, sppg: d.SPPG, sekolah: d.Sekolah, audits: d.Audits, notifs: d.Notifs, clock: clock}
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

// --- MVP-002.3 ---

// MenuListResult carries list rows plus empty-date markers.
type MenuListResult struct {
	Items   []domain.Menu
	Total   int
	Missing []string
}

// ListMenus implements GET /menus. Every authenticated role may list;
// scoping follows the MVP-002 matrix.
func (s *Service) ListMenus(ctx context.Context, actor *authdomain.Claims, filter domain.MenuFilter) (*MenuListResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if err := domain.ValidateMenuFilter(filter); err != nil {
		return nil, err
	}
	var scope *int64
	forceApproved := false
	switch actor.Role {
	case authdomain.RoleAdmin, authdomain.RolePengawas:
		scope = filter.SPPGID
	case authdomain.RolePICsekolah:
		if actor.SekolahID == nil {
			return nil, domain.ErrForbidden
		}
		found, owner, err := s.sekolah.FindOwner(ctx, *actor.SekolahID)
		if err != nil {
			return nil, fmt.Errorf("find sekolah owner: %w", err)
		}
		if !found {
			return nil, domain.ErrNotFound
		}
		if filter.SPPGID != nil && *filter.SPPGID != owner {
			return nil, domain.ErrNotFound
		}
		scope = &owner
		forceApproved = true
	default:
		if actor.SPPGID == nil {
			return nil, domain.ErrForbidden
		}
		if filter.SPPGID != nil && *filter.SPPGID != *actor.SPPGID {
			return nil, domain.ErrNotFound
		}
		scope = actor.SPPGID
	}
	status := filter.Status
	if forceApproved {
		if status != nil && *status != domain.MenuStatusDisetujui {
			// pic_sekolah never sees non-approved menus: empty result.
			missing := domain.MissingDates(filter.Dari, filter.Sampai, map[string]bool{})
			return &MenuListResult{Items: []domain.Menu{}, Missing: missing}, nil
		}
		approved := domain.MenuStatusDisetujui
		status = &approved
	}
	items, err := s.menus.ListByScope(ctx, scope, filter.Dari, filter.Sampai, status)
	if err != nil {
		return nil, fmt.Errorf("list menus: %w", err)
	}
	if actor.Role == authdomain.RolePICsekolah {
		kept := items[:0]
		for _, m := range items {
			if m.Status == domain.MenuStatusDisetujui {
				kept = append(kept, m)
			}
		}
		items = kept
	}
	if items == nil {
		items = []domain.Menu{}
	}
	var missing []string
	if scope != nil {
		present := map[string]bool{}
		for _, m := range items {
			present[m.Tanggal.Format("2006-01-02")] = true
		}
		missing = domain.MissingDates(filter.Dari, filter.Sampai, present)
	} else {
		missing = []string{}
	}
	return &MenuListResult{Items: items, Total: len(items), Missing: missing}, nil
}

// MenuDetail carries a menu with its composition.
type MenuDetail struct {
	Menu     *domain.Menu
	Items    []domain.MenuBahan
	Redacted bool
}

// GetMenu implements GET /menus/{id}.
func (s *Service) GetMenu(ctx context.Context, actor *authdomain.Claims, id int64) (*MenuDetail, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	m, err := s.menus.FindMenuByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find menu: %w", err)
	}
	if m == nil {
		return nil, domain.ErrMenuNotFound
	}
	redacted := false
	switch actor.Role {
	case authdomain.RoleAdmin, authdomain.RolePengawas:
		// Cross-SPPG read allowed.
	case authdomain.RolePICsekolah:
		if actor.SekolahID == nil {
			return nil, domain.ErrNotFound
		}
		found, owner, err := s.sekolah.FindOwner(ctx, *actor.SekolahID)
		if err != nil {
			return nil, fmt.Errorf("find sekolah owner: %w", err)
		}
		if !found || m.SPPGID != owner || m.Status != domain.MenuStatusDisetujui {
			return nil, domain.ErrMenuNotFound
		}
		redacted = true
	default:
		if actor.SPPGID == nil || m.SPPGID != *actor.SPPGID {
			return nil, domain.ErrMenuNotFound
		}
	}
	got, err := s.menus.ListItems(ctx, []int64{m.ID})
	if err != nil {
		return nil, fmt.Errorf("list menu items: %w", err)
	}
	items := got[m.ID]
	if items == nil {
		items = []domain.MenuBahan{}
	}
	if redacted {
		for i := range items {
			items[i].GramPerPorsi = 0
		}
	}
	return &MenuDetail{Menu: m, Items: items, Redacted: redacted}, nil
}

// --- MVP-002.4 ---

// UpdateMenuResult carries the updated menu and capacity warnings.
type UpdateMenuResult struct {
	Menu     *domain.Menu
	Items    []domain.MenuBahan
	Warnings []string
}

// UpdateMenu implements PATCH /menus/{id}. Only ahli_gizi on own-SPPG drafts.
func (s *Service) UpdateMenu(ctx context.Context, actor *authdomain.Claims, id int64, in domain.UpdateMenuInput, ip *string) (*UpdateMenuResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAhliGizi {
		return nil, domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return nil, domain.ErrForbidden
	}
	if err := domain.ValidateUpdateMenu(in); err != nil {
		return nil, err
	}
	current, err := s.menus.FindMenuByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find menu: %w", err)
	}
	if current == nil || current.SPPGID != *actor.SPPGID {
		return nil, domain.ErrMenuNotFound
	}
	if current.Status != domain.MenuStatusDraf {
		return nil, domain.ErrMenuApproved
	}
	used, err := s.menus.IsMenuUsed(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("check menu usage: %w", err)
	}
	if used {
		return nil, domain.ErrMenuInUse
	}
	oldSnap := menuSnapshot(current)
	if in.NamaMenu != nil {
		current.NamaMenu = strings.TrimSpace(*in.NamaMenu)
	}
	if in.EnergiKkal != nil {
		current.EnergiKkal = *in.EnergiKkal
	}
	if in.ProteinG != nil {
		current.ProteinG = *in.ProteinG
	}
	if in.KarbohidratG != nil {
		current.KarbohidratG = *in.KarbohidratG
	}
	if in.LemakG != nil {
		current.LemakG = *in.LemakG
	}
	if in.TargetPorsi != nil {
		current.TargetPorsi = *in.TargetPorsi
	}
	if in.ClearCatatan {
		current.Catatan = nil
	} else if in.Catatan != nil {
		v := *in.Catatan
		current.Catatan = &v
	}
	var replace *[]domain.MenuBahan
	if in.Items != nil {
		resolved := make([]domain.Bahan, 0, len(*in.Items))
		for _, it := range *in.Items {
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
		next := make([]domain.MenuBahan, 0, len(*in.Items))
		for i, it := range *in.Items {
			next = append(next, domain.MenuBahan{MenuID: current.ID, BahanID: it.BahanID, BahanNama: resolved[i].Nama, GramPerPorsi: it.GramPerPorsi})
		}
		replace = &next
	}
	var warnings []string
	if capPtr, found, err := s.sppg.GetCapacity(ctx, *actor.SPPGID); err != nil {
		return nil, fmt.Errorf("get sppg capacity: %w", err)
	} else if found && capPtr != nil && current.TargetPorsi > *capPtr {
		warnings = append(warnings, fmt.Sprintf("target_porsi %d exceeds sppg capacity %d", current.TargetPorsi, *capPtr))
	}
	now := s.clock()
	current.UpdatedAt = now
	updated, stored, err := s.menus.UpdateMenu(ctx, current, replace)
	if err != nil {
		return nil, fmt.Errorf("update menu: %w", err)
	}
	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditUpdate,
		Tabel:     "menu",
		RecordID:  &updated.ID,
		DataLama:  strPtr(mustJSON(oldSnap)),
		DataBaru:  strPtr(mustJSON(menuSnapshot(updated))),
		IPAddress: ip,
		CreatedAt: now,
	})
	if warnings == nil {
		warnings = []string{}
	}
	return &UpdateMenuResult{Menu: updated, Items: stored, Warnings: warnings}, nil
}

// DeleteMenu implements DELETE /menus/{id}. Only ahli_gizi on own-SPPG drafts.
func (s *Service) DeleteMenu(ctx context.Context, actor *authdomain.Claims, id int64, ip *string) error {
	if actor == nil {
		return domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAhliGizi {
		return domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return domain.ErrForbidden
	}
	current, err := s.menus.FindMenuByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find menu: %w", err)
	}
	if current == nil || current.SPPGID != *actor.SPPGID {
		return domain.ErrMenuNotFound
	}
	if current.Status != domain.MenuStatusDraf {
		return domain.ErrMenuApproved
	}
	used, err := s.menus.IsMenuUsed(ctx, id)
	if err != nil {
		return fmt.Errorf("check menu usage: %w", err)
	}
	if used {
		return domain.ErrMenuInUse
	}
	if err := s.menus.DeleteMenu(ctx, id); err != nil {
		return fmt.Errorf("delete menu: %w", err)
	}
	now := s.clock()
	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditUpdate,
		Tabel:     "menu",
		RecordID:  &id,
		DataLama:  strPtr(mustJSON(menuSnapshot(current))),
		DataBaru:  strPtr(mustJSON(map[string]any{"deleted": true})),
		IPAddress: ip,
		CreatedAt: now,
	})
	return nil
}

func menuSnapshot(m *domain.Menu) map[string]any {
	return map[string]any{
		"tanggal": m.Tanggal.Format("2006-01-02"), "nama_menu": m.NamaMenu,
		"energi_kkal": m.EnergiKkal, "protein_g": m.ProteinG,
		"karbohidrat_g": m.KarbohidratG, "lemak_g": m.LemakG,
		"target_porsi": m.TargetPorsi, "status": m.Status,
	}
}

// --- MVP-002.7 ---

// CopyMenuResult carries copied menus with their items plus warnings.
type CopyMenuResult struct {
	Menus    []*domain.Menu
	Items    [][]domain.MenuBahan
	Warnings []string
}

// CopyMenu implements POST /menus/{id}/copy. Actor: ahli_gizi, own SPPG.
// The source may be draf or disetujui; every copy is a fresh draf with the
// current user as author and target_porsi recalculated from today's
// recipients. Inactive bahan are skipped and reported. All copies persist
// in one transaction.
func (s *Service) CopyMenu(ctx context.Context, actor *authdomain.Claims, sourceID int64, in domain.CopyMenuInput, ip *string) (*CopyMenuResult, error) {
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
	if err := domain.ValidateCopyMenuInput(in, now); err != nil {
		return nil, err
	}
	source, err := s.menus.FindMenuByID(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("find menu: %w", err)
	}
	if source == nil || source.SPPGID != *actor.SPPGID {
		return nil, domain.ErrMenuNotFound
	}
	got, err := s.menus.ListItems(ctx, []int64{source.ID})
	if err != nil {
		return nil, fmt.Errorf("list menu items: %w", err)
	}
	srcItems := got[source.ID]
	// Every target date must be free; the first conflict rejects everything.
	for _, t := range in.TanggalTujuan {
		exists, err := s.menus.ExistsBySPPGDate(ctx, *actor.SPPGID, t)
		if err != nil {
			return nil, fmt.Errorf("check menu date: %w", err)
		}
		if exists {
			return nil, &domain.MenuDateConflictError{Tanggal: t.Format("2006-01-02")}
		}
	}
	// Resolve source composition; inactive bahan are skipped with warnings.
	type activeItem struct {
		bahan domain.Bahan
		gram  float64
	}
	var active []activeItem
	var warnings []string
	for _, it := range srcItems {
		b, err := s.bahan.FindByID(ctx, it.BahanID)
		if err != nil {
			return nil, fmt.Errorf("find bahan: %w", err)
		}
		if b == nil {
			return nil, domain.ErrBahanNotFound
		}
		if !b.Aktif {
			warnings = append(warnings, fmt.Sprintf("bahan %q tidak disalin karena nonaktif", b.Nama))
			continue
		}
		active = append(active, activeItem{bahan: *b, gram: it.GramPerPorsi})
	}
	sum, err := s.sekolah.SumRecipients(ctx, *actor.SPPGID)
	if err != nil {
		return nil, fmt.Errorf("sum recipients: %w", err)
	}
	if sum <= 0 {
		return nil, &domain.ValidationError{Fields: map[string]string{"target_porsi": "no recipients found for sppg"}}
	}
	if capPtr, found, err := s.sppg.GetCapacity(ctx, *actor.SPPGID); err != nil {
		return nil, fmt.Errorf("get sppg capacity: %w", err)
	} else if found && capPtr != nil && sum > *capPtr {
		warnings = append(warnings, fmt.Sprintf("target_porsi %d exceeds sppg capacity %d", sum, *capPtr))
	}
	menus := make([]*domain.Menu, 0, len(in.TanggalTujuan))
	allItems := make([][]domain.MenuBahan, 0, len(in.TanggalTujuan))
	for _, t := range in.TanggalTujuan {
		var catatan *string
		if source.Catatan != nil {
			v := *source.Catatan
			catatan = &v
		}
		menus = append(menus, &domain.Menu{
			SPPGID:       *actor.SPPGID,
			Tanggal:      t,
			NamaMenu:     source.NamaMenu,
			EnergiKkal:   source.EnergiKkal,
			ProteinG:     source.ProteinG,
			KarbohidratG: source.KarbohidratG,
			LemakG:       source.LemakG,
			TargetPorsi:  sum,
			Catatan:      catatan,
			DibuatOleh:   actor.UserID,
			Status:       domain.MenuStatusDraf,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		var items []domain.MenuBahan
		for _, a := range active {
			items = append(items, domain.MenuBahan{BahanID: a.bahan.ID, BahanNama: a.bahan.Nama, GramPerPorsi: a.gram})
		}
		allItems = append(allItems, items)
	}
	createdMenus, createdItems, err := s.menus.CreateMenus(ctx, menus, allItems)
	if err != nil {
		return nil, fmt.Errorf("copy menu: %w", err)
	}
	for _, cm := range createdMenus {
		_ = s.audits.Append(ctx, &authdomain.AuditEntry{
			UserID:    &actor.UserID,
			Aksi:      authdomain.AuditCreate,
			Tabel:     "menu",
			RecordID:  &cm.ID,
			DataBaru:  strPtr(mustJSON(map[string]any{"tanggal": cm.Tanggal.Format("2006-01-02"), "nama_menu": cm.NamaMenu, "copied_from": sourceID})),
			IPAddress: ip,
			CreatedAt: now,
		})
	}
	if warnings == nil {
		warnings = []string{}
	}
	return &CopyMenuResult{Menus: createdMenus, Items: createdItems, Warnings: warnings}, nil
}

// --- MVP-002.5 ---

// ApprovalResult carries a menu with its composition after status change.
type ApprovalResult struct {
	Menu  *domain.Menu
	Items []domain.MenuBahan
}

// approvalDetail loads items for an approval response (best-effort: empty on error).
func (s *Service) approvalDetail(ctx context.Context, m *domain.Menu) []domain.MenuBahan {
	got, err := s.menus.ListItems(ctx, []int64{m.ID})
	if err != nil {
		return []domain.MenuBahan{}
	}
	items := got[m.ID]
	if items == nil {
		items = []domain.MenuBahan{}
	}
	return items
}

// ApproveMenu implements POST /menus/{id}/approve. Actor: kepala_sppg, own SPPG.
func (s *Service) ApproveMenu(ctx context.Context, actor *authdomain.Claims, id int64, ip *string) (*ApprovalResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleKepalaSPPG {
		return nil, domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return nil, domain.ErrForbidden
	}
	current, err := s.menus.FindMenuByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find menu: %w", err)
	}
	if current == nil || current.SPPGID != *actor.SPPGID {
		return nil, domain.ErrMenuNotFound
	}
	if current.Status != domain.MenuStatusDraf {
		return nil, domain.ErrStatusConflict
	}
	items := s.approvalDetail(ctx, current)
	if !domain.MenuComplete(current, items) {
		return nil, domain.ErrMenuIncomplete
	}
	now := s.clock()
	approver := actor.UserID
	updated, err := s.menus.SetApproval(ctx, id, domain.MenuStatusDisetujui, &approver, &now)
	if err != nil {
		return nil, fmt.Errorf("approve menu: %w", err)
	}
	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditUpdate,
		Tabel:     "menu",
		RecordID:  &updated.ID,
		DataLama:  strPtr(mustJSON(map[string]any{"status": domain.MenuStatusDraf})),
		DataBaru:  strPtr(mustJSON(map[string]any{"status": domain.MenuStatusDisetujui, "disetujui_oleh": approver})),
		IPAddress: ip,
		CreatedAt: now,
	})
	if s.notifs != nil {
		_ = s.notifs.NotifySPPGRoles(ctx, *actor.SPPGID,
			[]string{string(authdomain.RoleAkuntan), string(authdomain.RoleAhliGizi)},
			"menu_disetujui", "Menu disetujui",
			fmt.Sprintf("Menu %s tanggal %s siap dipesan", updated.NamaMenu, updated.Tanggal.Format("2006-01-02")),
			"menu", updated.ID)
	}
	return &ApprovalResult{Menu: updated, Items: s.approvalDetail(ctx, updated)}, nil
}

// RevertMenu implements POST /menus/{id}/revert. Actor: kepala_sppg, own SPPG.
func (s *Service) RevertMenu(ctx context.Context, actor *authdomain.Claims, id int64, alasan string, ip *string) (*ApprovalResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleKepalaSPPG {
		return nil, domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return nil, domain.ErrForbidden
	}
	if err := domain.ValidateRevertReason(alasan); err != nil {
		return nil, err
	}
	current, err := s.menus.FindMenuByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find menu: %w", err)
	}
	if current == nil || current.SPPGID != *actor.SPPGID {
		return nil, domain.ErrMenuNotFound
	}
	if current.Status != domain.MenuStatusDisetujui {
		return nil, domain.ErrStatusConflict
	}
	used, err := s.menus.IsMenuUsed(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("check menu usage: %w", err)
	}
	if used {
		return nil, domain.ErrMenuInUse
	}
	now := s.clock()
	updated, err := s.menus.SetApproval(ctx, id, domain.MenuStatusDraf, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("revert menu: %w", err)
	}
	trimmed := strings.TrimSpace(alasan)
	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditUpdate,
		Tabel:     "menu",
		RecordID:  &updated.ID,
		DataLama:  strPtr(mustJSON(map[string]any{"status": domain.MenuStatusDisetujui})),
		DataBaru:  strPtr(mustJSON(map[string]any{"status": domain.MenuStatusDraf, "alasan": trimmed})),
		IPAddress: ip,
		CreatedAt: now,
	})
	return &ApprovalResult{Menu: updated, Items: s.approvalDetail(ctx, updated)}, nil
}

// --- MVP-002.6 ---

// KebutuhanItem is one computed row in GET kebutuhan-bahan.
type KebutuhanItem struct {
	BahanID      int64
	Nama         string
	Kategori     string
	Satuan       string
	GramPerPorsi float64
	TotalGram    float64
	Qty          float64
	MudahRusak   bool
}

// KebutuhanResult carries the read-only simulation output.
type KebutuhanResult struct {
	Menu           *domain.Menu
	TargetPorsi    int
	CadanganPersen float64
	Items          []KebutuhanItem
}

// GetKebutuhan implements GET /menus/{id}/kebutuhan-bahan.
// Actors: ahli_gizi, akuntan, kepala_sppg on their own SPPG. Read-only.
func (s *Service) GetKebutuhan(ctx context.Context, actor *authdomain.Claims, id int64, target *int, cadangan *float64) (*KebutuhanResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAhliGizi && actor.Role != authdomain.RoleAkuntan && actor.Role != authdomain.RoleKepalaSPPG {
		return nil, domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return nil, domain.ErrForbidden
	}
	if err := domain.ValidateKebutuhanParams(target, cadangan); err != nil {
		return nil, err
	}
	m, err := s.menus.FindMenuByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find menu: %w", err)
	}
	if m == nil || m.SPPGID != *actor.SPPGID {
		return nil, domain.ErrMenuNotFound
	}
	effTarget := m.TargetPorsi
	if target != nil {
		effTarget = *target
	}
	effCadangan := 0.0
	if cadangan != nil {
		effCadangan = *cadangan
	}
	got, err := s.menus.ListItems(ctx, []int64{m.ID})
	if err != nil {
		return nil, fmt.Errorf("list menu items: %w", err)
	}
	rows := got[m.ID]
	items := make([]KebutuhanItem, 0, len(rows))
	for _, it := range rows {
		b, err := s.bahan.FindByID(ctx, it.BahanID)
		if err != nil {
			return nil, fmt.Errorf("find bahan: %w", err)
		}
		if b == nil {
			return nil, domain.ErrBahanNotFound
		}
		total, qty := domain.ComputeKebutuhan(it.GramPerPorsi, b.Satuan, b.GramPerSatuan, effTarget, effCadangan)
		items = append(items, KebutuhanItem{
			BahanID: it.BahanID, Nama: b.Nama, Kategori: b.Kategori, Satuan: b.Satuan,
			GramPerPorsi: it.GramPerPorsi, TotalGram: total, Qty: qty, MudahRusak: b.MudahRusak,
		})
	}
	return &KebutuhanResult{Menu: m, TargetPorsi: effTarget, CadanganPersen: effCadangan, Items: items}, nil
}

func strPtr(s string) *string { return &s }

func mustJSON(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

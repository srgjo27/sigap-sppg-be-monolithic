package application

import (
	"context"
	"fmt"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

type KebutuhanResult struct {
	Meta  domain.KebutuhanMeta
	Items []domain.KebutuhanItem
}

// GetKebutuhan implements GET /pengadaan/kebutuhan.
// Actors: akuntan, ahli_gizi, kepala_sppg on their own SPPG. Read-only.
func (s *Service) GetKebutuhan(ctx context.Context, actor *authdomain.Claims, filter domain.KebutuhanFilter) (*KebutuhanResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAkuntan && actor.Role != authdomain.RoleAhliGizi && actor.Role != authdomain.RoleKepalaSPPG {
		return nil, domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return nil, domain.ErrForbidden
	}

	if err := domain.ValidateKebutuhanFilter(filter); err != nil {
		return nil, err
	}

	dari, _ := time.Parse("2006-01-02", filter.Dari)
	sampai, _ := time.Parse("2006-01-02", filter.Sampai)
	cadangan := filter.CadanganPersen
	if cadangan == 0 {
		cadangan = 0
	}

	// Get approved menus in range
	approvedStatus := domain.MenuStatusDisetujui
	menus, err := s.menus.ListByScope(ctx, actor.SPPGID, dari, sampai, &approvedStatus)
	if err != nil {
		return nil, fmt.Errorf("list approved menus: %w", err)
	}

	// Get draft menus in range for meta
	draftStatus := domain.MenuStatusDraf
	draftMenus, err := s.menus.ListByScope(ctx, actor.SPPGID, dari, sampai, &draftStatus)
	if err != nil {
		return nil, fmt.Errorf("list draft menus: %w", err)
	}

	menuIDs := make([]int64, 0, len(menus))
	for _, m := range menus {
		menuIDs = append(menuIDs, m.ID)
	}

	// Get menu compositions
	menuItems, err := s.menus.ListItems(ctx, menuIDs)
	if err != nil {
		return nil, fmt.Errorf("list menu items: %w", err)
	}

	// Aggregate bahan quantities per menu
	type bahanAgg struct {
		bahan        *domain.Bahan
		totalGram    float64
		qtyKebutuhan float64
		rincian      []domain.KebutuhanRincian
		firstDate    time.Time
	}
	aggMap := make(map[int64]*bahanAgg)

	for _, menu := range menus {
		items := menuItems[menu.ID]
		for _, mi := range items {
			b, err := s.bahan.FindByID(ctx, mi.BahanID)
			if err != nil {
				return nil, fmt.Errorf("find bahan: %w", err)
			}
			if b == nil || !b.Aktif {
				continue
			}
			if filter.Kategori != nil && b.Kategori != *filter.Kategori {
				continue
			}

			totalGram, qty := domain.ComputeKebutuhan(mi.GramPerPorsi, b.Satuan, b.GramPerSatuan, menu.TargetPorsi, cadangan)
			qty = domain.NormalizeSatuanQty(b.Satuan, qty)

			ba, ok := aggMap[mi.BahanID]
			if !ok {
				ba = &bahanAgg{bahan: b, firstDate: menu.Tanggal}
				aggMap[mi.BahanID] = ba
			}
			if menu.Tanggal.Before(ba.firstDate) {
				ba.firstDate = menu.Tanggal
			}
			ba.totalGram += totalGram
			ba.qtyKebutuhan += qty
			ba.rincian = append(ba.rincian, domain.KebutuhanRincian{
				Tanggal: menu.Tanggal.Format("2006-01-02"),
				MenuID:  menu.ID,
				Qty:     domain.NormalizeSatuanQty(b.Satuan, qty),
			})
		}
	}

	// Get PO items for these menus via po_menu
	poMenus, err := s.po.ListPOMenus(ctx, menuIDs)
	if err != nil {
		return nil, fmt.Errorf("list po menus: %w", err)
	}

	// Get POs that are linked to these menus (status != batal)
	poIDs := make([]int64, 0)
	poMenuMap := make(map[int64][]int64) // menu_id -> po_ids
	for poID, mIDs := range poMenus {
		for _, mID := range mIDs {
			poMenuMap[mID] = append(poMenuMap[mID], poID)
		}
		poIDs = append(poIDs, poID)
	}

	// Fetch PO items for these POs (status != batal)
	poItemsMap, err := s.po.ListItemsByPOIDs(ctx, poIDs)
	if err != nil {
		return nil, fmt.Errorf("list po items: %w", err)
	}

	// Aggregate qty_sudah_dipesan per bahan
	qtyDipesan := make(map[int64]float64)
	for _, items := range poItemsMap {
		for _, item := range items {
			qtyDipesan[item.BahanID] += item.QtyPesan
		}
	}

	// Build PO lintas rentang meta
	poLintas := make([]domain.POLintasRentang, 0)
	// We need to check which POs are linked to menus outside the range
	// This would require additional queries; for now return empty

	// Build result items
	resultItems := make([]domain.KebutuhanItem, 0, len(aggMap))
	for _, ba := range aggMap {
		already := qtyDipesan[ba.bahan.ID]
		sisa := ba.qtyKebutuhan - already
		if sisa < 0 {
			sisa = 0
		}
		resultItems = append(resultItems, domain.KebutuhanItem{
			BahanID:             ba.bahan.ID,
			Nama:                ba.bahan.Nama,
			Kategori:            ba.bahan.Kategori,
			Satuan:              ba.bahan.Satuan,
			MudahRusak:          ba.bahan.MudahRusak,
			TanggalPakaiPertama: ba.firstDate.Format("2006-01-02"),
			TotalGram:           ba.totalGram,
			QtyKebutuhan:        ba.qtyKebutuhan,
			QtySudahDipesan:     already,
			QtySisa:             sisa,
			Rincian:             ba.rincian,
		})
	}

	// Build meta
	menuBelum := make([]domain.MenuBelumDisetujui, 0, len(draftMenus))
	for _, m := range draftMenus {
		menuBelum = append(menuBelum, domain.MenuBelumDisetujui{
			Tanggal:  m.Tanggal.Format("2006-01-02"),
			MenuID:   m.ID,
			NamaMenu: m.NamaMenu,
		})
	}

	meta := domain.KebutuhanMeta{
		Dari:               filter.Dari,
		Sampai:             filter.Sampai,
		CadanganPersen:     cadangan,
		JumlahMenu:         len(menus),
		MenuBelumDisetujui: menuBelum,
		POLintasRentang:    poLintas,
	}

	return &KebutuhanResult{Meta: meta, Items: resultItems}, nil
}

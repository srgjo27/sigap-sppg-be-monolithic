package application

import (
	"context"
	"fmt"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// POCreateResult carries the created PO, its items, and warnings.
type POCreateResult struct {
	PO       *domain.PurchaseOrder
	Items    []domain.POItem
	Warnings []string
}

// POListResult carries the list of POs and pagination meta.
type POListResult struct {
	Data []domain.POListItem
	Meta PageMeta
}

// PODetailResult carries the detailed PO view.
type PODetailResult struct {
	Detail *domain.PODetail
}

// CreatePurchaseOrder implements POST /purchase-orders. Actor must be akuntan; sppg from token.
func (s *Service) CreatePurchaseOrder(ctx context.Context, actor *authdomain.Claims, in domain.CreatePOInput, ip *string) (*POCreateResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAkuntan {
		return nil, domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return nil, domain.ErrForbidden
	}

	now := s.clock()
	today := startOfDay(now)

	// Validate input
	if err := domain.ValidateCreatePOInput(in, today); err != nil {
		return nil, err
	}

	// Parse tgl_kirim_diminta
	tglKirimDiminta, err := time.Parse("2006-01-02", in.TglKirimDiminta)
	if err != nil {
		return nil, &domain.ValidationError{Fields: map[string]string{"tgl_kirim_diminta": "use YYYY-MM-DD"}}
	}

	// Check pemasok exists, active, and belongs to same SPPG
	if err := s.checkPemasokForPO(ctx, *actor.SPPGID, in.PemasokID); err != nil {
		return nil, err
	}

	// Check siklus if provided
	if in.SiklusID != nil {
		if err := s.checkSiklusForPO(ctx, *actor.SPPGID, *in.SiklusID); err != nil {
			return nil, err
		}
	}

	// Check menus if provided
	var menuIDs []int64
	if len(in.MenuIDs) > 0 {
		if err := s.checkMenusForPO(ctx, *actor.SPPGID, in.MenuIDs); err != nil {
			return nil, err
		}
		menuIDs = in.MenuIDs
	}

	// Check bahan for each item and build POItems
	poItems, err := s.validateBuildPOItems(ctx, in.Items)
	if err != nil {
		return nil, err
	}

	// Generate PO number
	sppgKode, err := s.sppg.GetKode(ctx, *actor.SPPGID)
	if err != nil {
		return nil, fmt.Errorf("get sppg kode: %w", err)
	}
	nomorPO, err := s.generatePONumber(ctx, *actor.SPPGID, sppgKode, tglKirimDiminta)
	if err != nil {
		return nil, fmt.Errorf("generate PO number: %w", err)
	}

	// Create PO
	po := &domain.PurchaseOrder{
		SPPGID:          *actor.SPPGID,
		NomorPO:         nomorPO,
		PemasokID:       in.PemasokID,
		SiklusID:        in.SiklusID,
		TglPO:           today,
		TglKirimDiminta: &tglKirimDiminta,
		Status:          domain.POStatusDraf,
		DibuatOleh:      actor.UserID,
		Catatan:         &in.Catatan,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// Generate warnings
	warnings := s.generateWarnings(ctx, *actor.SPPGID, po, poItems, menuIDs, bahanItems(in.Items))

	// Save PO, items, and menu links in transaction
	createdPO, createdItems, err := s.po.Create(ctx, po, poItems, menuIDs)
	if err != nil {
		return nil, fmt.Errorf("create purchase order: %w", err)
	}

	// Audit log
	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditCreate,
		Tabel:     "purchase_order",
		RecordID:  &createdPO.ID,
		DataBaru:  strPtr(mustJSON(map[string]any{"nomor_po": createdPO.NomorPO, "pemasok_id": createdPO.PemasokID, "status": createdPO.Status})),
		IPAddress: ip,
		CreatedAt: now,
	})

	return &POCreateResult{PO: createdPO, Items: createdItems, Warnings: warnings}, nil
}

// generatePONumber generates PO number: PO/{kode_sppg}/{YYYYMM}/{urut 4 digit}.
// The transactional reservation (ReserveNumberSeq) guarantees that numbers of
// deleted POs are never reused and concurrent issuances serialize.
func (s *Service) generatePONumber(ctx context.Context, sppgID int64, sppgKode string, tglKirimDiminta time.Time) (string, error) {
	yearMonth := tglKirimDiminta.Format("200601")
	seq, err := s.po.ReserveNumberSeq(ctx, sppgID, yearMonth)
	if err != nil {
		return "", fmt.Errorf("generate PO number: %w", err)
	}
	return fmt.Sprintf("PO/%s/%s/%04d", sppgKode, yearMonth, seq), nil
}

// bahanItems creates a map of bahan_id -> bahan for warnings
func bahanItems(items []domain.POItemInput) map[int64]domain.POItemInput {
	m := make(map[int64]domain.POItemInput)
	for _, item := range items {
		m[item.BahanID] = item
	}
	return m
}

// generateWarnings generates warnings for PO creation.
func (s *Service) generateWarnings(ctx context.Context, sppgID int64, po *domain.PurchaseOrder, items []domain.POItem, menuIDs []int64, _ map[int64]domain.POItemInput) []string {
	var warnings []string

	// Warning 1: qty_pesan exceeds qty_sisa from kebutuhan by more than 20% for linked menus
	if len(menuIDs) > 0 {
		kebutuhanFilter := domain.KebutuhanFilter{
			Dari:   po.TglKirimDiminta.Format("2006-01-02"),
			Sampai: po.TglKirimDiminta.Format("2006-01-02"),
		}
		// We need to get kebutuhan for the menus. For now, we'll check each menu's kebutuhan.
		// This is a simplified version - in reality we'd compute kebutuhan per menu.
		_ = kebutuhanFilter
	}

	// Warning 2: bahan mudah_rusak with tgl_kirim_diminta more than 1 day before first menu date
	if po.TglKirimDiminta != nil {
		for _, item := range items {
			bahan, err := s.bahan.FindByID(ctx, item.BahanID)
			if err != nil || bahan == nil {
				continue
			}
			if bahan.MudahRusak {
				// Find first menu date that uses this bahan
				firstMenuDate, err := s.findFirstMenuDateForBahan(ctx, sppgID, item.BahanID, menuIDs)
				if err == nil && firstMenuDate != nil {
					daysDiff := firstMenuDate.Sub(*po.TglKirimDiminta).Hours() / 24
					if daysDiff > 1 {
						warnings = append(warnings, fmt.Sprintf("Bahan %s mudah rusak tetapi tgl_kirim_diminta %d hari sebelum tanggal menu pertama", bahan.Nama, int(daysDiff)))
					}
				}
			}
		}
	}

	// Warning 3: bahan dipesan tetapi tidak dipakai di menu yang ditautkan
	if len(menuIDs) > 0 {
		for _, item := range items {
			used := false
			for _, mID := range menuIDs {
				menuItems, err := s.menus.ListItems(ctx, []int64{mID})
				if err != nil {
					continue
				}
				for _, mi := range menuItems[mID] {
					if mi.BahanID == item.BahanID {
						used = true
						break
					}
				}
				if used {
					break
				}
			}
			if !used {
				bahan, _ := s.bahan.FindByID(ctx, item.BahanID)
				nama := "unknown"
				if bahan != nil {
					nama = bahan.Nama
				}
				warnings = append(warnings, fmt.Sprintf("Bahan %s dipesan tetapi tidak dipakai di menu yang ditautkan", nama))
			}
		}
	}

	return warnings
}

// findFirstMenuDateForBahan finds the earliest menu date that uses a bahan among the given menu IDs.
func (s *Service) findFirstMenuDateForBahan(ctx context.Context, _ int64, bahanID int64, menuIDs []int64) (*time.Time, error) {
	if len(menuIDs) == 0 {
		return nil, nil
	}
	var firstDate *time.Time
	for _, mID := range menuIDs {
		menu, err := s.menus.FindByID(ctx, mID)
		if err != nil || menu == nil {
			continue
		}
		items, err := s.menus.ListItems(ctx, []int64{mID})
		if err != nil {
			continue
		}
		for _, mi := range items[mID] {
			if mi.BahanID == bahanID {
				if firstDate == nil || menu.Tanggal.Before(*firstDate) {
					firstDate = &menu.Tanggal
				}
				break
			}
		}
	}
	return firstDate, nil
}

// ListPurchaseOrders implements GET /purchase-orders.
func (s *Service) ListPurchaseOrders(ctx context.Context, actor *authdomain.Claims, filter domain.POFilter) (*POListResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}

	// Validate filter
	if err := domain.ValidatePOFilter(filter); err != nil {
		return nil, err
	}

	var sppgID int64
	if actor.Role == authdomain.RoleAdmin || actor.Role == authdomain.RolePengawas {
		if filter.SPPGID != nil {
			sppgID = *filter.SPPGID
		} else {
			// Admin/pengawas without filter sees all - but we need a sppgID for the query
			// For now, we'll return empty if no sppgID is provided
			sppgID = 0
		}
	} else {
		if actor.SPPGID == nil {
			return nil, domain.ErrForbidden
		}
		sppgID = *actor.SPPGID
	}

	// For petugas_dapur, only show PO with status dikirim, diterima_sebagian, selesai
	if actor.Role == authdomain.RolePetugasDapur {
		allowedStatuses := []string{domain.POStatusDikirim, domain.POStatusDiterimaSebagian, domain.POStatusSelesai}
		filter.Status = allowedStatuses
	}

	filter.Normalize()
	items, total, err := s.po.List(ctx, sppgID, filter)
	if err != nil {
		return nil, fmt.Errorf("list purchase orders: %w", err)
	}
	if items == nil {
		items = []domain.POListItem{}
	}

	// Add terlambat flag
	now := s.clock()
	today := startOfDay(now)
	for i := range items {
		if (items[i].Status == domain.POStatusDikirim || items[i].Status == domain.POStatusDiterimaSebagian) &&
			items[i].TglKirimDiminta != nil && items[i].TglKirimDiminta.Before(today) {
			items[i].Terlambat = true
		}
	}

	return &POListResult{
		Data: items,
		Meta: PageMeta{Page: filter.Page, PerPage: filter.Limit, Total: total},
	}, nil
}

// GetPurchaseOrder implements GET /purchase-orders/{id}.
func (s *Service) GetPurchaseOrder(ctx context.Context, actor *authdomain.Claims, id int64) (*PODetailResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}

	detail, err := s.po.GetDetail(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get po detail: %w", err)
	}
	if detail == nil {
		return nil, domain.ErrPONotFound
	}

	// Check SPPG access
	if actor.Role != authdomain.RoleAdmin && actor.Role != authdomain.RolePengawas {
		if actor.SPPGID == nil || detail.SPPGID != *actor.SPPGID {
			return nil, domain.ErrPONotFound
		}
	}

	// For petugas_dapur, only allow status dikirim, diterima_sebagian, selesai
	if actor.Role == authdomain.RolePetugasDapur {
		if detail.Status == domain.POStatusDraf || detail.Status == domain.POStatusBatal {
			return nil, domain.ErrPONotFound
		}
		// Redact price fields for petugas_dapur
		for i := range detail.Items {
			detail.Items[i].HargaSatuan = 0
			detail.Items[i].Subtotal = 0
			detail.Items[i].NilaiPotongNota = 0
			detail.Items[i].NilaiTagihan = 0
		}
		detail.RekapNilai.TotalPesanan = 0
		detail.RekapNilai.TotalNilaiDikirim = 0
		detail.RekapNilai.TotalPotongNota = 0
		detail.RekapNilai.TotalTagihan = 0
	}

	return &PODetailResult{Detail: detail}, nil
}

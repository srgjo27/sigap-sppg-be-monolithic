package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// POUpdateResult carries the updated PO detail and warnings.
type POUpdateResult struct {
	Detail   *domain.PODetail
	Warnings []string
}

// POTransitionResult carries the PO detail after a status transition
// ItemTidakTerpenuhi is filled only for tutup.
type POTransitionResult struct {
	Detail             *domain.PODetail
	ItemTidakTerpenuhi []domain.PODetailItem
}

// requireAkuntan enforces the single-writer role of PO use cases.
func requireAkuntan(actor *authdomain.Claims) error {
	if actor == nil {
		return domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAkuntan {
		return domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return domain.ErrForbidden
	}
	return nil
}

// requireCanceller allows akuntan and kepala_sppg for batal.
func requireCanceller(actor *authdomain.Claims) error {
	if actor == nil {
		return domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAkuntan && actor.Role != authdomain.RoleKepalaSPPG {
		return domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return domain.ErrForbidden
	}
	return nil
}

// loadOwnedPO fetches a PO and hides POs from other SPPGs (404 semantics).
func (s *Service) loadOwnedPO(ctx context.Context, actor *authdomain.Claims, id int64) (*domain.PurchaseOrder, error) {
	po, err := s.po.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find po: %w", err)
	}
	if po == nil || po.SPPGID != *actor.SPPGID {
		return nil, domain.ErrPONotFound
	}
	return po, nil
}

// checkPemasokForPO validates pemasok exists, same SPPG, and active
// currentID excludes self when changing pemasok.
func (s *Service) checkPemasokForPO(ctx context.Context, sppgID, pemasokID int64) error {
	pemasok, err := s.pemasok.FindByID(ctx, pemasokID)
	if err != nil {
		return fmt.Errorf("find pemasok: %w", err)
	}
	if pemasok == nil || pemasok.SPPGID != sppgID {
		return domain.ErrPemasokNotFound
	}
	if !pemasok.Aktif {
		return domain.ErrPemasokInactive
	}
	return nil
}

// checkSiklusForPO validates siklus belongs to the SPPG and is not closed.
func (s *Service) checkSiklusForPO(ctx context.Context, sppgID, siklusID int64) error {
	siklus, err := s.siklus.FindByID(ctx, siklusID)
	if err != nil {
		return fmt.Errorf("find siklus: %w", err)
	}
	if siklus == nil || siklus.SPPGID != sppgID {
		return domain.ErrSiklusNotFound
	}
	if siklus.Status == domain.SiklusStatusDitutup {
		return domain.ErrSiklusClosed
	}
	return nil
}

// checkMenusForPO validates every menu exists, same SPPG, and approved.
func (s *Service) checkMenusForPO(ctx context.Context, sppgID int64, menuIDs []int64) error {
	if len(menuIDs) == 0 {
		return nil
	}
	menus, err := s.menus.FindByIDs(ctx, menuIDs)
	if err != nil {
		return fmt.Errorf("find menus: %w", err)
	}
	if len(menus) != len(menuIDs) {
		return domain.ErrMenuNotFound
	}
	for _, m := range menus {
		if m == nil || m.SPPGID != sppgID {
			return domain.ErrMenuNotFound
		}
		if m.Status != domain.MenuStatusDisetujui {
			return domain.ErrMenuNotApproved
		}
	}
	return nil
}

// validateBuildPOItems checks every bahan exists, is active, and the quantity
// matches the satuan rule; returns the plain POItems.
func (s *Service) validateBuildPOItems(ctx context.Context, items []domain.POItemInput) ([]domain.POItem, error) {
	poItems := make([]domain.POItem, 0, len(items))
	for _, item := range items {
		bahan, err := s.bahan.FindByID(ctx, item.BahanID)
		if err != nil {
			return nil, fmt.Errorf("find bahan: %w", err)
		}
		if bahan == nil {
			return nil, domain.ErrBahanNotFound
		}
		if !bahan.Aktif {
			return nil, domain.ErrBahanInactive
		}
		if err := domain.ValidateSatuanForQty(bahan.Satuan, item.QtyPesan); err != nil {
			return nil, err
		}
		poItems = append(poItems, domain.POItem{
			BahanID:     item.BahanID,
			QtyPesan:    item.QtyPesan,
			HargaSatuan: item.HargaSatuan,
		})
	}
	return poItems, nil
}

func dateStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// startOfDay returns the start of t's calendar day in its own location.
// Unlike Truncate(24h), this is correct for zones with partial-hour offsets.
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// poHeaderSnapshot builds the audit payload for a PO.
func poHeaderSnapshot(po *domain.PurchaseOrder, jumlahItem int) map[string]any {
	snap := map[string]any{
		"nomor_po":           po.NomorPO,
		"pemasok_id":         po.PemasokID,
		"tgl_kirim_diminta":  dateStr(po.TglKirimDiminta),
		"status":             po.Status,
		"jumlah_item":        jumlahItem,
		"catatan":            derefOrEmpty(po.Catatan),
		"jumlah_menu_linked": 0,
	}
	if po.SiklusID != nil {
		snap["siklus_id"] = *po.SiklusID
	}
	return snap
}

// poAudit appends a best-effort audit entry for purchase_order changes.
func (s *Service) poAudit(ctx context.Context, actor *authdomain.Claims, ip *string, now time.Time, poID int64, aksi string, lama, baru map[string]any) {
	entry := &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      aksi,
		Tabel:     "purchase_order",
		RecordID:  &poID,
		IPAddress: ip,
		CreatedAt: now,
	}
	if lama != nil {
		entry.DataLama = strPtr(mustJSON(lama))
	}
	if baru != nil {
		entry.DataBaru = strPtr(mustJSON(baru))
	}
	_ = s.audits.Append(ctx, entry)
}

// UpdatePurchaseOrder implements PATCH /purchase-orders/{id}.
// Only akuntan may edit, only POs of own SPPG in status draf.
func (s *Service) UpdatePurchaseOrder(ctx context.Context, actor *authdomain.Claims, id int64, in domain.UpdatePOInput, ip *string) (*POUpdateResult, error) {
	if err := requireAkuntan(actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateUpdatePOInput(in); err != nil {
		return nil, err
	}
	sppgID := *actor.SPPGID

	now := s.clock()
	today := startOfDay(now)

	po, err := s.loadOwnedPO(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if po.Status != domain.POStatusDraf {
		return nil, domain.ErrPOInvalidStatus
	}

	// Merge present fields over current values, then reuse rules.
	currentItemsRes, err := s.po.ListItemsByPOIDs(ctx, []int64{id})
	if err != nil {
		return nil, fmt.Errorf("list po items: %w", err)
	}
	currentItems := currentItemsRes[id]
	currentMenuIDs, err := s.po.ListMenuIDsForPO(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list po menus: %w", err)
	}

	merged := domain.CreatePOInput{
		PemasokID:       po.PemasokID,
		TglKirimDiminta: dateStr(po.TglKirimDiminta),
		SiklusID:        po.SiklusID,
		MenuIDs:         currentMenuIDs,
		Catatan:         derefOrEmpty(po.Catatan),
		Items:           poItemsToInputs(currentItems),
	}
	if in.PemasokID != nil {
		merged.PemasokID = *in.PemasokID
	}
	if in.TglKirimDiminta != nil {
		merged.TglKirimDiminta = strings.TrimSpace(*in.TglKirimDiminta)
	}
	if in.SiklusIDSet {
		merged.SiklusID = in.SiklusID
	}
	if in.MenuIDsSet {
		merged.MenuIDs = append([]int64(nil), in.MenuIDs...)
	}
	if in.Catatan != nil {
		merged.Catatan = strings.TrimSpace(*in.Catatan)
	}
	if in.ItemsSet {
		merged.Items = append([]domain.POItemInput(nil), in.Items...)
	}

	if err := domain.ValidateCreatePOInput(merged, today); err != nil {
		return nil, err
	}

	// Reference checks only for the parts the caller changed.
	if in.PemasokID != nil {
		if err := s.checkPemasokForPO(ctx, sppgID, merged.PemasokID); err != nil {
			return nil, err
		}
	}
	if in.SiklusIDSet && merged.SiklusID != nil {
		if err := s.checkSiklusForPO(ctx, sppgID, *merged.SiklusID); err != nil {
			return nil, err
		}
	}
	if in.MenuIDsSet {
		if err := s.checkMenusForPO(ctx, sppgID, merged.MenuIDs); err != nil {
			return nil, err
		}
	}

	var newItems []domain.POItem
	warnItems := currentItems
	if in.ItemsSet {
		newItems, err = s.validateBuildPOItems(ctx, merged.Items)
		if err != nil {
			return nil, err
		}
		warnItems = newItems
	}

	// Recompute warnings with the merged shape.
	warningsPO := *po
	if tgl, err := time.Parse("2006-01-02", merged.TglKirimDiminta); err == nil {
		warningsPO.TglKirimDiminta = &tgl
	}
	warnings := s.generateWarnings(ctx, sppgID, &warningsPO, warnItems, merged.MenuIDs, bahanItems(merged.Items))

	var parsedTgl *time.Time
	if merged.TglKirimDiminta != "" {
		t, err := time.Parse("2006-01-02", merged.TglKirimDiminta)
		if err != nil {
			return nil, &domain.ValidationError{Fields: map[string]string{"tgl_kirim_diminta": "use YYYY-MM-DD"}}
		}
		parsedTgl = &t
	}
	poUpdate := &domain.PurchaseOrder{
		ID:              id,
		PemasokID:       merged.PemasokID,
		SiklusID:        merged.SiklusID,
		TglKirimDiminta: parsedTgl,
		Catatan:         &merged.Catatan,
	}

	updatedPO, _, err := s.po.Update(ctx, poUpdate, newItems, in.ItemsSet, merged.MenuIDs, in.MenuIDsSet)
	if err != nil {
		return nil, fmt.Errorf("update purchase order: %w", err)
	}

	// Snapshot the new item count against the state the caller posted.
	newCount := len(currentItems)
	if in.ItemsSet {
		newCount = len(merged.Items)
	}

	s.poAudit(ctx, actor, ip, now, id, authdomain.AuditUpdate,
		poHeaderSnapshot(po, len(currentItems)),
		poHeaderSnapshot(updatedPO, newCount))

	detail, err := s.po.GetDetail(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get po detail: %w", err)
	}
	if detail == nil {
		return nil, domain.ErrPONotFound
	}
	if warnings == nil {
		warnings = []string{}
	}
	return &POUpdateResult{Detail: detail, Warnings: warnings}, nil
}

// poItemsToInputs converts stored items back into input shape.
func poItemsToInputs(items []domain.POItem) []domain.POItemInput {
	inputs := make([]domain.POItemInput, 0, len(items))
	for _, it := range items {
		inputs = append(inputs, domain.POItemInput{
			BahanID:     it.BahanID,
			QtyPesan:    it.QtyPesan,
			HargaSatuan: it.HargaSatuan,
		})
	}
	return inputs
}

// DeletePurchaseOrder implements DELETE /purchase-orders/{id}.
func (s *Service) DeletePurchaseOrder(ctx context.Context, actor *authdomain.Claims, id int64, ip *string) error {
	if err := requireAkuntan(actor); err != nil {
		return err
	}
	now := s.clock()

	po, err := s.loadOwnedPO(ctx, actor, id)
	if err != nil {
		return err
	}
	if po.Status != domain.POStatusDraf {
		return domain.ErrPOInvalidStatus
	}

	currentItems, err := s.po.ListItemsByPOIDs(ctx, []int64{id})
	if err != nil {
		return fmt.Errorf("list po items: %w", err)
	}
	snap := poHeaderSnapshot(po, len(currentItems))

	if err := s.po.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete purchase order: %w", err)
	}

	s.poAudit(ctx, actor, ip, now, id, "delete", snap, nil)
	return nil
}

// KirimPurchaseOrder implements POST /purchase-orders/{id}/kirim.
func (s *Service) KirimPurchaseOrder(ctx context.Context, actor *authdomain.Claims, id int64, ip *string) (*POTransitionResult, error) {
	if err := requireAkuntan(actor); err != nil {
		return nil, err
	}
	now := s.clock()

	po, err := s.loadOwnedPO(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if po.Status != domain.POStatusDraf {
		return nil, domain.ErrPOInvalidStatus
	}

	// Kirim requires at least one item.
	poItemsRes, err := s.po.ListItemsByPOIDs(ctx, []int64{id})
	if err != nil {
		return nil, fmt.Errorf("list po items: %w", err)
	}
	if len(poItemsRes[id]) == 0 {
		return nil, domain.ErrPOCannotBeSent
	}

	// Pemasok must still be active.
	pemasok, err := s.pemasok.FindByID(ctx, po.PemasokID)
	if err != nil {
		return nil, fmt.Errorf("find pemasok: %w", err)
	}
	if pemasok == nil || pemasok.SPPGID != *actor.SPPGID {
		return nil, domain.ErrPemasokNotFound
	}
	if !pemasok.Aktif {
		return nil, domain.ErrPemasokInactive
	}

	// Delivery date must not have passed.
	today := startOfDay(now)
	if po.TglKirimDiminta == nil || po.TglKirimDiminta.Before(today) {
		return nil, domain.ErrPOCannotBeSent
	}

	updatedPO, err := s.po.SetDikirim(ctx, id, actor.UserID, now)
	if err != nil {
		return nil, fmt.Errorf("send purchase order: %w", err)
	}

	s.poAudit(ctx, actor, ip, now, id, authdomain.AuditUpdate,
		map[string]any{"status": domain.POStatusDraf},
		map[string]any{"status": updatedPO.Status, "dikirim_oleh": actor.UserID})

	detail, err := s.po.GetDetail(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get po detail: %w", err)
	}
	if detail == nil {
		return nil, domain.ErrPONotFound
	}

	// Notify petugas_dapur and kepala_sppg (best effort).
	if s.notifs != nil {
		namaBahan := make([]string, 0, len(detail.Items))
		for _, it := range detail.Items {
			namaBahan = append(namaBahan, fmt.Sprintf("%s %.1f %s", it.BahanNama, it.QtyPesan, it.BahanSatuan))
		}
		_ = s.notifs.NotifySPPGRoles(ctx, po.SPPGID,
			[]string{string(authdomain.RolePetugasDapur), string(authdomain.RoleKepalaSPPG)},
			"po_dikirim", "Purchase order dikirim",
			fmt.Sprintf("PO %s ke pemasok %s, tanggal kirim %s. Bahan: %s",
				po.NomorPO, detail.PemasokNama, dateStr(po.TglKirimDiminta), strings.Join(namaBahan, ", ")),
			"purchase_order", po.ID)
	}

	return &POTransitionResult{Detail: detail}, nil
}

// BatalPurchaseOrder implements POST /purchase-orders/{id}/batal.
func (s *Service) BatalPurchaseOrder(ctx context.Context, actor *authdomain.Claims, id int64, alasan string, ip *string) (*POTransitionResult, error) {
	if err := requireCanceller(actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateAlasan("alasan", alasan, 5, 500); err != nil {
		return nil, err
	}
	now := s.clock()

	po, err := s.loadOwnedPO(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if po.Status != domain.POStatusDraf && po.Status != domain.POStatusDikirim {
		return nil, domain.ErrPOInvalidStatus
	}
	fromStatus := po.Status

	hasReceipt, err := s.po.HasPenerimaan(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("check penerimaan: %w", err)
	}
	if hasReceipt {
		return nil, domain.ErrPOHasReceipt
	}

	trimmed := strings.TrimSpace(alasan)
	updatedPO, err := s.po.SetStatus(ctx, id, po.Status, domain.POStatusBatal, actor.UserID, &trimmed, now)
	if err != nil {
		return nil, fmt.Errorf("cancel purchase order: %w", err)
	}

	s.poAudit(ctx, actor, ip, now, id, authdomain.AuditUpdate,
		map[string]any{"status": po.Status},
		map[string]any{"status": updatedPO.Status, "alasan_tutup": trimmed})

	// After a sent PO is cancelled, petugas_dapur must know.
	if fromStatus == domain.POStatusDikirim && s.notifs != nil {
		_ = s.notifs.NotifySPPGRoles(ctx, po.SPPGID,
			[]string{string(authdomain.RolePetugasDapur)},
			"po_batal", "Purchase order dibatalkan",
			fmt.Sprintf("PO %s dibatalkan. Alasan: %s", po.NomorPO, trimmed),
			"purchase_order", po.ID)
	}

	detail, err := s.po.GetDetail(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get po detail: %w", err)
	}
	if detail == nil {
		return nil, domain.ErrPONotFound
	}
	return &POTransitionResult{Detail: detail}, nil
}

// TutupPurchaseOrder implements POST /purchase-orders/{id}/tutup.
func (s *Service) TutupPurchaseOrder(ctx context.Context, actor *authdomain.Claims, id int64, alasan string, ip *string) (*POTransitionResult, error) {
	if err := requireAkuntan(actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateAlasan("alasan", alasan, 5, 500); err != nil {
		return nil, err
	}
	now := s.clock()

	po, err := s.loadOwnedPO(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if po.Status != domain.POStatusDiterimaSebagian {
		return nil, domain.ErrPOInvalidStatus
	}

	trimmed := strings.TrimSpace(alasan)
	updatedPO, err := s.po.SetStatus(ctx, id, po.Status, domain.POStatusSelesai, actor.UserID, &trimmed, now)
	if err != nil {
		return nil, fmt.Errorf("close purchase order: %w", err)
	}

	s.poAudit(ctx, actor, ip, now, id, authdomain.AuditUpdate,
		map[string]any{"status": po.Status},
		map[string]any{"status": updatedPO.Status, "alasan_tutup": trimmed})

	detail, err := s.po.GetDetail(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get po detail: %w", err)
	}
	if detail == nil {
		return nil, domain.ErrPONotFound
	}

	// Report items that will never be fulfilled.
	var tidakTerpenuhi []domain.PODetailItem
	for _, it := range detail.Items {
		if !it.Terpenuhi {
			tidakTerpenuhi = append(tidakTerpenuhi, it)
		}
	}
	if tidakTerpenuhi == nil {
		tidakTerpenuhi = []domain.PODetailItem{}
	}
	return &POTransitionResult{Detail: detail, ItemTidakTerpenuhi: tidakTerpenuhi}, nil
}

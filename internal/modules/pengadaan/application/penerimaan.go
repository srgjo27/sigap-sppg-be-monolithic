package application

import (
	"context"
	"fmt"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// requirePenerimaanActor allows akuntan and petugas_dapur to record penerimaan.
func requirePenerimaanActor(actor *authdomain.Claims) error {
	if actor == nil {
		return domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleAkuntan && actor.Role != authdomain.RolePetugasDapur {
		return domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return domain.ErrForbidden
	}
	return nil
}

// PenerimaanCreateResult carries the created receipt lots and the updated PO.
type PenerimaanCreateResult struct {
	Lots     []*domain.PenerimaanBahan
	PO       *domain.PurchaseOrder
	Warnings []string
}

// PenerimaanListResult carries the filtered list and pagination meta.
type PenerimaanListResult struct {
	Data []domain.PenerimaanListItem
	Meta PageMeta
}

// PenerimaanDetailResult carries one receipt lot detail.
type PenerimaanDetailResult struct {
	Detail *domain.PenerimaanDetail
}

// CreatePenerimaan implements POST /purchase-orders/{id}/penerimaan (MVP-003.8).
// Actor must be akuntan or petugas_dapur; sppg from token.
// Idempotency: clientRef (Idempotency-Key) -> penerimaan_bahan.client_ref.
func (s *Service) CreatePenerimaan(ctx context.Context, actor *authdomain.Claims, poID int64, in domain.CreatePenerimaanInput, ip *string) (*PenerimaanCreateResult, error) {
	if err := requirePenerimaanActor(actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateCreatePenerimaanInput(in); err != nil {
		return nil, err
	}
	sppgID := *actor.SPPGID
	now := s.clock()
	today := startOfDay(now)

	// Load PO and check ownership + status.
	po, err := s.po.FindByID(ctx, poID)
	if err != nil {
		return nil, fmt.Errorf("find po: %w", err)
	}
	if po == nil || po.SPPGID != sppgID {
		return nil, domain.ErrPONotFound
	}
	if po.Status != domain.POStatusDikirim && po.Status != domain.POStatusDiterimaSebagian {
		return nil, domain.ErrPOInvalidStatus
	}

	// Get PO items to validate po_item_id ownership and get qty_pesan / harga_satuan.
	itemsRes, err := s.po.ListItemsByPOIDs(ctx, []int64{poID})
	if err != nil {
		return nil, fmt.Errorf("list po items: %w", err)
	}
	poItems := itemsRes[poID]
	poItemMap := make(map[int64]domain.POItem)
	for _, it := range poItems {
		poItemMap[it.ID] = it
	}

	// Get current rekap for tolerance check (existing qty_diterima per po_item_id).
	poItemIDs := make([]int64, 0, len(in.Items))
	for _, it := range in.Items {
		poItemIDs = append(poItemIDs, it.POItemID)
	}
	rekap, err := s.po.GetPOItemRekap(ctx, poItemIDs)
	if err != nil {
		return nil, fmt.Errorf("get po item rekap: %w", err)
	}

	type validatedItem struct {
		item             domain.PenerimaanItem
		poItem           domain.POItem
		bahan            domain.Bahan
		existingDiterima float64
		lolosSuhu        *bool
		nilaiPotongNota  float64
		warnings         []string
	}
	validated := make([]validatedItem, 0, len(in.Items))

	for i, it := range in.Items {
		poItem, ok := poItemMap[it.POItemID]
		if !ok {
			return nil, domain.ErrItemNotFound
		}

		bahan, err := s.bahan.FindByID(ctx, poItem.BahanID)
		if err != nil {
			return nil, fmt.Errorf("find bahan: %w", err)
		}
		if bahan == nil {
			return nil, domain.ErrBahanNotFound
		}
		if !bahan.Aktif {
			return nil, domain.ErrBahanInactive
		}

		// Integer check for butir/pcs.
		if bahan.Satuan == "butir" || bahan.Satuan == "pcs" {
			if it.QtyDiterima != float64(int64(it.QtyDiterima)) || it.QtyDitolak != float64(int64(it.QtyDitolak)) {
				return nil, &domain.ValidationError{Fields: map[string]string{"items": "qty must be integer for satuan " + bahan.Satuan}}
			}
		}

		existing := rekap[it.POItemID]
		existingDiterima := existing.QtyDiterima

		// Temperature check for mudah_rusak.
		var lolosSuhu *bool
		var suhuWarnings []string
		if bahan.MudahRusak {
			if it.SuhuTerima == nil {
				return nil, &domain.ValidationError{Fields: map[string]string{"items": "suhu_terima is required for mudah_rusak bahan"}}
			}
			lolos := *it.SuhuTerima <= *bahan.SuhuSimpanMaks
			lolosSuhu = &lolos
			if !lolos && it.QtyDiterima > 0 {
				return nil, domain.ErrTemperatureExceeded
			}
			if !lolos {
				suhuWarnings = append(suhuWarnings, fmt.Sprintf("bahan %s suhu tidak sesuai", bahan.Nama))
			}
			in.Items[i].LolosSuhu = lolosSuhu
		}

		// Expiry check.
		var expiryWarnings []string
		if it.TglKedaluwarsa != nil {
			expDate, err := time.Parse("2006-01-02", *it.TglKedaluwarsa)
			if err != nil {
				return nil, &domain.ValidationError{Fields: map[string]string{"items": "tgl_kedaluwarsa must use YYYY-MM-DD"}}
			}
			if expDate.Before(today) && it.QtyDiterima > 0 {
				return nil, domain.ErrExpired
			}
			if !expDate.Before(today.AddDate(0, 0, 1)) && it.QtyDiterima > 0 {
				expiryWarnings = append(expiryWarnings, fmt.Sprintf("bahan %s mendekati kedaluwarsa", bahan.Nama))
			}
		}

		// Tolerance: total diterima (existing + new) <= qty_pesan * 1.10.
		maxAllowed := poItem.QtyPesan * 1.10
		newTotal := existingDiterima + it.QtyDiterima
		if newTotal > maxAllowed {
			return nil, domain.ErrExceedsTolerance
		}

		// Nilai potong nota = round(qty_ditolak * harga_satuan, 2).
		nilaiPotongNota := round2(it.QtyDitolak * poItem.HargaSatuan)
		in.Items[i].NilaiPotongNota = nilaiPotongNota

		validated = append(validated, validatedItem{
			item:             it,
			poItem:           poItem,
			bahan:            *bahan,
			existingDiterima: existingDiterima,
			lolosSuhu:        lolosSuhu,
			nilaiPotongNota:  nilaiPotongNota,
			warnings:         append(suhuWarnings, expiryWarnings...),
		})
	}

	// Create lots in one transaction (insert + berkas claim + PO status).
	lots, updatedPO, err := s.penerimaan.CreatePenerimaan(ctx, poID, in, actor.UserID, now)
	if err != nil {
		return nil, fmt.Errorf("create penerimaan: %w", err)
	}

	// Audit log.
	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditCreate,
		Tabel:     "penerimaan_bahan",
		RecordID:  &poID,
		DataBaru:  strPtr(mustJSON(map[string]any{"po_id": poID, "items_count": len(lots), "status": updatedPO.Status})),
		IPAddress: ip,
		CreatedAt: now,
	})

	// Notifications.
	hasRejection := false
	hasTempFailure := false
	for _, it := range in.Items {
		if it.QtyDitolak > 0 {
			hasRejection = true
		}
	}
	for _, v := range validated {
		if v.lolosSuhu != nil && !*v.lolosSuhu && v.item.QtyDiterima > 0 {
			hasTempFailure = true
		}
	}
	if s.notifs != nil {
		if hasRejection {
			_ = s.notifs.NotifySPPGRoles(ctx, sppgID,
				[]string{string(authdomain.RoleAkuntan), string(authdomain.RoleKepalaSPPG)},
				"penerimaan_ditolak", "Penerimaan bahan ditolak",
				fmt.Sprintf("PO %s ada barang yang ditolak", po.NomorPO),
				"penerimaan_bahan", poID)
		}
		if hasTempFailure {
			_ = s.notifs.NotifySPPGRoles(ctx, sppgID,
				[]string{string(authdomain.RoleKepalaSPPG)},
				"suhu_tidak_sesuai", "Suhu penerimaan tidak sesuai",
				fmt.Sprintf("PO %s ada bahan dengan suhu melebihi batas simpan", po.NomorPO),
				"penerimaan_bahan", poID)
		}
	}

	var allWarnings []string
	for _, v := range validated {
		allWarnings = append(allWarnings, v.warnings...)
	}

	return &PenerimaanCreateResult{
		Lots:     lots,
		PO:       updatedPO,
		Warnings: allWarnings,
	}, nil
}

// ListPenerimaan implements GET /penerimaan (MVP-003.9).
func (s *Service) ListPenerimaan(ctx context.Context, actor *authdomain.Claims, filter domain.PenerimaanFilter) (*PenerimaanListResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if err := domain.ValidatePenerimaanFilter(filter); err != nil {
		return nil, err
	}

	var sppgID int64
	if actor.Role == authdomain.RoleAdmin || actor.Role == authdomain.RolePengawas {
		if filter.SPPGID != nil {
			sppgID = *filter.SPPGID
		} else {
			sppgID = 0
		}
	} else {
		if actor.SPPGID == nil {
			return nil, domain.ErrForbidden
		}
		sppgID = *actor.SPPGID
	}

	filter.Normalize()
	items, total, err := s.penerimaan.List(ctx, sppgID, filter)
	if err != nil {
		return nil, fmt.Errorf("list penerimaan: %w", err)
	}
	if items == nil {
		items = []domain.PenerimaanListItem{}
	}
	return &PenerimaanListResult{Data: items, Meta: PageMeta{Page: filter.Page, PerPage: filter.Limit, Total: total}}, nil
}

// GetPenerimaan implements GET /penerimaan/{id} (MVP-003.9).
func (s *Service) GetPenerimaan(ctx context.Context, actor *authdomain.Claims, id int64) (*PenerimaanDetailResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	detail, err := s.penerimaan.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get penerimaan: %w", err)
	}
	if detail == nil {
		return nil, domain.ErrLotNotFound
	}
	// SPPG access check (admin/pengawas see all).
	if actor.Role != authdomain.RoleAdmin && actor.Role != authdomain.RolePengawas {
		if actor.SPPGID == nil || detail.SPPGID != *actor.SPPGID {
			return nil, domain.ErrLotNotFound
		}
	}
	return &PenerimaanDetailResult{Detail: detail}, nil
}

// round2 rounds a float64 to 2 decimal places.
func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// requireKepalaSPPG allows only kepala_sppg to correct receipts.
func requireKepalaSPPG(actor *authdomain.Claims) error {
	if actor == nil {
		return domain.ErrUnauthorized
	}
	if actor.Role != authdomain.RoleKepalaSPPG {
		return domain.ErrForbidden
	}
	if actor.SPPGID == nil {
		return domain.ErrForbidden
	}
	return nil
}

// KoreksiPenerimaanResult carries the corrected lot and the updated PO.
type KoreksiPenerimaanResult struct {
	Detail *domain.PenerimaanDetail
	PO     *domain.PurchaseOrder
}

// KoreksiPenerimaan implements POST /penerimaan/{id}/koreksi (MVP-003.10).
// Only kepala_sppg may correct. Evidence fields are immutable.
// The lot must not be used in a production batch. The corrector cannot be
// the original inspector. All MVP-003.8 rules still apply to the result.
func (s *Service) KoreksiPenerimaan(ctx context.Context, actor *authdomain.Claims, id int64, in domain.KoreksiPenerimaanInput, ip *string) (*KoreksiPenerimaanResult, error) {
	if err := requireKepalaSPPG(actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateKoreksiPenerimaanInput(in); err != nil {
		return nil, err
	}
	sppgID := *actor.SPPGID
	now := s.clock()
	today := startOfDay(now)

	// Load existing lot detail.
	detail, err := s.penerimaan.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get penerimaan: %w", err)
	}
	if detail == nil {
		return nil, domain.ErrLotNotFound
	}
	if detail.SPPGID != sppgID {
		return nil, domain.ErrLotNotFound
	}

	// Lot used in a production batch cannot be corrected.
	if detail.BatchQtyDipakai > 0 {
		return nil, domain.ErrLotUsedInBatch
	}

	// Inspector cannot correct their own receipt.
	if detail.DiperiksaOleh == actor.UserID {
		return nil, domain.ErrSelfCorrection
	}

	// Load PO item to get qty_pesan, harga_satuan, bahan_id, and PO ID.
	itemsRes, err := s.po.ListItemsByPOIDs(ctx, []int64{detail.POID})
	if err != nil {
		return nil, fmt.Errorf("list po items: %w", err)
	}
	poItems := itemsRes[detail.POID]
	if len(poItems) == 0 {
		return nil, domain.ErrItemNotFound
	}
	poItem := poItems[0]

	// Load PO for status check and ownership.
	po, err := s.po.FindByID(ctx, detail.POID)
	if err != nil {
		return nil, fmt.Errorf("find po: %w", err)
	}
	if po == nil || po.SPPGID != sppgID {
		return nil, domain.ErrPONotFound
	}

	// Load bahan for temperature/expiry checks.
	bahan, err := s.bahan.FindByID(ctx, poItem.BahanID)
	if err != nil {
		return nil, fmt.Errorf("find bahan: %w", err)
	}
	if bahan == nil {
		return nil, domain.ErrBahanNotFound
	}

	// Load current rekap for this po_item (includes this lot's current qty).
	rekap, err := s.po.GetPOItemRekap(ctx, []int64{detail.POItemID})
	if err != nil {
		return nil, fmt.Errorf("get po item rekap: %w", err)
	}
	currentRekap := rekap[detail.POItemID]

	// Determine new values (keep existing if not provided).
	newQtyDiterima := detail.QtyDiterima
	if in.QtyDiterima != nil {
		newQtyDiterima = *in.QtyDiterima
	}
	newQtyDitolak := detail.QtyDitolak
	if in.QtyDitolak != nil {
		newQtyDitolak = *in.QtyDitolak
	}
	newKondisi := detail.Kondisi
	if in.Kondisi != nil {
		newKondisi = *in.Kondisi
	}
	newAlasanTolak := detail.AlasanTolak
	if in.AlasanTolak != nil {
		newAlasanTolak = in.AlasanTolak
	}
	newTglKedaluwarsa := detail.TglKedaluwarsa
	if in.TglKedaluwarsa != nil {
		t, err := time.Parse("2006-01-02", *in.TglKedaluwarsa)
		if err != nil {
			return nil, &domain.ValidationError{Fields: map[string]string{"tgl_kedaluwarsa": "must use YYYY-MM-DD"}}
		}
		newTglKedaluwarsa = &t
	}
	newCatatan := detail.Catatan
	if in.Catatan != nil {
		newCatatan = in.Catatan
	}
	newLolosSuhu := detail.LolosSuhu // immutable - keep existing

	// Validate damaged goods: rusak cannot have qty_diterima > 0.
	if newKondisi == domain.KondisiRusak && newQtyDiterima > 0 {
		return nil, domain.ErrDamagedAccepted
	}

	// Temperature check: if mudah_rusak and lolos_suhu = false, qty_diterima must be 0.
	if bahan.MudahRusak && newLolosSuhu != nil && !*newLolosSuhu && newQtyDiterima > 0 {
		return nil, domain.ErrTemperatureExceeded
	}

	// Expiry check: if kedaluwarsa < today and qty_diterima > 0 → expired.
	if newTglKedaluwarsa != nil && newTglKedaluwarsa.Before(today) && newQtyDiterima > 0 {
		return nil, domain.ErrExpired
	}

	// Tolerance: new total for this po_item (excluding this lot's current qty, plus new qty)
	// must not exceed qty_pesan * 1.10.
	existingExcludingThis := currentRekap.QtyDiterima - detail.QtyDiterima
	newTotal := existingExcludingThis + newQtyDiterima
	if newTotal > poItem.QtyPesan*1.10 {
		return nil, domain.ErrExceedsTolerance
	}

	// Nilai potong nota = round(qty_ditolak * harga_satuan, 2).
	newNilaiPotongNota := round2(newQtyDitolak * poItem.HargaSatuan)

	// Call repository to update in transaction.
	updatedDetail, updatedPO, err := s.penerimaan.KoreksiPenerimaan(ctx, id, domain.KoreksiPenerimaanInput{
		Alasan:          in.Alasan,
		QtyDiterima:     &newQtyDiterima,
		QtyDitolak:      &newQtyDitolak,
		AlasanTolak:     newAlasanTolak,
		Kondisi:         &newKondisi,
		TglKedaluwarsa:  in.TglKedaluwarsa, // pass original string; repo stores it
		Catatan:         newCatatan,
		NilaiPotongNota: newNilaiPotongNota,
	}, actor.UserID, now)
	if err != nil {
		return nil, fmt.Errorf("koreksi penerimaan: %w", err)
	}

	// Override computed fields (the repo stored what was passed; we computed values).
	updatedDetail.QtyDiterima = newQtyDiterima
	updatedDetail.QtyDitolak = newQtyDitolak
	updatedDetail.AlasanTolak = newAlasanTolak
	updatedDetail.TglKedaluwarsa = newTglKedaluwarsa
	updatedDetail.Catatan = newCatatan
	updatedDetail.Kondisi = newKondisi
	updatedDetail.NilaiPotongNota = newNilaiPotongNota
	updatedDetail.DikoreksiOleh = &actor.UserID
	updatedDetail.DikoreksiAt = &now

	// Audit log.
	_ = s.audits.Append(ctx, &authdomain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      authdomain.AuditUpdate,
		Tabel:     "penerimaan_bahan",
		RecordID:  &id,
		DataBaru:  strPtr(mustJSON(map[string]any{"penerimaan_id": id, "po_id": detail.POID, "status": updatedPO.Status})),
		IPAddress: ip,
		CreatedAt: now,
	})

	// Notify akuntan that nilai tagihan may have changed.
	if s.notifs != nil {
		_ = s.notifs.NotifySPPGRoles(ctx, sppgID,
			[]string{string(authdomain.RoleAkuntan)},
			"penerimaan_dikoreksi", "Penerimaan bahan dikoreksi",
			fmt.Sprintf("PO %s penerimaan telah dikoreksi", detail.NomorPO),
			"penerimaan_bahan", id)
	}

	return &KoreksiPenerimaanResult{
		Detail: updatedDetail,
		PO:     updatedPO,
	}, nil
}

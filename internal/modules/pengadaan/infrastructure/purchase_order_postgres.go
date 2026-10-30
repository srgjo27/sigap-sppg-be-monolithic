package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// PurchaseOrderPostgres implements PurchaseOrderRepository
type PurchaseOrderPostgres struct {
	Pool *pgxpool.Pool
}

func NewPurchaseOrderPostgres(pool *pgxpool.Pool) *PurchaseOrderPostgres {
	return &PurchaseOrderPostgres{Pool: pool}
}

const poColumns = `id, sppg_id, nomor_po, pemasok_id, siklus_id, tgl_po, tgl_kirim_diminta, total, status, dibuat_oleh, catatan, dikirim_oleh, dikirim_at, ditutup_oleh, ditutup_at, alasan_tutup, created_at, updated_at`

func scanPO(row interface{ Scan(...any) error }) (*domain.PurchaseOrder, error) {
	var po domain.PurchaseOrder
	var siklusID sql.NullInt64
	var tglKirim sql.NullTime
	var catatan sql.NullString
	var dikirimOleh, ditutupOleh sql.NullInt64
	var dikirimAt, ditutupAt sql.NullTime
	var alasanTutup sql.NullString
	var created, updated sql.NullTime
	var total sql.NullFloat64
	var status sql.NullString
	if err := row.Scan(&po.ID, &po.SPPGID, &po.NomorPO, &po.PemasokID, &siklusID, &po.TglPO, &tglKirim, &total, &status, &po.DibuatOleh, &catatan, &dikirimOleh, &dikirimAt, &ditutupOleh, &ditutupAt, &alasanTutup, &created, &updated); err != nil {
		return nil, err
	}
	if siklusID.Valid {
		v := siklusID.Int64
		po.SiklusID = &v
	}
	if tglKirim.Valid {
		po.TglKirimDiminta = &tglKirim.Time
	}
	if total.Valid {
		po.Total = total.Float64
	}
	po.Status = status.String
	if catatan.Valid {
		po.Catatan = &catatan.String
	}
	if dikirimOleh.Valid {
		v := dikirimOleh.Int64
		po.DikirimOleh = &v
	}
	if dikirimAt.Valid {
		po.DikirimAt = &dikirimAt.Time
	}
	if ditutupOleh.Valid {
		v := ditutupOleh.Int64
		po.DitutupOleh = &v
	}
	if ditutupAt.Valid {
		po.DitutupAt = &ditutupAt.Time
	}
	if alasanTutup.Valid {
		po.AlasanTutup = &alasanTutup.String
	}
	if created.Valid {
		po.CreatedAt = created.Time
	}
	if updated.Valid {
		po.UpdatedAt = updated.Time
	}
	return &po, nil
}

const poItemColumns = `id, po_id, bahan_id, qty_pesan, harga_satuan, subtotal`

func scanPOItem(row interface{ Scan(...any) error }) (*domain.POItem, error) {
	var item domain.POItem
	var qty, harga, subtotal sql.NullFloat64
	if err := row.Scan(&item.ID, &item.POID, &item.BahanID, &qty, &harga, &subtotal); err != nil {
		return nil, err
	}
	item.QtyPesan = qty.Float64
	item.HargaSatuan = harga.Float64
	item.Subtotal = subtotal.Float64
	return &item, nil
}

func (p *PurchaseOrderPostgres) ListBySPPGAndStatus(ctx context.Context, sppgID int64, statuses []string) ([]domain.PurchaseOrder, error) {
	statusClause := ""
	args := []any{sppgID}
	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i := range statuses {
			placeholders[i] = fmt.Sprintf("$%d::po_status", i+2)
			args = append(args, statuses[i])
		}
		statusClause = " AND status IN (" + strings.Join(placeholders, ",") + ")"
	}
	q := `SELECT ` + poColumns + ` FROM purchase_order WHERE sppg_id=$1` + statusClause + ` ORDER BY tgl_kirim_diminta ASC NULLS LAST, nomor_po ASC`
	rows, err := p.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list pos: %w", err)
	}
	defer rows.Close()
	var out []domain.PurchaseOrder
	for rows.Next() {
		po, err := scanPO(rows)
		if err != nil {
			return nil, fmt.Errorf("scan po: %w", err)
		}
		out = append(out, *po)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows pos: %w", err)
	}
	if out == nil {
		out = []domain.PurchaseOrder{}
	}
	return out, nil
}

func (p *PurchaseOrderPostgres) ListItemsByPOIDs(ctx context.Context, poIDs []int64) (map[int64][]domain.POItem, error) {
	out := map[int64][]domain.POItem{}
	for _, id := range poIDs {
		out[id] = []domain.POItem{}
	}
	if len(poIDs) == 0 {
		return out, nil
	}
	const q = `SELECT ` + poItemColumns + ` FROM po_item WHERE po_id = ANY($1) ORDER BY id ASC`
	rows, err := p.Pool.Query(ctx, q, poIDs)
	if err != nil {
		return nil, fmt.Errorf("list po items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanPOItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan po item: %w", err)
		}
		out[item.POID] = append(out[item.POID], *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows po items: %w", err)
	}
	return out, nil
}

func (p *PurchaseOrderPostgres) ListPOMenus(ctx context.Context, menuIDs []int64) (map[int64][]int64, error) {
	if len(menuIDs) == 0 {
		return map[int64][]int64{}, nil
	}
	const q = `SELECT po_id, menu_id FROM po_menu WHERE menu_id = ANY($1)`
	rows, err := p.Pool.Query(ctx, q, menuIDs)
	if err != nil {
		return nil, fmt.Errorf("list po menus: %w", err)
	}
	defer rows.Close()
	result := make(map[int64][]int64)
	for rows.Next() {
		var poID, menuID int64
		if err := rows.Scan(&poID, &menuID); err != nil {
			return nil, fmt.Errorf("scan po_menu: %w", err)
		}
		result[menuID] = append(result[menuID], poID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows po_menu: %w", err)
	}
	return result, nil
}

// Create creates a purchase order with items and menu links in a transaction.
func (p *PurchaseOrderPostgres) Create(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, menuIDs []int64) (*domain.PurchaseOrder, []domain.POItem, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert purchase_order
	const poQ = `INSERT INTO purchase_order (sppg_id, nomor_po, pemasok_id, siklus_id, tgl_po, tgl_kirim_diminta, status, dibuat_oleh, catatan, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,now(),now()) RETURNING ` + poColumns
	var siklusID any
	if po.SiklusID != nil {
		siklusID = *po.SiklusID
	}
	var tglKirim any
	if po.TglKirimDiminta != nil {
		tglKirim = *po.TglKirimDiminta
	}
	var catatan any
	if po.Catatan != nil {
		catatan = *po.Catatan
	}
	row := tx.QueryRow(ctx, poQ, po.SPPGID, po.NomorPO, po.PemasokID, siklusID, po.TglPO, tglKirim, po.Status, po.DibuatOleh, catatan)
	createdPO, err := scanPO(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, nil, domain.ErrPONumberConflict
		}
		return nil, nil, fmt.Errorf("insert purchase_order: %w", err)
	}

	// Insert po_item
	createdItems := make([]domain.POItem, 0, len(items))
	for _, item := range items {
		const itemQ = `INSERT INTO po_item (po_id, bahan_id, qty_pesan, harga_satuan) VALUES ($1,$2,$3,$4) RETURNING ` + poItemColumns
		itemRow := tx.QueryRow(ctx, itemQ, createdPO.ID, item.BahanID, item.QtyPesan, item.HargaSatuan)
		createdItem, err := scanPOItem(itemRow)
		if err != nil {
			if isUniqueViolation(err) {
				return nil, nil, domain.ErrItemDuplicate
			}
			return nil, nil, fmt.Errorf("insert po_item: %w", err)
		}
		createdItems = append(createdItems, *createdItem)
	}

	// Insert po_menu links
	for _, menuID := range menuIDs {
		const menuQ = `INSERT INTO po_menu (po_id, menu_id) VALUES ($1,$2)`
		if _, err := tx.Exec(ctx, menuQ, createdPO.ID, menuID); err != nil {
			if isForeignKeyViolation(err) {
				return nil, nil, domain.ErrMenuNotFound
			}
			return nil, nil, fmt.Errorf("insert po_menu: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit po: %w", err)
	}

	return createdPO, createdItems, nil
}

// Update replaces PO header fields and, when replaceItems/replaceMenus are
// set, the whole po_item / po_menu sets in a single transaction
// The status guard re-checks 'draf' inside the transaction.
func (p *PurchaseOrderPostgres) Update(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, replaceItems bool, menuIDs []int64, replaceMenus bool) (*domain.PurchaseOrder, []domain.POItem, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const poQ = `UPDATE purchase_order SET pemasok_id=$2, siklus_id=$3, tgl_kirim_diminta=$4, catatan=$5, updated_at=now()
		WHERE id=$1 AND status='draf' RETURNING ` + poColumns
	var siklusID any
	if po.SiklusID != nil {
		siklusID = *po.SiklusID
	}
	var tglKirim any
	if po.TglKirimDiminta != nil {
		tglKirim = *po.TglKirimDiminta
	}
	var catatan any
	if po.Catatan != nil {
		catatan = *po.Catatan
	}
	row := tx.QueryRow(ctx, poQ, po.ID, po.PemasokID, siklusID, tglKirim, catatan)
	updatedPO, err := scanPO(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil, domain.ErrPOInvalidStatus
		}
		return nil, nil, fmt.Errorf("update purchase_order: %w", err)
	}

	var updatedItems []domain.POItem
	if replaceItems {
		if _, err := tx.Exec(ctx, `DELETE FROM po_item WHERE po_id=$1`, po.ID); err != nil {
			return nil, nil, fmt.Errorf("reset po_item: %w", err)
		}
		updatedItems = make([]domain.POItem, 0, len(items))
		for _, item := range items {
			const itemQ = `INSERT INTO po_item (po_id, bahan_id, qty_pesan, harga_satuan) VALUES ($1,$2,$3,$4) RETURNING ` + poItemColumns
			createdItem, err := scanPOItem(tx.QueryRow(ctx, itemQ, po.ID, item.BahanID, item.QtyPesan, item.HargaSatuan))
			if err != nil {
				if isUniqueViolation(err) {
					return nil, nil, domain.ErrItemDuplicate
				}
				return nil, nil, fmt.Errorf("insert po_item: %w", err)
			}
			updatedItems = append(updatedItems, *createdItem)
		}
	}

	if replaceMenus {
		if _, err := tx.Exec(ctx, `DELETE FROM po_menu WHERE po_id=$1`, po.ID); err != nil {
			return nil, nil, fmt.Errorf("reset po_menu: %w", err)
		}
		for _, menuID := range menuIDs {
			const menuQ = `INSERT INTO po_menu (po_id, menu_id) VALUES ($1,$2)`
			if _, err := tx.Exec(ctx, menuQ, po.ID, menuID); err != nil {
				if isForeignKeyViolation(err) {
					return nil, nil, domain.ErrMenuNotFound
				}
				return nil, nil, fmt.Errorf("insert po_menu: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit po update: %w", err)
	}
	return updatedPO, updatedItems, nil
}

// Delete removes a draft PO; po_item and po_menu rows cascade.
func (p *PurchaseOrderPostgres) Delete(ctx context.Context, id int64) error {
	tag, err := p.Pool.Exec(ctx, `DELETE FROM purchase_order WHERE id=$1 AND status='draf'`, id)
	if err != nil {
		return fmt.Errorf("delete po: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPOInvalidStatus
	}
	return nil
}

// ListMenuIDsForPO returns menu ids linked to a PO.
func (p *PurchaseOrderPostgres) ListMenuIDsForPO(ctx context.Context, poID int64) ([]int64, error) {
	rows, err := p.Pool.Query(ctx, `SELECT menu_id FROM po_menu WHERE po_id=$1 ORDER BY menu_id ASC`, poID)
	if err != nil {
		return nil, fmt.Errorf("list po menus for po: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var menuID int64
		if err := rows.Scan(&menuID); err != nil {
			return nil, fmt.Errorf("scan po menu: %w", err)
		}
		out = append(out, menuID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows po menus: %w", err)
	}
	return out, nil
}

// HasPenerimaan reports whether the PO has at least one receipt lot.
func (p *PurchaseOrderPostgres) HasPenerimaan(ctx context.Context, poID int64) (bool, error) {
	const q = `SELECT EXISTS(
		SELECT 1 FROM penerimaan_bahan pb JOIN po_item pi ON pi.id = pb.po_item_id WHERE pi.po_id = $1)`
	var exists bool
	if err := p.Pool.QueryRow(ctx, q, poID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check penerimaan: %w", err)
	}
	return exists, nil
}

// SetDikirim moves draf -> dikirim and stamps the sender, guarded on status.
func (p *PurchaseOrderPostgres) SetDikirim(ctx context.Context, id int64, userID int64, at time.Time) (*domain.PurchaseOrder, error) {
	const q = `UPDATE purchase_order SET status='dikirim', dikirim_oleh=$2, dikirim_at=$3, updated_at=now()
		WHERE id=$1 AND status='draf' RETURNING ` + poColumns
	row := p.Pool.QueryRow(ctx, q, id, userID, at)
	po, err := scanPO(row)
	if err != nil {
		if isNoRows(err) {
			return nil, domain.ErrPOInvalidStatus
		}
		return nil, fmt.Errorf("set po dikirim: %w", err)
	}
	return po, nil
}

// SetStatus moves fromStatus -> toStatus and stamps ditutup_* columns,
// guarded on the current status to prevent concurrent transitions.
func (p *PurchaseOrderPostgres) SetStatus(ctx context.Context, id int64, fromStatus, toStatus string, userID int64, alasan *string, at time.Time) (*domain.PurchaseOrder, error) {
	const q = `UPDATE purchase_order SET status=$3::po_status, ditutup_oleh=$4, ditutup_at=$5, alasan_tutup=$6, updated_at=now()
		WHERE id=$1 AND status=$2::po_status RETURNING ` + poColumns
	row := p.Pool.QueryRow(ctx, q, id, fromStatus, toStatus, userID, at, nullableStr(alasan))
	po, err := scanPO(row)
	if err != nil {
		if isNoRows(err) {
			return nil, domain.ErrPOInvalidStatus
		}
		return nil, fmt.Errorf("set po status: %w", err)
	}
	return po, nil
}

// FindByID looks up a purchase order by ID.
func (p *PurchaseOrderPostgres) FindByID(ctx context.Context, id int64) (*domain.PurchaseOrder, error) {
	const q = `SELECT ` + poColumns + ` FROM purchase_order WHERE id=$1 LIMIT 1`
	row := p.Pool.QueryRow(ctx, q, id)
	res, err := scanPO(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find po: %w", err)
	}
	return res, nil
}

// List returns filtered purchase orders for list view.
func (p *PurchaseOrderPostgres) List(ctx context.Context, sppgID int64, filter domain.POFilter) ([]domain.POListItem, int, error) {
	filter.Normalize()
	where := `WHERE ($1::bigint = 0 OR po.sppg_id=$1)`
	args := []any{sppgID}
	argIdx := 2

	if len(filter.Status) > 0 {
		placeholders := make([]string, len(filter.Status))
		for i := range filter.Status {
			placeholders[i] = fmt.Sprintf("$%d::po_status", argIdx)
			args = append(args, filter.Status[i])
			argIdx++
		}
		where += " AND po.status IN (" + strings.Join(placeholders, ",") + ")"
	}
	if filter.PemasokID != nil {
		where += fmt.Sprintf(" AND po.pemasok_id=$%d", argIdx)
		args = append(args, *filter.PemasokID)
		argIdx++
	}
	if filter.TglKirimDari != nil {
		where += fmt.Sprintf(" AND po.tgl_kirim_diminta >= $%d", argIdx)
		args = append(args, *filter.TglKirimDari)
		argIdx++
	}
	if filter.TglKirimSampai != nil {
		where += fmt.Sprintf(" AND po.tgl_kirim_diminta <= $%d", argIdx)
		args = append(args, *filter.TglKirimSampai)
		argIdx++
	}
	if filter.Q != nil && *filter.Q != "" {
		where += fmt.Sprintf(" AND po.nomor_po ILIKE $%d", argIdx)
		args = append(args, "%"+*filter.Q+"%")
		argIdx++
	}

	countQ := `SELECT COUNT(*) FROM purchase_order po ` + where
	var total int
	if err := p.Pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count pos: %w", err)
	}

	listQ := `SELECT po.id, po.nomor_po, po.pemasok_id, p.nama as pemasok_nama, po.tgl_po, po.tgl_kirim_diminta, po.status, po.total,
		(SELECT COUNT(*) FROM po_item WHERE po_id=po.id) as jumlah_item,
		(SELECT COUNT(*) FROM po_item pi WHERE pi.po_id=po.id AND pi.qty_pesan <= COALESCE((SELECT SUM(pb.qty_diterima) FROM penerimaan_bahan pb WHERE pb.po_item_id=pi.id), 0)) as jumlah_item_terpenuhi
		FROM purchase_order po
		JOIN pemasok p ON p.id = po.pemasok_id
		` + where + ` ORDER BY po.tgl_kirim_diminta ASC NULLS LAST, po.nomor_po ASC LIMIT $` + fmt.Sprintf("%d", argIdx) + ` OFFSET $` + fmt.Sprintf("%d", argIdx+1)
	args = append(args, filter.Limit, filter.Offset())

	rows, err := p.Pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list pos: %w", err)
	}
	defer rows.Close()

	var out []domain.POListItem
	for rows.Next() {
		var item domain.POListItem
		var tglKirim sql.NullTime
		var total sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.NomorPO, &item.PemasokID, &item.PemasokNama, &item.TglPO, &tglKirim, &item.Status, &total, &item.JumlahItem, &item.JumlahItemTerpenuhi); err != nil {
			return nil, 0, fmt.Errorf("scan po list: %w", err)
		}
		if tglKirim.Valid {
			item.TglKirimDiminta = &tglKirim.Time
		}
		if total.Valid {
			item.Total = total.Float64
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows po list: %w", err)
	}
	if out == nil {
		out = []domain.POListItem{}
	}
	return out, total, nil
}

// GetDetail returns detailed PO with items, rekap, penerimaan, and menus.
func (p *PurchaseOrderPostgres) GetDetail(ctx context.Context, id int64) (*domain.PODetail, error) {
	// Get PO with pemasok name (explicit columns in poColumns order — po.* returns physical order)
	const poQ = `SELECT po.id, po.sppg_id, po.nomor_po, po.pemasok_id, po.siklus_id, po.tgl_po, po.tgl_kirim_diminta, po.total, po.status, po.dibuat_oleh, po.catatan, po.dikirim_oleh, po.dikirim_at, po.ditutup_oleh, po.ditutup_at, po.alasan_tutup, po.created_at, po.updated_at, p.nama as pemasok_nama
		FROM purchase_order po JOIN pemasok p ON p.id=po.pemasok_id WHERE po.id=$1`
	row := p.Pool.QueryRow(ctx, poQ, id)
	var po domain.PurchaseOrder
	var pemasokNama string
	var siklusID sql.NullInt64
	var tglKirim sql.NullTime
	var catatan sql.NullString
	var dikirimOleh, ditutupOleh sql.NullInt64
	var dikirimAt, ditutupAt sql.NullTime
	var alasanTutup sql.NullString
	var created, updated sql.NullTime
	var total sql.NullFloat64
	var status sql.NullString
	if err := row.Scan(&po.ID, &po.SPPGID, &po.NomorPO, &po.PemasokID, &siklusID, &po.TglPO, &tglKirim, &total, &status, &po.DibuatOleh, &catatan, &dikirimOleh, &dikirimAt, &ditutupOleh, &ditutupAt, &alasanTutup, &created, &updated, &pemasokNama); err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find po detail: %w", err)
	}
	if siklusID.Valid {
		v := siklusID.Int64
		po.SiklusID = &v
	}
	if tglKirim.Valid {
		po.TglKirimDiminta = &tglKirim.Time
	}
	if total.Valid {
		po.Total = total.Float64
	}
	po.Status = status.String
	if catatan.Valid {
		po.Catatan = &catatan.String
	}
	if dikirimOleh.Valid {
		v := dikirimOleh.Int64
		po.DikirimOleh = &v
	}
	if dikirimAt.Valid {
		po.DikirimAt = &dikirimAt.Time
	}
	if ditutupOleh.Valid {
		v := ditutupOleh.Int64
		po.DitutupOleh = &v
	}
	if ditutupAt.Valid {
		po.DitutupAt = &ditutupAt.Time
	}
	if alasanTutup.Valid {
		po.AlasanTutup = &alasanTutup.String
	}
	if created.Valid {
		po.CreatedAt = created.Time
	}
	if updated.Valid {
		po.UpdatedAt = updated.Time
	}

	detail := &domain.PODetail{
		PurchaseOrder: po,
		PemasokNama:   pemasokNama,
	}

	// Get items with rekap from v_po_item_rekap
	const itemsQ = `SELECT pi.id, pi.bahan_id, pi.qty_pesan, pi.harga_satuan, pi.subtotal,
		b.nama, b.satuan, b.mudah_rusak,
		COALESCE(r.qty_diterima, 0) as qty_diterima,
		COALESCE(r.qty_ditolak, 0) as qty_ditolak,
		COALESCE(r.nilai_potong_nota, 0) as nilai_potong_nota,
		COALESCE(r.terpenuhi, false) as terpenuhi,
		COALESCE(r.nilai_tagihan, 0) as nilai_tagihan
		FROM po_item pi
		JOIN bahan b ON b.id = pi.bahan_id
		LEFT JOIN v_po_item_rekap r ON r.po_item_id = pi.id
		WHERE pi.po_id = $1 ORDER BY pi.id ASC`
	rows, err := p.Pool.Query(ctx, itemsQ, id)
	if err != nil {
		return nil, fmt.Errorf("get po items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.PODetailItem
		var qty, harga, subtotal, qtyDiterima, qtyDitolak, potong, tagihan sql.NullFloat64
		var terpenuhi sql.NullBool
		if err := rows.Scan(&item.ID, &item.BahanID, &qty, &harga, &subtotal, &item.BahanNama, &item.BahanSatuan, &item.BahanMudahRusak, &qtyDiterima, &qtyDitolak, &potong, &terpenuhi, &tagihan); err != nil {
			return nil, fmt.Errorf("scan po detail item: %w", err)
		}
		item.QtyPesan = qty.Float64
		item.HargaSatuan = harga.Float64
		item.Subtotal = subtotal.Float64
		item.QtyDiterima = qtyDiterima.Float64
		item.QtyDitolak = qtyDitolak.Float64
		item.QtyKurang = item.QtyPesan - item.QtyDiterima
		if item.QtyKurang < 0 {
			item.QtyKurang = 0
		}
		item.NilaiPotongNota = potong.Float64
		item.Terpenuhi = terpenuhi.Bool
		item.NilaiTagihan = tagihan.Float64
		detail.Items = append(detail.Items, item)
	}

	// Compute rekap nilai
	var totalPesanan, totalNilaiDikirim, totalPotongNota, totalTagihan float64
	for _, item := range detail.Items {
		totalPesanan += item.Subtotal
		totalNilaiDikirim += (item.QtyDiterima + item.QtyDitolak) * item.HargaSatuan
		totalPotongNota += item.NilaiPotongNota
		totalTagihan += item.NilaiTagihan
	}
	detail.RekapNilai = domain.PODetailRekap{
		TotalPesanan:      totalPesanan,
		TotalNilaiDikirim: totalNilaiDikirim,
		TotalPotongNota:   totalPotongNota,
		TotalTagihan:      totalTagihan,
	}

	// Get penerimaan (lots)
	const penerimaanQ = `SELECT pb.id, pb.kode_lot, pb.po_item_id, pb.waktu_terima, pb.qty_diterima, pb.qty_ditolak, pb.kondisi, pb.diperiksa_oleh
		FROM penerimaan_bahan pb
		JOIN po_item pi ON pi.id = pb.po_item_id
		WHERE pi.po_id = $1 ORDER BY pb.waktu_terima ASC`
	penerimaanRows, err := p.Pool.Query(ctx, penerimaanQ, id)
	if err != nil {
		return nil, fmt.Errorf("get penerimaan: %w", err)
	}
	defer penerimaanRows.Close()
	for penerimaanRows.Next() {
		var p domain.PODetailPenerimaan
		if err := penerimaanRows.Scan(&p.ID, &p.KodeLot, &p.POItemID, &p.WaktuTerima, &p.QtyDiterima, &p.QtyDitolak, &p.Kondisi, &p.DiperiksaOleh); err != nil {
			return nil, fmt.Errorf("scan penerimaan: %w", err)
		}
		detail.Penerimaan = append(detail.Penerimaan, p)
	}

	// Get linked menus
	const menusQ = `SELECT m.id, m.tanggal, m.nama_menu FROM menu m JOIN po_menu pm ON pm.menu_id = m.id WHERE pm.po_id = $1 ORDER BY m.tanggal ASC`
	menuRows, err := p.Pool.Query(ctx, menusQ, id)
	if err != nil {
		return nil, fmt.Errorf("get po menus: %w", err)
	}
	defer menuRows.Close()
	for menuRows.Next() {
		var m domain.PODetailMenu
		if err := menuRows.Scan(&m.ID, &m.Tanggal, &m.NamaMenu); err != nil {
			return nil, fmt.Errorf("scan po menu: %w", err)
		}
		detail.Menu = append(detail.Menu, m)
	}

	return detail, nil
}

// GetPOItemRekap returns rekap data for PO items.
func (p *PurchaseOrderPostgres) GetPOItemRekap(ctx context.Context, poItemIDs []int64) (map[int64]domain.POItemRekap, error) {
	out := make(map[int64]domain.POItemRekap)
	if len(poItemIDs) == 0 {
		return out, nil
	}
	const q = `SELECT po_item_id, qty_diterima, qty_ditolak, nilai_potong_nota, terpenuhi, nilai_tagihan FROM v_po_item_rekap WHERE po_item_id = ANY($1)`
	rows, err := p.Pool.Query(ctx, q, poItemIDs)
	if err != nil {
		return nil, fmt.Errorf("get po item rekap: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r domain.POItemRekap
		var qtyDiterima, qtyDitolak, potong, tagihan sql.NullFloat64
		var terpenuhi sql.NullBool
		if err := rows.Scan(&r.POItemID, &qtyDiterima, &qtyDitolak, &potong, &terpenuhi, &tagihan); err != nil {
			return nil, fmt.Errorf("scan po item rekap: %w", err)
		}
		r.QtyDiterima = qtyDiterima.Float64
		r.QtyDitolak = qtyDitolak.Float64
		r.NilaiPotongNota = potong.Float64
		r.Terpenuhi = terpenuhi.Bool
		r.NilaiTagihan = tagihan.Float64
		out[r.POItemID] = r
	}
	return out, nil
}

// ReserveNumberSeq reserves the next PO number sequence for the SPPG and
// month inside a single transaction guarded by a transaction-scoped advisory
// lock. The next value is the highest currently reserved or still attached
// to any PO, + 1, so numbers of deleted POs are never reused.
// Sequence selection and reservation commit together, so a failed allocation
// releases its number. A rare conflict after the lock (e.g. rows inserted
// outside this flow) is retried; exhaustion yields ErrPONumberConflict.
func (p *PurchaseOrderPostgres) ReserveNumberSeq(ctx context.Context, sppgID int64, tahunBulan string) (int, error) {
	ym, err := strconv.Atoi(tahunBulan)
	if err != nil {
		return 0, fmt.Errorf("reserve po number: parse tahun_bulan %q: %w", tahunBulan, err)
	}

	for attempt := 0; attempt < 3; attempt++ {
		seq, err := p.reserveNumberOnce(ctx, sppgID, int64(ym), tahunBulan)
		if err == nil {
			return seq, nil
		}
		if !isUniqueViolation(err) {
			return 0, fmt.Errorf("reserve po number: %w", err)
		}
	}
	return 0, fmt.Errorf("reserve po number: %w", domain.ErrPONumberConflict)
}

// reserveNumberOnce allocates one sequence number attempt inside a
// transaction; the deferred rollback discards everything on failure, which
// also releases the transaction-scoped advisory lock.
func (p *PurchaseOrderPostgres) reserveNumberOnce(ctx context.Context, sppgID, ym int64, tahunBulan string) (int, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Serialize concurrent issuances for the same SPPG and month.
	const lockQ = `SELECT pg_advisory_xact_lock($1)`
	lockKey := sppgID*1_000_000 + ym
	if _, err := tx.Exec(ctx, lockQ, lockKey); err != nil {
		return 0, fmt.Errorf("advisory lock: %w", err)
	}

	const seqQ = `
		SELECT COALESCE(GREATEST(
			(SELECT COALESCE(MAX(urut), 0) FROM po_nomor_terpakai WHERE sppg_id=$1 AND tahun_bulan=$2),
			(SELECT COALESCE(MAX(CAST(SUBSTRING(nomor_po FROM '(\d+)$') AS INT)), 0)
				FROM purchase_order WHERE sppg_id=$1 AND nomor_po LIKE $3)
		), 0) + 1`
	prefixLike := fmt.Sprintf("PO/%%/%s/%%", tahunBulan)

	var seq int
	if err := tx.QueryRow(ctx, seqQ, sppgID, tahunBulan, prefixLike).Scan(&seq); err != nil {
		return 0, fmt.Errorf("get sequence: %w", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO po_nomor_terpakai (sppg_id, tahun_bulan, urut) VALUES ($1, $2, $3)`, sppgID, tahunBulan, seq); err != nil {
		return 0, fmt.Errorf("insert reservation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit po number reservation: %w", err)
	}
	return seq, nil
}

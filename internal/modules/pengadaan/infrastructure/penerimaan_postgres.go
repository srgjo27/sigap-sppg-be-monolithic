package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// PenerimaanPostgres implements PenerimaanRepository.
type PenerimaanPostgres struct {
	Pool *pgxpool.Pool
}

func NewPenerimaanPostgres(pool *pgxpool.Pool) *PenerimaanPostgres {
	return &PenerimaanPostgres{Pool: pool}
}

// penerimaanColumnsInsert is the same column list without the "pb." table
// alias prefix, for use in INSERT ... RETURNING clauses.
const penerimaanColumnsInsert = `id, po_item_id, kode_lot, waktu_terima, qty_diterima, qty_ditolak,
	alasan_tolak, suhu_terima, kondisi, tgl_kedaluwarsa, foto_url,
	diperiksa_oleh, nilai_potong_nota, sppg_id, client_ref, lolos_suhu,
	catatan, dikoreksi_oleh, dikoreksi_at, created_at`

func scanPenerimaan(row interface{ Scan(...any) error }) (*domain.PenerimaanBahan, error) {
	var p domain.PenerimaanBahan
	var waktuTerima, createdAt time.Time
	var suhuTerima sql.NullFloat64
	var tglKedaluwarsaDate sql.NullTime
	var alasanTolak, catatan sql.NullString
	var dikoreksiOleh sql.NullInt64
	var dikoreksiAt sql.NullTime
	var lolosSuhu sql.NullBool
	var fotoURL sql.NullString
	if err := row.Scan(&p.ID, &p.POItemID, &p.KodeLot, &waktuTerima, &p.QtyDiterima, &p.QtyDitolak,
		&alasanTolak, &suhuTerima, &p.Kondisi, &tglKedaluwarsaDate, &fotoURL,
		&p.DiperiksaOleh, &p.NilaiPotongNota, &p.SPPGID, &p.ClientRef, &lolosSuhu,
		&catatan, &dikoreksiOleh, &dikoreksiAt, &createdAt); err != nil {
		return nil, err
	}
	p.WaktuTerima = waktuTerima
	if suhuTerima.Valid {
		v := suhuTerima.Float64
		p.SuhuTerima = &v
	}
	if tglKedaluwarsaDate.Valid {
		p.TglKedaluwarsa = &tglKedaluwarsaDate.Time
	}
	if alasanTolak.Valid {
		p.AlasanTolak = &alasanTolak.String
	}
	if fotoURL.Valid {
		p.FotoURL = fotoURL.String
	}
	if lolosSuhu.Valid {
		p.LolosSuhu = &lolosSuhu.Bool
	}
	if catatan.Valid {
		p.Catatan = &catatan.String
	}
	if dikoreksiOleh.Valid {
		v := dikoreksiOleh.Int64
		p.DikoreksiOleh = &v
	}
	if dikoreksiAt.Valid {
		p.DikoreksiAt = &dikoreksiAt.Time
	}
	p.CreatedAt = createdAt
	return &p, nil
}

// CreatePenerimaan inserts receipt lots, marks foto berkas as used, and
// advances PO status — all in one transaction.
func (p *PenerimaanPostgres) CreatePenerimaan(ctx context.Context, poID int64, in domain.CreatePenerimaanInput, diperiksaOleh int64, at time.Time) ([]*domain.PenerimaanBahan, *domain.PurchaseOrder, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Idempotency: if any lot exists with this client_ref for this PO, return
	// the existing lots (replay-safe).
	const idemQ = `
		SELECT pb.id, pb.kode_lot, pb.po_item_id, pb.waktu_terima, pb.qty_diterima, pb.qty_ditolak,
			pb.alasan_tolak, pb.suhu_terima, pb.kondisi, pb.tgl_kedaluwarsa, pb.foto_url,
			pb.diperiksa_oleh, pb.nilai_potong_nota, pb.sppg_id, pb.client_ref, pb.lolos_suhu,
			pb.catatan, pb.dikoreksi_oleh, pb.dikoreksi_at, pb.created_at
		FROM penerimaan_bahan pb
		JOIN po_item pi ON pi.id = pb.po_item_id
		WHERE pb.client_ref::text = $1 AND pi.po_id = $2`
	rows, err := tx.Query(ctx, idemQ, in.ClientRef, poID)
	if err != nil {
		return nil, nil, fmt.Errorf("check idempotency: %w", err)
	}
	var existing []*domain.PenerimaanBahan
	for rows.Next() {
		lot, err := scanPenerimaan(rows)
		if err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scan existing lot: %w", err)
		}
		existing = append(existing, lot)
	}
	rows.Close()
	if len(existing) > 0 {
		// Idempotent replay: return existing lots and current PO.
		po, err := findPOSTx(ctx, tx, poID)
		if err != nil {
			return nil, nil, err
		}
		return existing, po, nil
	}

	// Get sppg_kode and PO items for kode_lot generation and validation.
	var sppgID int64
	var kodeLotPrefix string
	poRow := tx.QueryRow(ctx, `SELECT sppg_id FROM purchase_order WHERE id=$1`, poID)
	if err := poRow.Scan(&sppgID); err != nil {
		if isNoRows(err) {
			return nil, nil, domain.ErrPONotFound
		}
		return nil, nil, fmt.Errorf("get po sppg: %w", err)
	}

	// Kode_lot generation with advisory lock (per SPPG per day).
	dateStr := at.Format("20060102")
	lockKey := sppgID*1_000_000 + int64(parseYM(dateStr))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockKey); err != nil {
		return nil, nil, fmt.Errorf("advisory lock: %w", err)
	}
	kodeLotPrefix = fmt.Sprintf("LOT-%s-%s-", "", dateStr) // kode filled below
	var kode string
	if err := tx.QueryRow(ctx, `SELECT kode FROM sppg WHERE id=$1`, sppgID).Scan(&kode); err != nil {
		return nil, nil, fmt.Errorf("get sppg kode: %w", err)
	}
	kodeLotPrefix = fmt.Sprintf("LOT-%s-%s-", kode, dateStr)

	var maxUrut int
	const maxUrutQ = `
		SELECT COALESCE(MAX(CAST(SUBSTRING(kode_lot FROM '-(\d{3})$') AS INT)), 0)
		FROM penerimaan_bahan
		WHERE sppg_id = $1 AND kode_lot LIKE $2`
	if err := tx.QueryRow(ctx, maxUrutQ, sppgID, kodeLotPrefix+"%").Scan(&maxUrut); err != nil {
		return nil, nil, fmt.Errorf("max urut: %w", err)
	}

	// Get PO items map for validation.
	poItemRows, err := tx.Query(ctx, `SELECT id, bahan_id, qty_pesan, harga_satuan FROM po_item WHERE po_id=$1`, poID)
	if err != nil {
		return nil, nil, fmt.Errorf("list po items: %w", err)
	}
	poItemMap := make(map[int64]struct {
		bahanID     int64
		qtyPesan    float64
		hargaSatuan float64
	})
	for poItemRows.Next() {
		var id, bahanID int64
		var qtyPesan, hargaSatuan float64
		if err := poItemRows.Scan(&id, &bahanID, &qtyPesan, &hargaSatuan); err != nil {
			poItemRows.Close()
			return nil, nil, fmt.Errorf("scan po item: %w", err)
		}
		poItemMap[id] = struct {
			bahanID     int64
			qtyPesan    float64
			hargaSatuan float64
		}{bahanID, qtyPesan, hargaSatuan}
	}
	poItemRows.Close()

	// Get current rekap for tolerance.
	poItemIDs := make([]int64, 0, len(in.Items))
	for _, it := range in.Items {
		poItemIDs = append(poItemIDs, it.POItemID)
	}
	rekap, err := getPOItemRekapTx(ctx, tx, poItemIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("get rekap: %w", err)
	}

	// Insert each lot.
	var lots []*domain.PenerimaanBahan
	for i, it := range in.Items {
		poItem, ok := poItemMap[it.POItemID]
		if !ok {
			return nil, nil, domain.ErrItemNotFound
		}

		// Tolerance check.
		existing := rekap[it.POItemID]
		if existing.QtyDiterima+it.QtyDiterima > poItem.qtyPesan*1.10 {
			return nil, nil, domain.ErrExceedsTolerance
		}

		maxUrut++
		kodeLot := fmt.Sprintf("%s%03d", kodeLotPrefix, maxUrut)

		// Nilai potong nota.
		nilaiPotongNota := round2(it.QtyDitolak * poItem.hargaSatuan)

		// Insert penerimaan_bahan.
		const insertQ = `
			INSERT INTO penerimaan_bahan
				(po_item_id, kode_lot, waktu_terima, qty_diterima, qty_ditolak,
				 alasan_tolak, suhu_terima, kondisi, tgl_kedaluwarsa, foto_url,
				 diperiksa_oleh, nilai_potong_nota, sppg_id, client_ref,
				 lolos_suhu, catatan)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
			RETURNING ` + penerimaanColumnsInsert
		var alasanTolak any = nullableStr(it.AlasanTolak)
		var suhuTerima any = nullableFloat(it.SuhuTerima)
		var tglKedaluwarsa any
		if it.TglKedaluwarsa != nil {
			tglKedaluwarsa = *it.TglKedaluwarsa
		} else {
			tglKedaluwarsa = nil
		}
		var lolosSuhu any = nullableBool(it.LolosSuhu)
		var catatan any = nullableStr(it.Catatan)

		row := tx.QueryRow(ctx, insertQ,
			it.POItemID, kodeLot, at, it.QtyDiterima, it.QtyDitolak,
			alasanTolak, suhuTerima, it.Kondisi, tglKedaluwarsa, strconv.FormatInt(it.FotoBerkasID, 10),
			diperiksaOleh, nilaiPotongNota, sppgID, in.ClientRef,
			lolosSuhu, catatan,
		)
		lot, err := scanPenerimaan(row)
		if err != nil {
			if isUniqueViolation(err) {
				return nil, nil, domain.ErrKodeLotConflict
			}
			return nil, nil, fmt.Errorf("insert penerimaan_bahan: %w", err)
		}
		lots = append(lots, lot)

		// Mark berkas as used (ref_tabel='penerimaan_bahan', ref_id=new lot id).
		const markBerkasQ = `
			UPDATE berkas
			SET ref_tabel='penerimaan_bahan', ref_id=$2, dipakai_at=$3
			WHERE id=$1 AND dipakai_at IS NULL`
		tag, err := tx.Exec(ctx, markBerkasQ, it.FotoBerkasID, lot.ID, at)
		if err != nil {
			return nil, nil, fmt.Errorf("mark berkas used: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil, nil, domain.ErrBerkasInvalid
		}
		_ = i
	}

	// Recalculate PO status: all items terpenuhi -> selesai, else -> diterima_sebagian.
	// Get all items' rekap to determine if all are terpenuhi.
	allItemRows, err := tx.Query(ctx, `SELECT id FROM po_item WHERE po_id=$1`, poID)
	if err != nil {
		return nil, nil, fmt.Errorf("list all po items: %w", err)
	}
	var allItemIDs []int64
	for allItemRows.Next() {
		var id int64
		if err := allItemRows.Scan(&id); err != nil {
			allItemRows.Close()
			return nil, nil, fmt.Errorf("scan all po items: %w", err)
		}
		allItemIDs = append(allItemIDs, id)
	}
	allItemRows.Close()

	allRekap, err := getPOItemRekapTx(ctx, tx, allItemIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("get all rekap: %w", err)
	}

	allTerpenuhi := true
	for _, id := range allItemIDs {
		r := allRekap[id]
		if !r.Terpenuhi {
			allTerpenuhi = false
			break
		}
	}

	var newStatus string
	if allTerpenuhi {
		newStatus = domain.POStatusSelesai
	} else {
		newStatus = domain.POStatusDiterimaSebagian
	}

	const statusQ = `UPDATE purchase_order SET status=$1::po_status, updated_at=now()
		WHERE id=$2 AND status IN ($3::po_status, $4::po_status) RETURNING ` + poColumns
	updatedPO, err := scanPO(tx.QueryRow(ctx, statusQ, newStatus, poID, domain.POStatusDikirim, domain.POStatusDiterimaSebagian))
	if err != nil {
		if isNoRows(err) {
			return nil, nil, domain.ErrPOInvalidStatus
		}
		return nil, nil, fmt.Errorf("update po status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit penerimaan: %w", err)
	}
	return lots, updatedPO, nil
}

// List returns filtered receipt lots for the SPPG.
func (p *PenerimaanPostgres) List(ctx context.Context, sppgID int64, filter domain.PenerimaanFilter) ([]domain.PenerimaanListItem, int, error) {
	where := ` WHERE pb.sppg_id = $1`
	args := []any{sppgID}
	argIdx := 2

	if filter.BahanID != nil {
		where += fmt.Sprintf(" AND pi.bahan_id = $%d", argIdx)
		args = append(args, *filter.BahanID)
		argIdx++
	}
	if filter.PemasokID != nil {
		where += fmt.Sprintf(" AND po.pemasok_id = $%d", argIdx)
		args = append(args, *filter.PemasokID)
		argIdx++
	}
	if filter.POID != nil {
		where += fmt.Sprintf(" AND po.id = $%d", argIdx)
		args = append(args, *filter.POID)
		argIdx++
	}
	if filter.Kondisi != nil {
		where += fmt.Sprintf(" AND pb.kondisi = $%d", argIdx)
		args = append(args, *filter.Kondisi)
		argIdx++
	}
	if filter.KodeLot != nil {
		where += fmt.Sprintf(" AND pb.kode_lot = $%d", argIdx)
		args = append(args, *filter.KodeLot)
		argIdx++
	}
	if filter.Dari != nil {
		where += fmt.Sprintf(" AND pb.waktu_terima::date >= $%d", argIdx)
		args = append(args, *filter.Dari)
		argIdx++
	}
	if filter.Sampai != nil {
		where += fmt.Sprintf(" AND pb.waktu_terima::date <= $%d", argIdx)
		args = append(args, *filter.Sampai)
		argIdx++
	}
	if filter.Tersedia != nil && *filter.Tersedia {
		// tersedia: qty_sisa > 0, kondisi != rusak, tgl_kedaluwarsa null or >= today
		where += " AND (pb.qty_diterima - COALESCE((SELECT SUM(bb.qty_dipakai) FROM batch_bahan bb WHERE bb.penerimaan_id = pb.id), 0)) > 0"
		where += " AND pb.kondisi <> 'rusak'"
		where += " AND (pb.tgl_kedaluwarsa IS NULL OR pb.tgl_kedaluwarsa >= CURRENT_DATE)"
	}

	countQ := `SELECT COUNT(*) FROM penerimaan_bahan pb
		JOIN po_item pi ON pi.id = pb.po_item_id
		JOIN purchase_order po ON po.id = pi.po_id` + where
	var total int
	if err := p.Pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count penerimaan: %w", err)
	}

	// FEFO order: tgl_kedaluwarsa ASC (nulls last), waktu_terima ASC.
	orderBy := `ORDER BY (pb.tgl_kedaluwarsa IS NULL), pb.tgl_kedaluwarsa ASC, pb.waktu_terima ASC`
	listQ := `
		SELECT pb.id, pb.kode_lot, po.nomor_po, p.nama as pemasok_nama,
			b.id as bahan_id, b.nama as bahan_nama, b.satuan as bahan_satuan, b.mudah_rusak as bahan_mudah_rusak,
			pb.waktu_terima, pb.qty_diterima, pb.qty_ditolak,
			COALESCE((SELECT SUM(bb.qty_dipakai) FROM batch_bahan bb WHERE bb.penerimaan_id = pb.id), 0) as qty_terpakai,
			(pb.qty_diterima - COALESCE((SELECT SUM(bb.qty_dipakai) FROM batch_bahan bb WHERE bb.penerimaan_id = pb.id), 0)) as qty_sisa,
			pb.kondisi, pb.suhu_terima, pb.lolos_suhu, pb.tgl_kedaluwarsa, pb.nilai_potong_nota
		FROM penerimaan_bahan pb
		JOIN po_item pi ON pi.id = pb.po_item_id
		JOIN purchase_order po ON po.id = pi.po_id
		JOIN pemasok p ON p.id = po.pemasok_id
		JOIN bahan b ON b.id = pi.bahan_id` + where + ` ` + orderBy +
		` LIMIT $` + fmt.Sprintf("%d", argIdx) + ` OFFSET $` + fmt.Sprintf("%d", argIdx+1)
	args = append(args, filter.Limit, filter.Offset())

	rows, err := p.Pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list penerimaan: %w", err)
	}
	defer rows.Close()

	var out []domain.PenerimaanListItem
	for rows.Next() {
		var item domain.PenerimaanListItem
		var suhuTerima sql.NullFloat64
		var lolosSuhu sql.NullBool
		var tglKedaluwarsa sql.NullTime
		if err := rows.Scan(&item.ID, &item.KodeLot, &item.NomorPO, &item.PemasokNama,
			&item.BahanID, &item.BahanNama, &item.BahanSatuan, &item.BahanMudahRusak,
			&item.WaktuTerima, &item.QtyDiterima, &item.QtyDitolak,
			&item.QtyTerpakai, &item.QtySisa, &item.Kondisi,
			&suhuTerima, &lolosSuhu, &tglKedaluwarsa, &item.NilaiPotongNota); err != nil {
			return nil, 0, fmt.Errorf("scan penerimaan item: %w", err)
		}
		if suhuTerima.Valid {
			v := suhuTerima.Float64
			item.SuhuTerima = &v
		}
		if lolosSuhu.Valid {
			item.LolosSuhu = &lolosSuhu.Bool
		}
		if tglKedaluwarsa.Valid {
			item.TglKedaluwarsa = &tglKedaluwarsa.Time
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate penerimaan: %w", err)
	}
	if out == nil {
		out = []domain.PenerimaanListItem{}
	}
	return out, total, nil
}

// GetByID returns one receipt lot with full detail.
func (p *PenerimaanPostgres) GetByID(ctx context.Context, id int64) (*domain.PenerimaanDetail, error) {
	return getPenerimaanDetail(ctx, p.Pool, id)
}

func getPenerimaanDetail(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id int64) (*domain.PenerimaanDetail, error) {
	const q = `
		SELECT pb.id, pb.po_item_id, po.id as po_id, po.nomor_po, b.id as bahan_id, b.nama as bahan_nama, b.satuan as bahan_satuan,
			pb.kode_lot, pb.waktu_terima, pb.qty_diterima, pb.qty_ditolak, pb.alasan_tolak,
			pb.suhu_terima, pb.kondisi, pb.tgl_kedaluwarsa, pb.foto_url, pb.diperiksa_oleh,
			u.nama as diperiksa_nama, pb.nilai_potong_nota, pb.sppg_id, pb.client_ref,
			pb.catatan, pb.dikoreksi_oleh, pb.dikoreksi_at, pb.created_at,
			po.status as po_status, pemasok.id as pemasok_id, pemasok.nama as pemasok_nama, pemasok.no_hp as pemasok_no_hp,
			COALESCE((SELECT SUM(bb.qty_dipakai) FROM batch_bahan bb WHERE bb.penerimaan_id = pb.id), 0) as batch_qty_dipakai
		FROM penerimaan_bahan pb
		JOIN po_item pi ON pi.id = pb.po_item_id
		JOIN purchase_order po ON po.id = pi.po_id
		JOIN bahan b ON b.id = pi.bahan_id
		JOIN users u ON u.id = pb.diperiksa_oleh
		JOIN pemasok ON pemasok.id = po.pemasok_id
		WHERE pb.id = $1`
	var d domain.PenerimaanDetail
	var suhuTerima sql.NullFloat64
	var tglKedaluwarsa sql.NullTime
	var alasanTolak, catatan sql.NullString
	var dikoreksiOleh sql.NullInt64
	var dikoreksiAt sql.NullTime
	var lolosSuhu sql.NullBool
	var fotoURL sql.NullString
	var diperiksaNama sql.NullString
	if err := db.QueryRow(ctx, q, id).Scan(
		&d.ID, &d.POItemID, &d.POID, &d.NomorPO, &d.BahanID, &d.BahanNama, &d.BahanSatuan,
		&d.KodeLot, &d.WaktuTerima, &d.QtyDiterima, &d.QtyDitolak, &alasanTolak,
		&suhuTerima, &d.Kondisi, &tglKedaluwarsa, &fotoURL, &d.DiperiksaOleh,
		&diperiksaNama, &d.NilaiPotongNota, &d.SPPGID, &d.ClientRef,
		&catatan, &dikoreksiOleh, &dikoreksiAt, &d.CreatedAt,
		&d.POStatus, &d.PemasokID, &d.PemasokNama, &d.PemasokNoHP,
		&d.BatchQtyDipakai); err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get penerimaan detail: %w", err)
	}
	if alasanTolak.Valid {
		d.AlasanTolak = &alasanTolak.String
	}
	if suhuTerima.Valid {
		v := suhuTerima.Float64
		d.SuhuTerima = &v
	}
	if tglKedaluwarsa.Valid {
		d.TglKedaluwarsa = &tglKedaluwarsa.Time
	}
	if fotoURL.Valid {
		d.FotoURL = fotoURL.String
	}
	if diperiksaNama.Valid {
		d.DiperiksaNama = diperiksaNama.String
	}
	if catatan.Valid {
		d.Catatan = &catatan.String
	}
	if dikoreksiOleh.Valid {
		v := dikoreksiOleh.Int64
		d.DikoreksiOleh = &v
	}
	if dikoreksiAt.Valid {
		d.DikoreksiAt = &dikoreksiAt.Time
	}
	if lolosSuhu.Valid {
		d.LolosSuhu = &lolosSuhu.Bool
	}
	return &d, nil
}

// KoreksiPenerimaan corrects a receipt lot in a single transaction.
// Evidence fields (suhu_terima, lolos_suhu, foto_url, waktu_terima,
// diperiksa_oleh, kode_lot) are not changed. PO status is recalculated:
// all items terpenuhi → selesai, else → diterima_sebagian.
// A manually closed PO (ditutup) stays selesai.
func (p *PenerimaanPostgres) KoreksiPenerimaan(ctx context.Context, id int64, in domain.KoreksiPenerimaanInput, disetujuiOleh int64, at time.Time) (*domain.PenerimaanDetail, *domain.PurchaseOrder, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	return koreksiPenerimaanTx(ctx, tx, id, in, disetujuiOleh, at)
}

func koreksiPenerimaanTx(ctx context.Context, tx pgx.Tx, id int64, in domain.KoreksiPenerimaanInput, disetujuiOleh int64, at time.Time) (*domain.PenerimaanDetail, *domain.PurchaseOrder, error) {
	// Lock the lot row for update.
	var poID, poItemID int64
	var currentPOStatus string
	var ditutupOleh sql.NullInt64
	err := tx.QueryRow(ctx, `
		SELECT pb.po_item_id, pi.po_id, po.status, po.ditutup_oleh
		FROM penerimaan_bahan pb
		JOIN po_item pi ON pi.id = pb.po_item_id
		JOIN purchase_order po ON po.id = pi.po_id
		WHERE pb.id = $1
		FOR UPDATE`, id).Scan(&poItemID, &poID, &currentPOStatus, &ditutupOleh)
	if err != nil {
		if isNoRows(err) {
			return nil, nil, domain.ErrLotNotFound
		}
		return nil, nil, fmt.Errorf("lock lot: %w", err)
	}

	// Build the update. Only correctable fields change; evidence fields stay.
	const updateQ = `
		UPDATE penerimaan_bahan
		SET qty_diterima = $2, qty_ditolak = $3, alasan_tolak = $4,
			kondisi = $5, tgl_kedaluwarsa = $6, catatan = $7,
			nilai_potong_nota = $8, dikoreksi_oleh = $9, dikoreksi_at = $10
		WHERE id = $1
		RETURNING ` + penerimaanColumnsInsert

	var alasanTolak any = nullableStr(in.AlasanTolak)
	var tglKedaluwarsa any
	if in.TglKedaluwarsa != nil {
		t := *in.TglKedaluwarsa
		tglKedaluwarsa = t
	} else {
		tglKedaluwarsa = nil
	}
	var catatan any = nullableStr(in.Catatan)

	row := tx.QueryRow(ctx, updateQ,
		id, in.QtyDiterima, in.QtyDitolak, alasanTolak, in.Kondisi,
		tglKedaluwarsa, catatan, in.NilaiPotongNota, disetujuiOleh, at,
	)
	_, scanErr := scanPenerimaan(row)
	if scanErr != nil {
		return nil, nil, fmt.Errorf("update penerimaan_bahan: %w", scanErr)
	}

	// Recalculate PO status.
	// Get all items' rekap for this PO.
	allItemRows, err := tx.Query(ctx, `SELECT id FROM po_item WHERE po_id=$1`, poID)
	if err != nil {
		return nil, nil, fmt.Errorf("list all po items: %w", err)
	}
	var allItemIDs []int64
	for allItemRows.Next() {
		var itemID int64
		if err := allItemRows.Scan(&itemID); err != nil {
			allItemRows.Close()
			return nil, nil, fmt.Errorf("scan po item id: %w", err)
		}
		allItemIDs = append(allItemIDs, itemID)
	}
	allItemRows.Close()

	allRekap, err := getPOItemRekapTx(ctx, tx, allItemIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("get all rekap: %w", err)
	}

	allTerpenuhi := true
	for _, itemID := range allItemIDs {
		r := allRekap[itemID]
		if !r.Terpenuhi {
			allTerpenuhi = false
			break
		}
	}

	var newStatus string
	if allTerpenuhi {
		newStatus = domain.POStatusSelesai
	} else {
		newStatus = domain.POStatusDiterimaSebagian
	}

	// If PO was manually closed (ditutup_oleh set) and currently selesai,
	// it stays selesai even if koreksi makes items incomplete.
	if currentPOStatus == domain.POStatusSelesai && ditutupOleh.Valid {
		newStatus = domain.POStatusSelesai
	}

	// Only update status if it changed (avoid unnecessary write).
	var updatedPO *domain.PurchaseOrder
	if newStatus != currentPOStatus {
		const statusQ = `UPDATE purchase_order SET status=$1::po_status, updated_at=now()
			WHERE id=$2 RETURNING ` + poColumns
		updatedPO, err = scanPO(tx.QueryRow(ctx, statusQ, newStatus, poID))
		if err != nil {
			if isNoRows(err) {
				return nil, nil, domain.ErrPOInvalidStatus
			}
			return nil, nil, fmt.Errorf("update po status: %w", err)
		}
	} else {
		updatedPO, err = findPOSTx(ctx, tx, poID)
		if err != nil {
			return nil, nil, fmt.Errorf("get current po: %w", err)
		}
	}

	// Read the response inside the transaction so read failures roll back the
	// correction, and no queries use a transaction that has already committed.
	detail, err := getPenerimaanDetail(ctx, tx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get updated detail: %w", err)
	}
	if detail == nil {
		return nil, nil, domain.ErrLotNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit koreksi: %w", err)
	}
	return detail, updatedPO, nil
}

// Helpers used by PenerimaanPostgres

func findPOSTx(ctx context.Context, tx pgx.Tx, id int64) (*domain.PurchaseOrder, error) {
	row := tx.QueryRow(ctx, `SELECT `+poColumns+` FROM purchase_order WHERE id=$1 LIMIT 1`, id)
	return scanPO(row)
}

func getPOItemRekapTx(ctx context.Context, tx pgx.Tx, poItemIDs []int64) (map[int64]domain.POItemRekap, error) {
	out := make(map[int64]domain.POItemRekap)
	if len(poItemIDs) == 0 {
		return out, nil
	}
	const q = `SELECT po_item_id, qty_diterima, qty_ditolak, nilai_potong_nota, terpenuhi, nilai_tagihan FROM v_po_item_rekap WHERE po_item_id = ANY($1)`
	rows, err := tx.Query(ctx, q, poItemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r domain.POItemRekap
		var qtyDiterima, qtyDitolak, potong, tagihan sql.NullFloat64
		var terpenuhi sql.NullBool
		if err := rows.Scan(&r.POItemID, &qtyDiterima, &qtyDitolak, &potong, &terpenuhi, &tagihan); err != nil {
			return nil, err
		}
		if qtyDiterima.Valid {
			r.QtyDiterima = qtyDiterima.Float64
		}
		if qtyDitolak.Valid {
			r.QtyDitolak = qtyDitolak.Float64
		}
		if potong.Valid {
			r.NilaiPotongNota = potong.Float64
		}
		if terpenuhi.Valid {
			r.Terpenuhi = terpenuhi.Bool
		}
		if tagihan.Valid {
			r.NilaiTagihan = tagihan.Float64
		}
		out[r.POItemID] = r
	}
	return out, nil
}

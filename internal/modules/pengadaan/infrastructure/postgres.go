package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// PemasokPostgres implements PemasokRepository
type PemasokPostgres struct {
	Pool *pgxpool.Pool
}

func NewPemasokPostgres(pool *pgxpool.Pool) *PemasokPostgres {
	return &PemasokPostgres{Pool: pool}
}

const pemasokColumns = `id, sppg_id, nama, jenis_usaha, kontak, no_hp, alamat, aktif, created_at, updated_at`

func scanPemasok(row interface{ Scan(...any) error }) (*domain.Pemasok, error) {
	var p domain.Pemasok
	var kontak, alamat sql.NullString
	var created, updated sql.NullTime
	if err := row.Scan(&p.ID, &p.SPPGID, &p.Nama, &p.JenisUsaha, &kontak, &p.NoHP, &alamat, &p.Aktif, &created, &updated); err != nil {
		return nil, err
	}
	p.Kontak = kontak.String
	p.Alamat = alamat.String
	if created.Valid {
		p.CreatedAt = created.Time
	}
	if updated.Valid {
		p.UpdatedAt = updated.Time
	}
	return &p, nil
}

func (p *PemasokPostgres) Create(ctx context.Context, pemasok *domain.Pemasok) (*domain.Pemasok, error) {
	const q = `INSERT INTO pemasok (sppg_id, nama, jenis_usaha, kontak, no_hp, alamat, aktif, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,now(),now()) RETURNING ` + pemasokColumns
	row := p.Pool.QueryRow(ctx, q, pemasok.SPPGID, pemasok.Nama, pemasok.JenisUsaha,
		nullableStr(&pemasok.Kontak), pemasok.NoHP, nullableStr(&pemasok.Alamat), pemasok.Aktif)
	created, err := scanPemasok(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrPemasokExists
		}
		if isForeignKeyViolation(err) {
			return nil, domain.ErrSPPGNotFound
		}
		return nil, fmt.Errorf("insert pemasok: %w", err)
	}
	return created, nil
}

func (p *PemasokPostgres) FindByID(ctx context.Context, id int64) (*domain.Pemasok, error) {
	const q = `SELECT ` + pemasokColumns + ` FROM pemasok WHERE id=$1 LIMIT 1`
	row := p.Pool.QueryRow(ctx, q, id)
	res, err := scanPemasok(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find pemasok: %w", err)
	}
	return res, nil
}

func (p *PemasokPostgres) FindByNameInsensitive(ctx context.Context, sppgID int64, nama string) (*domain.Pemasok, error) {
	const q = `SELECT ` + pemasokColumns + ` FROM pemasok WHERE sppg_id=$1 AND lower(nama)=lower($2) LIMIT 1`
	row := p.Pool.QueryRow(ctx, q, sppgID, nama)
	res, err := scanPemasok(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find pemasok by name: %w", err)
	}
	return res, nil
}

func (p *PemasokPostgres) Update(ctx context.Context, pemasok *domain.Pemasok) (*domain.Pemasok, error) {
	const q = `UPDATE pemasok SET nama=$2, jenis_usaha=$3, kontak=$4, no_hp=$5, alamat=$6, aktif=$7, updated_at=now()
		WHERE id=$1 RETURNING ` + pemasokColumns
	row := p.Pool.QueryRow(ctx, q, pemasok.ID, pemasok.Nama, pemasok.JenisUsaha,
		nullableStr(&pemasok.Kontak), pemasok.NoHP, nullableStr(&pemasok.Alamat), pemasok.Aktif)
	updated, err := scanPemasok(row)
	if err != nil {
		if isNoRows(err) {
			return nil, domain.ErrPemasokNotFound
		}
		if isUniqueViolation(err) {
			return nil, domain.ErrPemasokExists
		}
		return nil, fmt.Errorf("update pemasok: %w", err)
	}
	return updated, nil
}

func (p *PemasokPostgres) List(ctx context.Context, sppgID int64, filter domain.PemasokFilter) ([]domain.Pemasok, int, error) {
	filter.Normalize()
	where := `WHERE ($1::bigint = 0 OR sppg_id=$1) AND ($2::text IS NULL OR lower(nama) LIKE '%'||lower($2)||'%') AND ($3::text IS NULL OR jenis_usaha=$3) AND ($4::boolean IS NULL OR aktif=$4)`
	var qVal, jenisVal, aktifVal any
	if filter.Q != nil && *filter.Q != "" {
		qVal = *filter.Q
	}
	if filter.JenisUsaha != nil {
		jenisVal = *filter.JenisUsaha
	}
	if filter.Aktif != nil {
		aktifVal = *filter.Aktif
	}
	countQ := `SELECT COUNT(*) FROM pemasok ` + where
	var total int
	if err := p.Pool.QueryRow(ctx, countQ, sppgID, qVal, jenisVal, aktifVal).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count pemasok: %w", err)
	}
	listQ := `SELECT ` + pemasokColumns + ` FROM pemasok ` + where + ` ORDER BY nama ASC LIMIT $5 OFFSET $6`
	rows, err := p.Pool.Query(ctx, listQ, sppgID, qVal, jenisVal, aktifVal, filter.Limit, filter.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list pemasok: %w", err)
	}
	defer rows.Close()
	var out []domain.Pemasok
	for rows.Next() {
		var pm domain.Pemasok
		var kontak, alamat sql.NullString
		var created, updated sql.NullTime
		if err := rows.Scan(&pm.ID, &pm.SPPGID, &pm.Nama, &pm.JenisUsaha, &kontak, &pm.NoHP, &alamat, &pm.Aktif, &created, &updated); err != nil {
			return nil, 0, fmt.Errorf("scan pemasok: %w", err)
		}
		pm.Kontak = kontak.String
		pm.Alamat = alamat.String
		if created.Valid {
			pm.CreatedAt = created.Time
		}
		if updated.Valid {
			pm.UpdatedAt = updated.Time
		}
		out = append(out, pm)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows pemasok: %w", err)
	}
	if out == nil {
		out = []domain.Pemasok{}
	}
	return out, total, nil
}

func (p *PemasokPostgres) GetKinerja(ctx context.Context, sppgID, pemasokID int64) (*domain.Kinerja, error) {
	now := time.Now()
	cutoff := now.AddDate(0, 0, -90)

	const q = `
		WITH po_filtered AS (
			SELECT po.id, po.total, po.tgl_kirim_diminta, po.status
			FROM purchase_order po
			WHERE po.pemasok_id = $1 AND po.sppg_id = $2
				AND po.status NOT IN ('draf', 'batal')
				AND po.created_at >= $3
		),
		receipts AS (
			SELECT po.id AS po_id,
			       COALESCE(SUM(pb.nilai_potong_nota), 0) AS potong_nota,
			       COALESCE(SUM((pb.qty_diterima + pb.qty_ditolak) * pi.harga_satuan), 0) AS nilai_kedatangan,
			       MIN(pb.waktu_terima)::date AS first_receipt_date
			FROM purchase_order po
			JOIN po_item pi ON pi.po_id = po.id
			LEFT JOIN penerimaan_bahan pb ON pb.po_item_id = pi.id
			WHERE po.pemasok_id = $1 AND po.sppg_id = $2
				AND po.status NOT IN ('draf', 'batal')
			GROUP BY po.id
		)
		SELECT COUNT(DISTINCT po.id) AS jumlah_po,
		       COALESCE(SUM(po.total), 0) AS total_nilai_po,
		       COALESCE(SUM(r.potong_nota), 0) AS total_potong_nota,
		       CASE WHEN SUM(r.nilai_kedatangan) > 0
		            THEN ROUND(SUM(r.potong_nota) / SUM(r.nilai_kedatangan) * 100, 2)
		            ELSE 0 END AS persen_ditolak,
		       CASE WHEN COUNT(DISTINCT r.po_id) FILTER (WHERE r.first_receipt_date IS NOT NULL) > 0
		            THEN ROUND(COUNT(DISTINCT r.po_id) FILTER (WHERE r.first_receipt_date <= po.tgl_kirim_diminta)::numeric /
		                          COUNT(DISTINCT r.po_id) FILTER (WHERE r.first_receipt_date IS NOT NULL) * 100, 2)
		            ELSE 0 END AS persen_tepat_waktu
		FROM po_filtered po
		LEFT JOIN receipts r ON r.po_id = po.id
	`
	var k domain.Kinerja
	var totalNilai, totalPotong sql.NullFloat64
	var persenDitolak, persenTepat sql.NullFloat64
	err := p.Pool.QueryRow(ctx, q, pemasokID, sppgID, cutoff).Scan(
		&k.JumlahPO, &totalNilai, &totalPotong, &persenDitolak, &persenTepat,
	)
	if err != nil {
		return nil, fmt.Errorf("get kinerja: %w", err)
	}
	k.PeriodeHari = 90
	if totalNilai.Valid {
		k.TotalNilaiPO = domain.FormatMoney(totalNilai.Float64)
	} else {
		k.TotalNilaiPO = "0.00"
	}
	if totalPotong.Valid {
		k.TotalPotongNota = domain.FormatMoney(totalPotong.Float64)
	} else {
		k.TotalPotongNota = "0.00"
	}
	if persenDitolak.Valid {
		k.PersenDitolak = persenDitolak.Float64
	}
	if persenTepat.Valid {
		k.PersenTepatWaktu = persenTepat.Float64
	}
	return &k, nil
}

// BahanPostgres implements BahanRepository
type BahanPostgres struct {
	Pool *pgxpool.Pool
}

func NewBahanPostgres(pool *pgxpool.Pool) *BahanPostgres {
	return &BahanPostgres{Pool: pool}
}

const bahanColumns = `id, nama, kategori, satuan, gram_per_satuan, mudah_rusak, suhu_simpan_maks, aktif, created_at, updated_at`

func scanBahan(row interface{ Scan(...any) error }) (*domain.Bahan, error) {
	var b domain.Bahan
	var suhu sql.NullFloat64
	var created, updated sql.NullTime
	if err := row.Scan(&b.ID, &b.Nama, &b.Kategori, &b.Satuan, &b.GramPerSatuan, &b.MudahRusak, &suhu, &b.Aktif, &created, &updated); err != nil {
		return nil, err
	}
	if suhu.Valid {
		v := suhu.Float64
		b.SuhuSimpanMaks = &v
	}
	if created.Valid {
		b.CreatedAt = created.Time
	}
	if updated.Valid {
		b.UpdatedAt = updated.Time
	}
	return &b, nil
}

func (p *BahanPostgres) FindByID(ctx context.Context, id int64) (*domain.Bahan, error) {
	const q = `SELECT ` + bahanColumns + ` FROM bahan WHERE id=$1 LIMIT 1`
	row := p.Pool.QueryRow(ctx, q, id)
	b, err := scanBahan(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find bahan: %w", err)
	}
	return b, nil
}

func (p *BahanPostgres) List(ctx context.Context, filter domain.BahanFilter) ([]domain.Bahan, int, error) {
	filter.Normalize()
	where := `WHERE ($1::text IS NULL OR lower(nama) LIKE '%'||lower($1)||'%') AND ($2::text IS NULL OR kategori=$2) AND ($3::boolean IS NULL OR aktif=$3)`
	var qVal, katVal, aktifVal any
	if filter.Q != nil && *filter.Q != "" {
		qVal = *filter.Q
	}
	if filter.Kategori != nil {
		katVal = *filter.Kategori
	}
	if filter.Aktif != nil {
		aktifVal = *filter.Aktif
	}
	countQ := `SELECT COUNT(*) FROM bahan ` + where
	var total int
	if err := p.Pool.QueryRow(ctx, countQ, qVal, katVal, aktifVal).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count bahan: %w", err)
	}
	listQ := `SELECT ` + bahanColumns + ` FROM bahan ` + where + ` ORDER BY id ASC LIMIT $4 OFFSET $5`
	rows, err := p.Pool.Query(ctx, listQ, qVal, katVal, aktifVal, filter.Limit, filter.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list bahan: %w", err)
	}
	defer rows.Close()
	var out []domain.Bahan
	for rows.Next() {
		b, err := scanBahan(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan bahan: %w", err)
		}
		out = append(out, *b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows bahan: %w", err)
	}
	if out == nil {
		out = []domain.Bahan{}
	}
	return out, total, nil
}

// MenuPostgres implements MenuRepository
type MenuPostgres struct {
	Pool *pgxpool.Pool
}

func NewMenuPostgres(pool *pgxpool.Pool) *MenuPostgres {
	return &MenuPostgres{Pool: pool}
}

const menuColumns = `id, sppg_id, tanggal, nama_menu, energi_kkal, protein_g, karbohidrat_g, lemak_g, target_porsi, catatan, dibuat_oleh, status, disetujui_oleh, disetujui_at, created_at, updated_at`

func scanMenu(row interface{ Scan(...any) error }) (*domain.Menu, error) {
	var m domain.Menu
	var tanggal time.Time
	var catatan sql.NullString
	var disetujuiOleh sql.NullInt64
	var disetujuiAt sql.NullTime
	var created, updated sql.NullTime
	var energi, protein, karbo, lemak sql.NullFloat64
	var target sql.NullInt32
	var status sql.NullString
	if err := row.Scan(&m.ID, &m.SPPGID, &tanggal, &m.NamaMenu, &energi, &protein, &karbo, &lemak, &target, &catatan, &m.DibuatOleh, &status, &disetujuiOleh, &disetujuiAt, &created, &updated); err != nil {
		return nil, err
	}
	m.Tanggal = tanggal
	m.EnergiKkal = energi.Float64
	m.ProteinG = protein.Float64
	m.KarbohidratG = karbo.Float64
	m.LemakG = lemak.Float64
	m.TargetPorsi = int(target.Int32)
	if catatan.Valid {
		v := catatan.String
		m.Catatan = &v
	}
	m.Status = status.String
	if disetujuiOleh.Valid {
		v := disetujuiOleh.Int64
		m.DisetujuiOleh = &v
	}
	if disetujuiAt.Valid {
		v := disetujuiAt.Time
		m.DisetujuiAt = &v
	}
	if created.Valid {
		m.CreatedAt = created.Time
	}
	if updated.Valid {
		m.UpdatedAt = updated.Time
	}
	return &m, nil
}

func (p *MenuPostgres) ListByScope(ctx context.Context, sppgID *int64, dari, sampai time.Time, status *string) ([]domain.Menu, error) {
	const q = `SELECT ` + menuColumns + ` FROM menu WHERE ($1::bigint IS NULL OR sppg_id=$1) AND tanggal >= $2 AND tanggal <= $3 AND ($4::menu_status IS NULL OR status=$4::menu_status) ORDER BY tanggal ASC, id ASC`
	var statusVal any
	if status != nil {
		statusVal = *status
	}
	var sppgVal any
	if sppgID != nil {
		sppgVal = *sppgID
	}
	rows, err := p.Pool.Query(ctx, q, sppgVal, dari.Format("2006-01-02"), sampai.Format("2006-01-02"), statusVal)
	if err != nil {
		return nil, fmt.Errorf("list menus: %w", err)
	}
	defer rows.Close()
	var out []domain.Menu
	for rows.Next() {
		m, err := scanMenu(rows)
		if err != nil {
			return nil, fmt.Errorf("scan menu: %w", err)
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows menus: %w", err)
	}
	if out == nil {
		out = []domain.Menu{}
	}
	return out, nil
}

func (p *MenuPostgres) ListItems(ctx context.Context, menuIDs []int64) (map[int64][]domain.MenuBahan, error) {
	out := map[int64][]domain.MenuBahan{}
	for _, id := range menuIDs {
		out[id] = []domain.MenuBahan{}
	}
	if len(menuIDs) == 0 {
		return out, nil
	}
	const q = `SELECT mb.id, mb.menu_id, mb.bahan_id, b.nama, mb.gram_per_porsi FROM menu_bahan mb JOIN bahan b ON b.id=mb.bahan_id WHERE mb.menu_id = ANY($1) ORDER BY mb.id ASC`
	rows, err := p.Pool.Query(ctx, q, menuIDs)
	if err != nil {
		return nil, fmt.Errorf("list menu items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var it domain.MenuBahan
		if err := rows.Scan(&it.ID, &it.MenuID, &it.BahanID, &it.BahanNama, &it.GramPerPorsi); err != nil {
			return nil, fmt.Errorf("scan menu item: %w", err)
		}
		out[it.MenuID] = append(out[it.MenuID], it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows menu items: %w", err)
	}
	return out, nil
}

func (p *MenuPostgres) FindByID(ctx context.Context, id int64) (*domain.Menu, error) {
	const q = `SELECT ` + menuColumns + ` FROM menu WHERE id=$1 LIMIT 1`
	row := p.Pool.QueryRow(ctx, q, id)
	m, err := scanMenu(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find menu: %w", err)
	}
	return m, nil
}

// FindByIDs finds multiple menus by IDs.
func (p *MenuPostgres) FindByIDs(ctx context.Context, ids []int64) ([]*domain.Menu, error) {
	if len(ids) == 0 {
		return []*domain.Menu{}, nil
	}
	const q = `SELECT ` + menuColumns + ` FROM menu WHERE id = ANY($1)`
	rows, err := p.Pool.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("find menus by ids: %w", err)
	}
	defer rows.Close()
	var out []*domain.Menu
	for rows.Next() {
		m, err := scanMenu(rows)
		if err != nil {
			return nil, fmt.Errorf("scan menu: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows menus: %w", err)
	}
	return out, nil
}

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

// PenerimaanPostgres implements PenerimaanRepository.
type PenerimaanPostgres struct {
	Pool *pgxpool.Pool
}

func NewPenerimaanPostgres(pool *pgxpool.Pool) *PenerimaanPostgres {
	return &PenerimaanPostgres{Pool: pool}
}

const penerimaanColumns = `pb.id, pb.po_item_id, pb.kode_lot, pb.waktu_terima, pb.qty_diterima, pb.qty_ditolak,
	pb.alasan_tolak, pb.suhu_terima, pb.kondisi, pb.tgl_kedaluwarsa, pb.foto_url,
	pb.diperiksa_oleh, pb.nilai_potong_nota, pb.sppg_id, pb.client_ref, pb.lolos_suhu,
	pb.catatan, pb.dikoreksi_oleh, pb.dikoreksi_at, pb.created_at`

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
		WHERE pb.client_ref = $1 AND pi.po_id = $2`
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
			RETURNING ` + penerimaanColumns
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
			alasanTolak, suhuTerima, it.Kondisi, tglKedaluwarsa, it.FotoBerkasID,
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
	where := `WHERE pb.sppg_id = $1`
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
		where += fmt.Sprintf(" AND (pb.qty_diterima - COALESCE((SELECT SUM(bb.qty_dipakai) FROM batch_bahan bb WHERE bb.penerimaan_id = pb.id), 0)) > 0")
		where += fmt.Sprintf(" AND pb.kondisi <> 'rusak'")
		where += fmt.Sprintf(" AND (pb.tgl_kedaluwarsa IS NULL OR pb.tgl_kedaluwarsa >= CURRENT_DATE)")
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
	const q = `
		SELECT pb.id, pb.po_item_id, po.nomor_po, b.id as bahan_id, b.nama as bahan_nama, b.satuan as bahan_satuan,
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
	if err := p.Pool.QueryRow(ctx, q, id).Scan(
		&d.ID, &d.POItemID, &d.NomorPO, &d.BahanID, &d.BahanNama, &d.BahanSatuan,
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

func nullableFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableBool(p *bool) any {
	if p == nil {
		return nil
	}
	return *p
}

func parseYM(yyyymmdd string) int {
	if len(yyyymmdd) >= 6 {
		var y, m int
		fmt.Sscanf(yyyymmdd[:6], "%04d%02d", &y, &m)
		return y*100 + m
	}
	return 0
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// SPPGPostgres implements SPPGProvider
type SPPGPostgres struct {
	Pool *pgxpool.Pool
}

func NewSPPGPostgres(pool *pgxpool.Pool) *SPPGPostgres {
	return &SPPGPostgres{Pool: pool}
}

func (p *SPPGPostgres) GetKode(ctx context.Context, id int64) (string, error) {
	var kode string
	err := p.Pool.QueryRow(ctx, `SELECT kode FROM sppg WHERE id=$1`, id).Scan(&kode)
	if err != nil {
		if isNoRows(err) {
			return "", domain.ErrSPPGNotFound
		}
		return "", fmt.Errorf("get sppg kode: %w", err)
	}
	return kode, nil
}

// SiklusPostgres implements SiklusRepository
type SiklusPostgres struct {
	Pool *pgxpool.Pool
}

func NewSiklusPostgres(pool *pgxpool.Pool) *SiklusPostgres {
	return &SiklusPostgres{Pool: pool}
}

const siklusColumns = `id, sppg_id, periode_mulai, periode_selesai, dana_diajukan, dana_diterima, sisa_siklus_lalu, status, diajukan_at, dicairkan_at, dilaporkan_at, created_at, updated_at`

func scanSiklus(row interface{ Scan(...any) error }) (*domain.SiklusDana, error) {
	var s domain.SiklusDana
	var periodeMulai, periodeSelesai time.Time
	var danaDiterima sql.NullFloat64
	var diajukanAt, dicairkanAt, dilaporkanAt sql.NullTime
	var created, updated sql.NullTime
	var danaDiajukan, sisaSiklusLalu sql.NullFloat64
	var status sql.NullString
	if err := row.Scan(&s.ID, &s.SPPGID, &periodeMulai, &periodeSelesai, &danaDiajukan, &danaDiterima, &sisaSiklusLalu, &status, &diajukanAt, &dicairkanAt, &dilaporkanAt, &created, &updated); err != nil {
		return nil, err
	}
	s.PeriodeMulai = periodeMulai
	s.PeriodeSelesai = periodeSelesai
	if danaDiajukan.Valid {
		s.DanaDiajukan = danaDiajukan.Float64
	}
	if danaDiterima.Valid {
		v := danaDiterima.Float64
		s.DanaDiterima = &v
	}
	if sisaSiklusLalu.Valid {
		s.SisaSiklusLalu = sisaSiklusLalu.Float64
	}
	s.Status = status.String
	if diajukanAt.Valid {
		s.DiajukanAt = &diajukanAt.Time
	}
	if dicairkanAt.Valid {
		s.DicairkanAt = &dicairkanAt.Time
	}
	if dilaporkanAt.Valid {
		s.DilaporkanAt = &dilaporkanAt.Time
	}
	if created.Valid {
		s.CreatedAt = created.Time
	}
	if updated.Valid {
		s.UpdatedAt = updated.Time
	}
	return &s, nil
}

func (p *SiklusPostgres) FindByID(ctx context.Context, id int64) (*domain.SiklusDana, error) {
	const q = `SELECT ` + siklusColumns + ` FROM siklus_dana WHERE id=$1 LIMIT 1`
	row := p.Pool.QueryRow(ctx, q, id)
	res, err := scanSiklus(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find siklus: %w", err)
	}
	return res, nil
}

// AuditPostgres implements AuditRepository
type AuditPostgres struct {
	Pool *pgxpool.Pool
}

func NewAuditPostgres(pool *pgxpool.Pool) *AuditPostgres {
	return &AuditPostgres{Pool: pool}
}

func (p *AuditPostgres) Append(ctx context.Context, e *authdomain.AuditEntry) error {
	const q = `INSERT INTO audit_log (user_id, aksi, tabel, record_id, data_lama, data_baru, ip_address)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7::inet)`
	if _, err := p.Pool.Exec(ctx, q,
		nullableInt(e.UserID), e.Aksi, e.Tabel, nullableInt(e.RecordID),
		nullableStr(e.DataLama), nullableStr(e.DataBaru), nullableStr(e.IPAddress),
	); err != nil {
		return fmt.Errorf("append audit: %w", err)
	}
	return nil
}

// NotificationPostgres implements NotificationRepository
type NotificationPostgres struct {
	Pool *pgxpool.Pool
}

func NewNotificationPostgres(pool *pgxpool.Pool) *NotificationPostgres {
	return &NotificationPostgres{Pool: pool}
}

func (p *NotificationPostgres) NotifySPPGRoles(ctx context.Context, sppgID int64, roles []string, jenis, judul, isi, refTabel string, refID int64) error {
	const q = `INSERT INTO notifikasi (user_id, jenis, judul, isi, ref_tabel, ref_id)
		SELECT id, $3, $4, $5, $6, $7 FROM users WHERE sppg_id=$1 AND peran = ANY($2) AND aktif`
	if _, err := p.Pool.Exec(ctx, q, sppgID, roles, jenis, judul, isi, refTabel, refID); err != nil {
		return fmt.Errorf("notify sppg roles: %w", err)
	}
	return nil
}

// Helpers
func nullableInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key") || strings.Contains(msg, "already exists")
}

func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	// Check for pgx PgError with code 23503 (foreign_key_violation)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	// Fallback to string matching
	msg := err.Error()
	return strings.Contains(msg, "23503") || strings.Contains(msg, "foreign key") || strings.Contains(msg, "violates foreign key")
}

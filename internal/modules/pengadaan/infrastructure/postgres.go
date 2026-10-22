package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

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
	const q = `SELECT ` + menuColumns + ` FROM menu WHERE ($1::bigint IS NULL OR sppg_id=$1) AND tanggal >= $2 AND tanggal <= $3 AND ($4::text IS NULL OR status=$4) ORDER BY tanggal ASC, id ASC`
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
			placeholders[i] = fmt.Sprintf("$%d", i+2)
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
	if err == nil {
		return false
	}
	return err.Error() == "no rows in result set"
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key") || strings.Contains(msg, "already exists")
}

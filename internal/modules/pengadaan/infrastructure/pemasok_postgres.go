package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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

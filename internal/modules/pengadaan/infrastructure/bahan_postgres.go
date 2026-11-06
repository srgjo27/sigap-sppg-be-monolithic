package infrastructure

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

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

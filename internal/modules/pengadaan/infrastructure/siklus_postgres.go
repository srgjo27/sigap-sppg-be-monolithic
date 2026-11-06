package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

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

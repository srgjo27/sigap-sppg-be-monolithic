package infrastructure

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

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

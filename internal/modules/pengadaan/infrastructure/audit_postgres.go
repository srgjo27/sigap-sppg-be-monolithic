package infrastructure

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

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

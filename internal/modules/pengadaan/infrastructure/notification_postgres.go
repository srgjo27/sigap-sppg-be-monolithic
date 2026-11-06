package infrastructure

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

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

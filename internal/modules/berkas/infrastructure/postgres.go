package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/domain"
)

// BerkasPostgres implements application.BerkasRepository.
type BerkasPostgres struct {
	Pool *pgxpool.Pool
}

func NewBerkasPostgres(pool *pgxpool.Pool) *BerkasPostgres {
	return &BerkasPostgres{Pool: pool}
}

// Create inserts one berkas row (unclaimed: ref_tabel/ref_id/dipakai_at NULL)
// and sets the generated ID.
func (p *BerkasPostgres) Create(ctx context.Context, b *domain.Berkas) error {
	err := p.Pool.QueryRow(ctx, `
		INSERT INTO berkas
			(sppg_id, object_key, mime_type, ukuran_byte, sha256, konteks, diunggah_oleh, diunggah_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		b.SPPGID, b.ObjectKey, b.MimeType, b.UkuranByte, b.SHA256, b.Konteks, b.DiunggahOleh, b.DiunggahAt,
	).Scan(&b.ID)
	if err != nil {
		return fmt.Errorf("insert berkas: %w", err)
	}
	return nil
}

// UnusedBefore lists unclaimed berkas rows uploaded before the cutoff.
func (p *BerkasPostgres) UnusedBefore(ctx context.Context, cutoff time.Time) ([]domain.Berkas, error) {
	rows, err := p.Pool.Query(ctx, `
		SELECT id, sppg_id, object_key, mime_type, ukuran_byte, sha256, konteks, diunggah_oleh, diunggah_at
		FROM berkas
		WHERE dipakai_at IS NULL AND diunggah_at < $1`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("list unused berkas: %w", err)
	}
	defer rows.Close()

	var out []domain.Berkas
	for rows.Next() {
		var b domain.Berkas
		if err := rows.Scan(&b.ID, &b.SPPGID, &b.ObjectKey, &b.MimeType, &b.UkuranByte, &b.SHA256, &b.Konteks, &b.DiunggahOleh, &b.DiunggahAt); err != nil {
			return nil, fmt.Errorf("scan berkas: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate berkas: %w", err)
	}
	return out, nil
}

// DeleteUnusedBefore deletes unclaimed rows uploaded before the cutoff
// and returns their object keys (for object-store cleanup).
func (p *BerkasPostgres) DeleteUnusedBefore(ctx context.Context, cutoff time.Time) ([]string, error) {
	rows, err := p.Pool.Query(ctx, `
		DELETE FROM berkas
		WHERE dipakai_at IS NULL AND diunggah_at < $1
		RETURNING object_key`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("delete unused berkas: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan berkas key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate berkas keys: %w", err)
	}
	return keys, nil
}

// SPPGPostgres implements application.SPPGCodeProvider with a tiny
// self-contained lookup (keeps the berkas module decoupled from other
// modules' infrastructure types).
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
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrSPPGNotFound
		}
		return "", fmt.Errorf("get sppg kode: %w", err)
	}
	return kode, nil
}

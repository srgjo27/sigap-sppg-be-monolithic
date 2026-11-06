package application

import (
	"context"
	"crypto/rand"
	"io"
	"time"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/domain"
)

// ObjectStore is the S3-compatible object storage port.
type ObjectStore interface {
	// Put uploads the object with a known content length.
	Put(ctx context.Context, key string, r io.Reader, size int64) error
	// Presign issues a temporary GET URL for the object.
	Presign(ctx context.Context, key string, ttl time.Duration) (string, error)
	// Delete removes objects in bulk.
	Delete(ctx context.Context, keys []string) error
}

// BerkasRepository is the persistence port for the berkas table.
type BerkasRepository interface {
	// Create inserts one berkas row and sets its generated ID.
	Create(ctx context.Context, b *domain.Berkas) error
	// UnusedBefore lists berkas rows that were never claimed by any
	// endpoint and were uploaded before the cutoff.
	UnusedBefore(ctx context.Context, cutoff time.Time) ([]domain.Berkas, error)
	// DeleteUnusedBefore deletes (in the DB) unclaimed rows uploaded
	// before the cutoff and returns their object keys.
	DeleteUnusedBefore(ctx context.Context, cutoff time.Time) ([]string, error)
}

// SPPGCodeProvider resolves the kode used in object storage keys.
type SPPGCodeProvider interface {
	GetKode(ctx context.Context, sppgID int64) (string, error)
}

// Service orchestrates berkas use cases.
type Service struct {
	store ObjectStore
	repo  BerkasRepository
	sppg  SPPGCodeProvider
	now   func() time.Time
	rand  io.Reader
}

// Deps wires the service without a framework.
type Deps struct {
	Store ObjectStore
	Repo  BerkasRepository
	SPPG  SPPGCodeProvider
	// Now is the injectable server clock (bukti waktu, not client time).
	Now func() time.Time
	// Rand is the random source for object key suffixes; nil = crypto/rand.
	Rand io.Reader
}

// New builds a Service with sensible defaults.
func New(d Deps) *Service {
	now := d.Now
	if now == nil {
		now = time.Now
	}
	randReader := d.Rand
	if randReader == nil {
		randReader = rand.Reader
	}
	return &Service{store: d.Store, repo: d.Repo, sppg: d.SPPG, now: now, rand: randReader}
}

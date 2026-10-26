package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/domain"
)

// UploadResult is returned by Service.Upload.
type UploadResult struct {
	BerkasID   int64
	Konteks    string
	MimeType   string
	UkuranByte int
	SHA256     string
	DiunggahAt time.Time
	URL        string // presigned GET, valid for domain.PresignTTL
}

// Upload implements POST /api/v1/uploads.
// Order: konteks validate -> role allow -> empty/size -> detect type
// (magic bytes) -> strip EXIF -> sha256 -> object key -> store -> DB ->
// presign. The route-level middleware already blocks pengawas; the
// role check here is defense in depth.
func (s *Service) Upload(ctx context.Context, claims *authdomain.Claims, konteks string, payload []byte) (*UploadResult, error) {
	if claims == nil {
		return nil, domain.ErrUnauthorized
	}
	if !domain.ValidateKonteks(konteks) {
		return nil, domain.ErrKonteksUnknown
	}
	claimsRole := claims.Role
	if !domain.RoleAllowed(konteks, claimsRole) {
		return nil, domain.ErrKonteksForbidden
	}
	// pic_sekolah has no SPPG scoping in claims but object keys need one.
	if claims.SPPGID == nil {
		return nil, domain.ErrKonteksForbidden
	}
	if len(payload) == 0 {
		return nil, domain.ErrFileEmpty
	}
	if len(payload) > domain.MaxUploadBytes {
		return nil, domain.ErrFileTooLarge
	}

	mimeType, ext, ok := domain.DetectImageType(payload)
	if !ok {
		return nil, domain.ErrUnsupportedImage
	}
	stripped, err := domain.StripImageMetadata(payload, mimeType)
	if err != nil {
		// Structurally invalid bytes must never be stored.
		return nil, domain.ErrUnsupportedImage
	}
	sum := sha256.Sum256(stripped)
	sha := hex.EncodeToString(sum[:])

	kodeSPPG, err := s.sppg.GetKode(ctx, *claims.SPPGID)
	if err != nil {
		return nil, fmt.Errorf("resolve sppg kode: %w", err)
	}

	key := domain.BuildObjectKey(kodeSPPG, konteks, ext, s.now(), s.rand)
	if err := s.store.Put(ctx, key, bytes.NewReader(stripped), int64(len(stripped))); err != nil {
		return nil, fmt.Errorf("store object: %w", err)
	}

	uploadedAt := s.now()
	b := &domain.Berkas{
		SPPGID:       *claims.SPPGID,
		ObjectKey:    key,
		MimeType:     mimeType,
		UkuranByte:   len(stripped),
		SHA256:       sha,
		Konteks:      konteks,
		DiunggahOleh: claims.UserID,
		DiunggahAt:   uploadedAt,
	}
	if err := s.repo.Create(ctx, b); err != nil {
		return nil, fmt.Errorf("save berkas: %w", err)
	}

	url, err := s.store.Presign(ctx, key, domain.PresignTTL)
	if err != nil {
		return nil, fmt.Errorf("presign berkas: %w", err)
	}

	return &UploadResult{
		BerkasID:   b.ID,
		Konteks:    konteks,
		MimeType:   mimeType,
		UkuranByte: len(stripped),
		SHA256:     sha,
		DiunggahAt: uploadedAt,
		URL:        url,
	}, nil
}

// PurgeUnused deletes DB rows and stored objects that were never
// claimed by another endpoint before the cutoff (hourly job, 24h).
func (s *Service) PurgeUnused(ctx context.Context, olderThan time.Duration) error {
	cutoff := s.now().Add(-olderThan)
	unused, err := s.repo.UnusedBefore(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("list unused berkas: %w", err)
	}
	if len(unused) == 0 {
		return nil
	}
	keys := make([]string, 0, len(unused))
	for _, b := range unused {
		keys = append(keys, b.ObjectKey)
	}
	if err := s.store.Delete(ctx, keys); err != nil {
		return fmt.Errorf("delete_objects: %w", err)
	}
	if _, err := s.repo.DeleteUnusedBefore(ctx, cutoff); err != nil {
		return fmt.Errorf("delete unused berkas rows: %w", err)
	}
	return nil
}

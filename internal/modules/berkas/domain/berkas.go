package domain

import (
	"encoding/hex"
	"fmt"
	"io"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// Konteks upload (lihat docs/product/mvp-003-pengadaan-penerimaan.md).
const (
	KonteksPenerimaanBahan  = "penerimaan_bahan"
	KonteksChecklistSOP     = "checklist_sop"
	KonteksPengiriman       = "pengiriman"
	KonteksKonfirmasiTerima = "konfirmasi_terima"
	KonteksPengaduan        = "pengaduan"
)

// Berkas mirrors the berkas table in db/schema.sql.
type Berkas struct {
	ID           int64
	SPPGID       int64
	ObjectKey    string    // unique path in object storage
	MimeType     string    // image/jpeg, image/png, image/webp
	UkuranByte   int       // 1..5242880
	SHA256       string    // hex, of the stripped bytes actually stored
	Konteks      string    // penerimaan_bahan, checklist_sop, pengiriman, ...
	DiunggahOleh int64     // users.id
	DiunggahAt   time.Time // server time, not client time

	// Usage binding: NULL until another endpoint claims the berkas.
	// Invariant from schema: (dipakai_at IS NULL) = (ref_id IS NULL).
	RefTabel  *string
	RefID     *int64
	DipakaiAt *time.Time
}

// KonteksRoles maps each upload konteks to the roles allowed to use it
// (docs/product/mvp-003-pengadaan-penerimaan.md). Pengawas is
// denied everywhere: it is blocked route-level by PengawasReadOnly and
// intentionally absent from every allowlist.
var KonteksRoles = map[string][]authdomain.Role{
	KonteksPenerimaanBahan:  {authdomain.RoleAkuntan, authdomain.RolePetugasDapur},
	KonteksChecklistSOP:     {authdomain.RolePetugasDapur, authdomain.RoleAhliGizi, authdomain.RoleKepalaSPPG},
	KonteksPengiriman:       {authdomain.RolePetugasDistribusi},
	KonteksKonfirmasiTerima: {authdomain.RolePICsekolah},
	KonteksPengaduan:        {authdomain.RolePICsekolah},
}

// RoleAllowed reports whether a role may upload under a konteks.
func RoleAllowed(konteks string, role authdomain.Role) bool {
	for _, r := range KonteksRoles[konteks] {
		if r == role {
			return true
		}
	}
	return false
}

// ValidateKonteks reports whether konteks is a known upload context.
func ValidateKonteks(konteks string) bool {
	_, ok := KonteksRoles[konteks]
	return ok
}

// BuildObjectKey builds the object storage key for one berkas:
// {kode_sppg}/{konteks}/{YYYY}/{MM}/{DD}/{32-hex-random}.{ext}.
// randReader must yield at least 16 bytes (crypto/rand in production);
// it is injected so tests can produce deterministic keys.
func BuildObjectKey(kodeSPPG, konteks, ext string, t time.Time, randReader io.Reader) string {
	buf := make([]byte, 16) // 128-bit random suffix (uuid-like, no uuid dep)
	if _, err := io.ReadFull(randReader, buf); err != nil {
		// crypto/rand only fails here on a broken OS entropy source;
		// fall back to a time-derived suffix so callers still get a
		// shaped key (uniqueness is enforced by object_key UNIQUE).
		buf = []byte(fmt.Sprintf("%016X", t.UnixNano()))
	}
	suffix := hex.EncodeToString(buf) // 32 lowercase hex chars
	return fmt.Sprintf("%s/%s/%04d/%02d/%02d/%s.%s",
		kodeSPPG, konteks, t.Year(), int(t.Month()), t.Day(), suffix, ext)
}

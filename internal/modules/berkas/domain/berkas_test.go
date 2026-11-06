package domain

import (
	"strings"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

func TestRoleAllowedMatrix(t *testing.T) {
	cases := []struct {
		konteks string
		role    authdomain.Role
		want    bool
	}{
		// penerimaan_bahan: akuntan, petugas_dapur
		{KonteksPenerimaanBahan, authdomain.RoleAkuntan, true},
		{KonteksPenerimaanBahan, authdomain.RolePetugasDapur, true},
		{KonteksPenerimaanBahan, authdomain.RoleKepalaSPPG, false},
		{KonteksPenerimaanBahan, authdomain.RoleAhliGizi, false},
		{KonteksPenerimaanBahan, authdomain.RolePICsekolah, false},
		// checklist_sop: petugas_dapur, ahli_gizi, kepala_sppg
		{KonteksChecklistSOP, authdomain.RolePetugasDapur, true},
		{KonteksChecklistSOP, authdomain.RoleAhliGizi, true},
		{KonteksChecklistSOP, authdomain.RoleKepalaSPPG, true},
		{KonteksChecklistSOP, authdomain.RoleAkuntan, false},
		// pengiriman: petugas_distribusi only
		{KonteksPengiriman, authdomain.RolePetugasDistribusi, true},
		{KonteksPengiriman, authdomain.RoleAkuntan, false},
		{KonteksPengiriman, authdomain.RoleKepalaSPPG, false},
		// konfirmasi_terima & pengaduan: pic_sekolah only
		{KonteksKonfirmasiTerima, authdomain.RolePICsekolah, true},
		{KonteksKonfirmasiTerima, authdomain.RoleAkuntan, false},
		{KonteksPengaduan, authdomain.RolePICsekolah, true},
		{KonteksPengaduan, authdomain.RolePetugasDapur, false},
		// pengawas denied everywhere
		{KonteksPenerimaanBahan, authdomain.RolePengawas, false},
		{KonteksChecklistSOP, authdomain.RolePengawas, false},
		{KonteksPengiriman, authdomain.RolePengawas, false},
		{KonteksKonfirmasiTerima, authdomain.RolePengawas, false},
		{KonteksPengaduan, authdomain.RolePengawas, false},
		// admin ditolak: menulis bukti bukan kewenangan admin per
		{KonteksPenerimaanBahan, authdomain.RoleAdmin, false},
		{KonteksChecklistSOP, authdomain.RoleAdmin, false},
	}
	for _, tc := range cases {
		if got := RoleAllowed(tc.konteks, tc.role); got != tc.want {
			t.Fatalf("RoleAllowed(%q, %q) = %v, want %v", tc.konteks, tc.role, got, tc.want)
		}
	}
}

func TestValidateKonteks(t *testing.T) {
	for _, k := range []string{
		KonteksPenerimaanBahan, KonteksChecklistSOP, KonteksPengiriman,
		KonteksKonfirmasiTerima, KonteksPengaduan,
	} {
		if !ValidateKonteks(k) {
			t.Fatalf("ValidateKonteks(%q) = false, want true", k)
		}
	}
	for _, k := range []string{"", "unknown", "PENERIMAAN_BAHAN", "pengaduan "} {
		if ValidateKonteks(k) {
			t.Fatalf("ValidateKonteks(%q) = true, want false", k)
		}
	}
}

func TestBuildObjectKeyShape(t *testing.T) {
	// 16 bytes of 'a' → 32 hex chars of "6161...".
	fixedRand := strings.NewReader(strings.Repeat("a", 16))
	now := time.Date(2026, time.March, 7, 10, 30, 0, 0, time.UTC)

	got := BuildObjectKey("SPG001", KonteksPenerimaanBahan, "jpg", now, fixedRand)
	want := "SPG001/penerimaan_bahan/2026/03/07/61616161616161616161616161616161.jpg"
	if got != want {
		t.Fatalf("BuildObjectKey = %q, want %q", got, want)
	}

	// Two draws from crypto/rand-equivalent input differ.
	r1 := strings.NewReader(strings.Repeat("\x00", 16))
	r2 := strings.NewReader(strings.Repeat("\x01", 16))
	k1 := BuildObjectKey("SPG001", "pengaduan", "webp", now, r1)
	k2 := BuildObjectKey("SPG001", "pengaduan", "webp", now, r2)
	if k1 == k2 {
		t.Fatalf("different random inputs produced identical keys")
	}
	// Kedua key tetap berbentuk {kode}/{konteks}/{YYYY}/{MM}/{DD}/32hex.ext
	for _, k := range []string{k1, k2} {
		parts := strings.Split(k, "/")
		if len(parts) != 6 {
			t.Fatalf("key %q should have 6 slash-separated parts", k)
		}
		suffix := strings.TrimSuffix(parts[5], ".webp")
		if len(suffix) != 32 || suffix != strings.ToLower(suffix) {
			t.Fatalf("suffix %q is not 32 lowercase hex chars", suffix)
		}
	}
}

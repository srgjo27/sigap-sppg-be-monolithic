package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/domain"
)

// --- fakes (hand-written, seperti pengadaan_test) --------------------------

type fakeStore struct {
	putKey      string
	putSize     int64
	putContent  []byte
	presignKey  string
	presignTTL  time.Duration
	deletedKeys []string
	failPut     error
	failPresign error
	failDelete  error
}

func (f *fakeStore) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	content, _ := io.ReadAll(r) // simulate what storage would consume
	if f.failPut != nil {
		return f.failPut
	}
	f.putKey, f.putSize, f.putContent = key, size, content
	return nil
}

func (f *fakeStore) Presign(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if f.failPresign != nil {
		return "", f.failPresign
	}
	f.presignKey, f.presignTTL = key, ttl
	return "http://presigned/" + key, nil
}

func (f *fakeStore) Delete(ctx context.Context, keys []string) error {
	if f.failDelete != nil {
		return f.failDelete
	}
	f.deletedKeys = append(f.deletedKeys, keys...)
	return nil
}

type fakeRepo struct {
	created     *domain.Berkas
	unusedList  []domain.Berkas
	deletedKeys []string
	failCreate  error
}

func (f *fakeRepo) Create(ctx context.Context, b *domain.Berkas) error {
	if f.failCreate != nil {
		return f.failCreate
	}
	b.ID = 42
	f.created = b
	return nil
}

func (f *fakeRepo) UnusedBefore(ctx context.Context, cutoff time.Time) ([]domain.Berkas, error) {
	return f.unusedList, nil
}

func (f *fakeRepo) DeleteUnusedBefore(ctx context.Context, cutoff time.Time) ([]string, error) {
	keys := make([]string, 0, len(f.unusedList))
	for _, b := range f.unusedList {
		keys = append(keys, b.ObjectKey)
	}
	f.deletedKeys = append(f.deletedKeys, keys...)
	return keys, nil
}

type fakeSPPG struct{ kode string }

func (f fakeSPPG) GetKode(ctx context.Context, id int64) (string, error) {
	return f.kode, nil
}

// --- test fixtures ---------------------------------------------------------

var fixedNow = time.Date(2026, time.March, 7, 10, 30, 0, 0, time.UTC)

func fixedClaims(role authdomain.Role, sppg int64) *authdomain.Claims {
	return &authdomain.Claims{UserID: 7, Role: role, SPPGID: &sppg}
}

// minimal JPEG with one APP1 (Exif) segment
func jpegBytes() []byte {
	return []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x04, 'J', 'F',
		0xFF, 0xE1, 0x00, 0x06, 'E', 'x', 'i', 'f',
		0xFF, 0xDA, 0x00, 0x02, 0x11, 0x22,
		0xFF, 0xD9}
}

// minimal PNG with an eXIf chunk
func pngBytes() []byte {
	var b bytes.Buffer
	b.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	// IHDR (13 bytes payload)
	var ihdr = []byte{1, 0, 0, 1, 8, 6, 0, 0, 0, 0, 13, 0}
	chunk := func(typ string, data []byte) {
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
		b.Write(lenBuf[:])
		b.WriteString(typ)
		b.Write(data)
		b.Write([]byte{0, 0, 0, 0})
	}
	chunk("IHDR", ihdr)
	chunk("eXIf", []byte("exif"))
	chunk("IDAT", []byte("data"))
	chunk("IEND", nil)
	return b.Bytes()
}

// minimal WebP with EXIF chunk
func webpBytes() []byte {
	var body bytes.Buffer
	body.WriteString("WEBP")
	emit := func(typ string, data []byte) {
		body.WriteString(typ)
		var sizeBuf [4]byte
		total := uint32(len(data))
		if len(data)%2 == 1 {
			total++
		}
		binary.LittleEndian.PutUint32(sizeBuf[:], total)
		body.Write(sizeBuf[:])
		body.Write(data)
		if len(data)%2 == 1 {
			body.WriteByte(0)
		}
	}
	emit("VP8 ", []byte("12345"))
	emit("EXIF", []byte("exif"))
	out := []byte("RIFF")
	var riff [4]byte
	binary.LittleEndian.PutUint32(riff[:], uint32(body.Len()))
	out = append(out, riff[:]...)
	out = append(out, body.Bytes()...)
	return out
}

func newTestService(store ObjectStore, repo BerkasRepository, sppg SPPGCodeProvider) *Service {
	return New(Deps{
		Store: store, Repo: repo, SPPG: sppg,
		Now:  func() time.Time { return fixedNow },
		Rand: strings.NewReader(strings.Repeat("A", 16)),
	})
}

// --- Upload ----------------------------------------------------------------

func TestUploadHappyPathJPEG(t *testing.T) {
	store := &fakeStore{}
	repo := &fakeRepo{}
	svc := newTestService(store, repo, fakeSPPG{kode: "SPG001"})

	payload := jpegBytes()
	stripped, err := domain.StripImageMetadata(payload, "image/jpeg")
	if err != nil {
		t.Fatalf("fixture strip: %v", err)
	}
	res, err := svc.Upload(context.Background(), fixedClaims(authdomain.RoleAkuntan, 1), domain.KonteksPenerimaanBahan, payload)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// sha256 stored must equal sha256 of the *stripped* bytes.
	sum := sha256.Sum256(stripped)
	wantSHA := hex.EncodeToString(sum[:])
	if res.SHA256 != wantSHA {
		t.Fatalf("SHA256 = %q, want sha256(stripped) %q", res.SHA256, wantSHA)
	}
	// Object key shape and stored on repo.
	wantKeyPrefix := "SPG001/penerimaan_bahan/" + fixedNow.Format("2006/01/02/") + "41414141414141414141414141414141" + ".jpg"
	if store.putKey != wantKeyPrefix {
		t.Fatalf("object key = %q, want %q", store.putKey, wantKeyPrefix)
	}
	// Content-length honored: size matches stripped byte count.
	if store.putSize != int64(len(stripped)) {
		t.Fatalf("Put size = %d, want %d (post-strip)", store.putSize, len(stripped))
	}
	// Repo row mirrors schema columns.
	if repo.created == nil {
		t.Fatalf("repo.Create not called")
	}
	b := repo.created
	if b.ObjectKey != store.putKey || b.SPPGID != 1 || b.Konteks != domain.KonteksPenerimaanBahan ||
		b.MimeType != "image/jpeg" || b.UkuranByte != len(stripped) ||
		b.SHA256 != wantSHA || b.DiunggahOleh != 7 || !b.DiunggahAt.Equal(fixedNow) {
		t.Fatalf("repo berkas row = %+v", b)
	}
	if b.ID != 42 {
		t.Fatalf("res should carry generated id, got %d on repo row", b.ID)
	}
	// Presign TTL passthrough and URL.
	if store.presignTTL != domain.PresignTTL {
		t.Fatalf("presign TTL = %v, want %v", store.presignTTL, domain.PresignTTL)
	}
	if res.URL != "http://presigned/"+store.putKey {
		t.Fatalf("URL = %q", res.URL)
	}
	if res.BerkasID != 42 || res.Konteks != domain.KonteksPenerimaanBahan || res.MimeType != "image/jpeg" ||
		res.UkuranByte != len(stripped) || !res.DiunggahAt.Equal(fixedNow) {
		t.Fatalf("UploadResult = %+v", res)
	}
	// EXIF APP1 must be stripped before storing.
	if bytes.Contains(store.putContent, []byte("Exif")) {
		t.Fatalf("stored bytes still contain EXIF payload")
	}
}

func TestUploadUnknownKonteks(t *testing.T) {
	store := &fakeStore{}
	repo := &fakeRepo{}
	svc := newTestService(store, repo, fakeSPPG{})
	_, err := svc.Upload(context.Background(), fixedClaims(authdomain.RoleAkuntan, 1), "konteks_aneh", jpegBytes())
	if !errors.Is(err, domain.ErrKonteksUnknown) {
		t.Fatalf("err = %v, want ErrKonteksUnknown", err)
	}
	if store.putKey != "" || repo.created != nil {
		t.Fatalf("nothing should be stored on invalid konteks")
	}
}

func TestUploadRoleDenied(t *testing.T) {
	cases := []struct {
		role    authdomain.Role
		konteks string
	}{
		{authdomain.RolePengawas, domain.KonteksPenerimaanBahan},  // pengawas everywhere
		{authdomain.RolePengawas, domain.KonteksKonfirmasiTerima}, // defense in depth
		{authdomain.RoleAhliGizi, domain.KonteksPengiriman},       // wrong role for context
		{authdomain.RolePetugasDapur, domain.KonteksPengiriman},
		{authdomain.RolePICsekolah, domain.KonteksChecklistSOP},
	}
	for _, tc := range cases {
		store := &fakeStore{}
		repo := &fakeRepo{}
		svc := newTestService(store, repo, fakeSPPG{})
		_, err := svc.Upload(context.Background(), fixedClaims(tc.role, 1), tc.konteks, jpegBytes())
		if !errors.Is(err, domain.ErrKonteksForbidden) {
			t.Fatalf("Upload(%q as %q) err = %v, want ErrKonteksForbidden", tc.konteks, tc.role, err)
		}
		if store.putKey != "" || repo.created != nil {
			t.Fatalf("denied upload must not store or persist anything")
		}
	}
}

func TestUploadNilSPPGIDForbidden(t *testing.T) {
	svc := newTestService(&fakeStore{}, &fakeRepo{}, fakeSPPG{})
	claims := &authdomain.Claims{UserID: 7, Role: authdomain.RolePICsekolah, SPPGID: nil} // pic_sekolah tanpa SPPG
	_, err := svc.Upload(context.Background(), claims, domain.KonteksPengaduan, jpegBytes())
	if !errors.Is(err, domain.ErrKonteksForbidden) {
		t.Fatalf("err = %v, want ErrKonteksForbidden (nil SPPGID)", err)
	}
}

func TestUploadNilClaims(t *testing.T) {
	svc := newTestService(&fakeStore{}, &fakeRepo{}, fakeSPPG{})
	_, err := svc.Upload(context.Background(), nil, domain.KonteksPengiriman, jpegBytes())
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestUploadEmptyPayload(t *testing.T) {
	store := &fakeStore{}
	repo := &fakeRepo{}
	svc := newTestService(store, repo, fakeSPPG{})
	_, err := svc.Upload(context.Background(), fixedClaims(authdomain.RoleAkuntan, 1), domain.KonteksPenerimaanBahan, nil)
	if !errors.Is(err, domain.ErrFileEmpty) {
		t.Fatalf("err = %v, want ErrFileEmpty", err)
	}
	if store.putKey != "" || repo.created != nil {
		t.Fatalf("empty file must not store anything")
	}
}

func TestUploadTooLarge(t *testing.T) {
	store := &fakeStore{}
	repo := &fakeRepo{}
	svc := newTestService(store, repo, fakeSPPG{})
	// 5MB + 1 byte: valid PNG header signature, so the size gate must win.
	oversize := make([]byte, domain.MaxUploadBytes+1)
	copy(oversize, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	_, err := svc.Upload(context.Background(), fixedClaims(authdomain.RoleAkuntan, 1), domain.KonteksPenerimaanBahan, oversize)
	if !errors.Is(err, domain.ErrFileTooLarge) {
		t.Fatalf("err = %v, want ErrFileTooLarge", err)
	}
	if store.putKey != "" || repo.created != nil {
		t.Fatalf("oversize file must not store anything")
	}
}

func TestUploadNonImage(t *testing.T) {
	store := &fakeStore{}
	repo := &fakeRepo{}
	svc := newTestService(store, repo, fakeSPPG{})
	_, err := svc.Upload(context.Background(), fixedClaims(authdomain.RoleAkuntan, 1), domain.KonteksPenerimaanBahan, []byte("GIF89a-not-an-image"))
	if !errors.Is(err, domain.ErrUnsupportedImage) {
		t.Fatalf("err = %v, want ErrUnsupportedImage", err)
	}
	if store.putKey != "" || repo.created != nil {
		t.Fatalf("non-image must not store anything")
	}
}

func TestUploadStripsPNGAndWebP(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		mime    string
	}{
		{"png", pngBytes(), "image/png"},
		{"webp", webpBytes(), "image/webp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{}
			repo := &fakeRepo{}
			svc := newTestService(store, repo, fakeSPPG{})
			res, err := svc.Upload(context.Background(), fixedClaims(authdomain.RolePetugasDapur, 1), domain.KonteksChecklistSOP, tc.payload)
			if err != nil {
				t.Fatalf("Upload: %v", err)
			}
			if res.MimeType != tc.mime {
				t.Fatalf("mime = %q, want %q", res.MimeType, tc.mime)
			}
			if bytes.Contains(store.putContent, []byte("exif")) {
				t.Fatalf("stored bytes still contain EXIF metadata")
			}
		})
	}
}

func TestUploadSPPGNotFound(t *testing.T) {
	svc := New(Deps{
		Store: &fakeStore{}, Repo: &fakeRepo{},
		SPPG: sppgNotFoundFake{},
		Now:  func() time.Time { return fixedNow },
		Rand: strings.NewReader(strings.Repeat("A", 16)),
	})
	_, err := svc.Upload(context.Background(), fixedClaims(authdomain.RoleAkuntan, 99), domain.KonteksPenerimaanBahan, jpegBytes())
	if !errors.Is(err, domain.ErrSPPGNotFound) {
		t.Fatalf("err = %v, want ErrSPPGNotFound", err)
	}
}

type sppgNotFoundFake struct{}

func (sppgNotFoundFake) GetKode(ctx context.Context, id int64) (string, error) {
	return "", domain.ErrSPPGNotFound
}

func TestUploadStoreErrorMaps500NotSentinel(t *testing.T) {
	store := &fakeStore{failPut: errors.New("minio down")}
	repo := &fakeRepo{}
	svc := newTestService(store, repo, fakeSPPG{})
	_, err := svc.Upload(context.Background(), fixedClaims(authdomain.RoleAkuntan, 1), domain.KonteksPenerimaanBahan, jpegBytes())
	if err == nil || errors.Is(err, domain.ErrUnsupportedImage) {
		t.Fatalf("store failure should surface wrapped infra error, got %v", err)
	}
	if repo.created != nil {
		t.Fatalf("failed store must not persist row")
	}
}

// --- PurgeUnused -----------------------------------------------------------

func TestPurgeUnusedDeletesObjectsAndRows(t *testing.T) {
	store := &fakeStore{}
	unused := []domain.Berkas{
		{ID: 1, ObjectKey: "SPG001/penerimaan_bahan/2026/03/05/aaa.jpg"},
		{ID: 2, ObjectKey: "SPG001/pengaduan/2026/03/05/bbb.png"},
	}
	repo := &fakeRepo{unusedList: unused}
	svc := newTestService(store, repo, fakeSPPG{})

	if err := svc.PurgeUnused(context.Background(), 24*time.Hour); err != nil {
		t.Fatalf("PurgeUnused: %v", err)
	}
	wantKeys := []string{
		"SPG001/penerimaan_bahan/2026/03/05/aaa.jpg",
		"SPG001/pengaduan/2026/03/05/bbb.png",
	}
	if len(store.deletedKeys) != len(wantKeys) {
		t.Fatalf("store.Delete keys = %v, want %v", store.deletedKeys, wantKeys)
	}
	for i, key := range wantKeys {
		if store.deletedKeys[i] != key {
			t.Fatalf("store.Delete keys[%d] = %q, want %q", i, store.deletedKeys[i], key)
		}
	}
	if len(repo.deletedKeys) != len(wantKeys) {
		t.Fatalf("repo.DeleteUnusedBefore should remove the rows, got %v", repo.deletedKeys)
	}
}

func TestPurgeUnusedNoRows(t *testing.T) {
	store := &fakeStore{}
	repo := &fakeRepo{} // unusedList nil
	svc := newTestService(store, repo, fakeSPPG{})
	if err := svc.PurgeUnused(context.Background(), 24*time.Hour); err != nil {
		t.Fatalf("PurgeUnused: %v", err)
	}
	if len(store.deletedKeys) != 0 {
		t.Fatalf("no rows → no object deletions, got %v", store.deletedKeys)
	}
}

func TestFixturesStructurallyValid(t *testing.T) {
	// Guard test: fixtures must survive strip so service tests stay honest.
	if _, err := domain.StripImageMetadata(jpegBytes(), "image/jpeg"); err != nil {
		t.Fatalf("jpeg fixture: %v", err)
	}
	if _, err := domain.StripImageMetadata(pngBytes(), "image/png"); err != nil {
		t.Fatalf("png fixture: %v", err)
	}
	if _, err := domain.StripImageMetadata(webpBytes(), "image/webp"); err != nil {
		t.Fatalf("webp fixture: %v", err)
	}
}

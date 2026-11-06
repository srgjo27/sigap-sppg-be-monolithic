package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/domain"
)

// --- fakes (local to transport tests; hand-written like the repo style) ----

type stubStore struct {
	putKey   string
	putSize  int64
	content  []byte
	presignT time.Duration
	deleted  []string
}

func (s *stubStore) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	content, _ := io.ReadAll(r)
	s.putKey, s.putSize, s.content = key, size, content
	return nil
}

func (s *stubStore) Presign(ctx context.Context, key string, ttl time.Duration) (string, error) {
	s.presignT = ttl
	return "http://presigned.local/" + key, nil
}

func (s *stubStore) Delete(ctx context.Context, keys []string) error {
	s.deleted = append(s.deleted, keys...)
	return nil
}

type stubRepo struct {
	created *domain.Berkas
}

func (r *stubRepo) Create(ctx context.Context, b *domain.Berkas) error {
	b.ID = 101
	r.created = b
	return nil
}

func (r *stubRepo) UnusedBefore(ctx context.Context, cutoff time.Time) ([]domain.Berkas, error) {
	return nil, nil
}

func (r *stubRepo) DeleteUnusedBefore(ctx context.Context, cutoff time.Time) ([]string, error) {
	return nil, nil
}

type stubSPPG struct{}

func (stubSPPG) GetKode(ctx context.Context, id int64) (string, error) { return "SPG001", nil }

// --- helpers ---------------------------------------------------------------

// setClaims mimics authhttp.Authenticate by placing claims in the same
// context key ("current_user") used by authhttp middleware, so that
// authhttp.CurrentUser(c) resolves them. Used instead of a real JWT to
// keep these tests on the transport contract only.
func setClaims(claims *authdomain.Claims) gin.HandlerFunc {
	return func(c *gin.Context) {
		if claims != nil {
			c.Set("current_user", claims) // see auth/transport/http/middleware.go claimsKey
		}
		c.Next()
	}
}

// mountUpload builds a fresh engine with the berkas upload chain minus
// the real JWT middleware (claims are injected directly).
func mountUpload(claims *authdomain.Claims) (*gin.Engine, *Handler, *stubStore, *stubRepo) {
	gin.SetMode(gin.TestMode)
	store := &stubStore{}
	repo := &stubRepo{}
	svc := application.New(application.Deps{
		Store: store, Repo: repo, SPPG: stubSPPG{},
		Now:  func() time.Time { return time.Date(2026, 3, 7, 10, 30, 0, 0, time.UTC) },
		Rand: bytes.NewReader(bytes.Repeat([]byte{0xAB}, 16)),
	})
	h := NewHandler(svc)
	r := gin.New()
	r.POST("/uploads", setClaims(claims), mwPengawas(), h.RateLimitUpload(), h.Upload)
	return r, h, store, repo
}

// mwPengawas reproduces PengawasReadOnly route-level behavior without a
// real middleware (upload must be a POST-deny for pengawas).
func mwPengawas() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := authhttp.CurrentUser(c)
		if ok && claims.Role == authdomain.RolePengawas {
			c.AbortWithStatusJSON(http.StatusForbidden, ErrorEnvelope{Error: ErrorBody{Code: "FORBIDDEN", Message: "pengawas hanya boleh membaca data"}})
			return
		}
		c.Next()
	}
}

// multipartBody builds multipart/form-data with the given file contents
// and returns the encoded raw bytes plus its Content-Type.
func multipartBody(t *testing.T, filename string, content []byte, konteks string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if filename != "" {
		fw, err := w.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("write file part: %v", err)
		}
	}
	if konteks != "" {
		if err := w.WriteField("konteks", konteks); err != nil {
			t.Fatalf("write konteks field: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

// doUpload posts the multipart request and decodes the JSON envelope.
// raw is the encoded multipart body; it is re-read on every call, so the
// same payload can be posted repeatedly (needed by the rate-limit test).
func doUpload(t *testing.T, r *gin.Engine, raw []byte, contentType string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/uploads", bytes.NewReader(raw))
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var decoded map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &decoded)
	return rec, decoded
}

// errorBody extracts {"error":{...}} from a response.
type errEnvelope struct {
	Error struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields,omitempty"`
	} `json:"error"`
}

func errorOf(t *testing.T, rec *httptest.ResponseRecorder) errEnvelope {
	t.Helper()
	var e errEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode error envelope: %v (body %q)", err, rec.Body.String())
	}
	return e
}

// valid JPEG with an APP1 (Exif) segment so 201 tests strip real bytes.
func happyJPEG() []byte {
	return []byte{0xFF, 0xD8,
		0xFF, 0xE1, 0x00, 0x06, 'E', 'x', 'i', 'f', // APP1/Exif → stripped
		0xFF, 0xDA, 0x00, 0x02, 0x11, 0x22, // SOS + entropy
		0xFF, 0xD9}
}

var claimsAkuntan = &authdomain.Claims{UserID: 7, Role: authdomain.RoleAkuntan, SPPGID: int64Ptr(1)}

func int64Ptr(v int64) *int64 { return &v }

// --- tests -----------------------------------------------------------------

func TestUploadHandlerSuccess201(t *testing.T) {
	r, _, store, repo := mountUpload(claimsAkuntan)
	body, ct := multipartBody(t, "foto.jpg", happyJPEG(), domain.KonteksPenerimaanBahan)
	rec, decoded := doUpload(t, r, body, ct)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}
	if decoded["berkas_id"].(float64) != 101 {
		t.Fatalf("berkas_id = %v", decoded["berkas_id"])
	}
	if decoded["konteks"] != domain.KonteksPenerimaanBahan {
		t.Fatalf("konteks = %v", decoded["konteks"])
	}
	if decoded["mime_type"] != "image/jpeg" {
		t.Fatalf("mime_type = %v", decoded["mime_type"])
	}
	// ukuran_byte is post-strip (APP1 dropped).
	wantLen := len(happyJPEG()) - 8 // 0xFF E1 + 6-byte APP1 removed
	if int(decoded["ukuran_byte"].(float64)) != wantLen {
		t.Fatalf("ukuran_byte = %v, want %d", decoded["ukuran_byte"], wantLen)
	}
	if decoded["url"] != "http://presigned.local/"+store.putKey {
		t.Fatalf("url = %v", decoded["url"])
	}
	if _, ok := decoded["sha256"].(string); !ok || len(decoded["sha256"].(string)) != 64 {
		t.Fatalf("sha256 = %v", decoded["sha256"])
	}
	if _, ok := decoded["diunggah_at"].(string); !ok {
		t.Fatalf("diunggah_at missing: %v", decoded)
	}
	if repo.created == nil || repo.created.ObjectKey != store.putKey {
		t.Fatalf("repo row not linked to stored object: %+v", repo.created)
	}
	if store.presignT != domain.PresignTTL {
		t.Fatalf("presign TTL = %v, want %v", store.presignT, domain.PresignTTL)
	}
}

func TestUploadHandlerMissingFilePart400(t *testing.T) {
	r, _, _, _ := mountUpload(claimsAkuntan)
	body, ct := multipartBody(t, "", []byte(""), domain.KonteksPenerimaanBahan) // no file part
	rec, _ := doUpload(t, r, body, ct)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	env := errorOf(t, rec)
	if env.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %v, want VALIDATION_ERROR", env.Error.Code)
	}
}

func TestUploadHandlerEmptyFilePart400(t *testing.T) {
	r, _, _, _ := mountUpload(claimsAkuntan)
	body, ct := multipartBody(t, "foto.jpg", []byte{}, domain.KonteksPenerimaanBahan)
	rec, decoded := doUpload(t, r, body, ct)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := decoded["error"].(map[string]any)["code"]; got != "VALIDATION_ERROR" {
		t.Fatalf("code = %v, want VALIDATION_ERROR", got)
	}
}

func TestUploadHandlerUnknownKonteks400(t *testing.T) {
	r, _, _, _ := mountUpload(claimsAkuntan)
	body, ct := multipartBody(t, "foto.jpg", happyJPEG(), "konteks_aneh")
	rec, decoded := doUpload(t, r, body, ct)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := decoded["error"].(map[string]any)["code"]; got != "KONTEKS_TIDAK_DIKENAL" {
		t.Fatalf("code = %v, want KONTEKS_TIDAK_DIKENAL", got)
	}
}

func TestUploadHandlerMissingKonteks400(t *testing.T) {
	r, _, store, _ := mountUpload(claimsAkuntan)
	body, ct := multipartBody(t, "foto.jpg", happyJPEG(), "") // konteks field absent
	rec, decoded := doUpload(t, r, body, ct)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := decoded["error"].(map[string]any)["code"]; got != "KONTEKS_TIDAK_DIKENAL" {
		t.Fatalf("code = %v, want KONTEKS_TIDAK_DIKENAL", got)
	}
	if store.putKey != "" {
		t.Fatalf("nothing should be stored on invalid request")
	}
}

func TestUploadHandlerRoleForbidden403(t *testing.T) {
	// akuntan di-tolak pada konteks pengaduan / pengiriman; petugas lain juga.
	for _, claims := range []*authdomain.Claims{
		claimsAkuntan,
		{UserID: 7, Role: authdomain.RolePengawas, SPPGID: int64Ptr(1)},
		{UserID: 7, Role: authdomain.RoleAhliGizi, SPPGID: int64Ptr(1)},
	} {
		r, _, _, _ := mountUpload(claims)
		body, ct := multipartBody(t, "foto.jpg", happyJPEG(), domain.KonteksPengiriman)
		rec, _ := doUpload(t, r, body, ct)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("role %q: status = %d, want 403", claims.Role, rec.Code)
		}
		env := errorOf(t, rec)
		if env.Error.Code != "FORBIDDEN" {
			t.Fatalf("role %q: code = %q, want FORBIDDEN", claims.Role, env.Error.Code)
		}
	}
}

func TestUploadHandlerBodyTooLarge413(t *testing.T) {
	r, _, _, _ := mountUpload(claimsAkuntan)
	// > maxRequestBodyBytes (6MB) → MaxBytesReader failure branch.
	big := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF, 0x00}, (maxRequestBodyBytes/4)+1024)
	body, ct := multipartBody(t, "big.jpg", big, domain.KonteksPenerimaanBahan)
	rec, decoded := doUpload(t, r, body, ct)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if got := decoded["error"].(map[string]any)["code"]; got != "FILE_TOO_LARGE" {
		t.Fatalf("code = %v, want FILE_TOO_LARGE", got)
	}
}

func TestUploadHandlerFileAbove5MBBelowBodyLimit413(t *testing.T) {
	r, _, _, _ := mountUpload(claimsAkuntan)
	// 5.5MB: within the 6MB body cap but past the 5MB upload limit.
	big := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF, 0x11}, (5<<20)/3+20480)
	body, ct := multipartBody(t, "big.jpg", big, domain.KonteksPenerimaanBahan)
	rec, decoded := doUpload(t, r, body, ct)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if got := decoded["error"].(map[string]any)["code"]; got != "FILE_TOO_LARGE" {
		t.Fatalf("code = %v, want FILE_TOO_LARGE", got)
	}
}

func TestUploadHandlerUnsupportedType415(t *testing.T) {
	r, _, _, _ := mountUpload(claimsAkuntan)
	body, ct := multipartBody(t, "gif.gif", []byte("GIF89a\x01\x00\x01\x00\x00"), domain.KonteksPenerimaanBahan)
	rec, decoded := doUpload(t, r, body, ct)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
	if got := decoded["error"].(map[string]any)["code"]; got != "TIPE_BERKAS_TIDAK_DIDUKUNG" {
		t.Fatalf("code = %v, want TIPE_BERKAS_TIDAK_DIDUKUNG", got)
	}
}

func TestUploadHandlerRateLimit429(t *testing.T) {
	r, h, _, _ := mountUpload(claimsAkuntan)
	h.limiter.Reset()
	body, ct := multipartBody(t, "foto.jpg", happyJPEG(), domain.KonteksPenerimaanBahan)

	for i := 0; i < rateLimitUpload; i++ {
		rec, _ := doUpload(t, r, body, ct)
		if rec.Code != http.StatusCreated {
			t.Fatalf("upload #%d: status = %d, want 201", i+1, rec.Code)
		}
	}
	// 31st upload within the window → 429 RATE_LIMITED.
	rec, decoded := doUpload(t, r, body, ct)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("upload #31: status = %d, want 429 (body %q)", rec.Code, rec.Body.String())
	}
	if got := decoded["error"].(map[string]any)["code"]; got != "RATE_LIMITED" {
		t.Fatalf("code = %v, want RATE_LIMITED", got)
	}
}

func TestUploadHandlerRateLimitPerUser(t *testing.T) {
	r, h, _, _ := mountUpload(claimsAkuntan)
	h.limiter.Reset()
	// user 7 gaplok 30; user lain tetap boleh.
	other := &authdomain.Claims{UserID: 8, Role: authdomain.RoleAkuntan, SPPGID: int64Ptr(1)}
	rOther := gin.New()
	rOther.POST("/uploads", setClaims(other), h.RateLimitUpload(), h.Upload)

	body, ct := multipartBody(t, "foto.jpg", happyJPEG(), domain.KonteksPenerimaanBahan)
	for i := 0; i < rateLimitUpload; i++ {
		if rec, _ := doUpload(t, r, body, ct); rec.Code != http.StatusCreated {
			t.Fatalf("user 7 upload #%d: %d", i+1, rec.Code)
		}
	}
	if rec, _ := doUpload(t, r, body, ct); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("user 7 #31: want 429, got %d", rec.Code)
	}
	if rec, _ := doUpload(t, rOther, body, ct); rec.Code != http.StatusCreated {
		t.Fatalf("user 8 harus tetap boleh: %d", rec.Code)
	}
}

func TestUploadHandlerUnauthenticatedReturns401(t *testing.T) {
	_, h, _, _ := mountUpload(nil) // claims nil → CurrentUser fails
	r := gin.New()
	r.POST("/uploads", h.RateLimitUpload(), h.Upload)
	body, ct := multipartBody(t, "foto.jpg", happyJPEG(), domain.KonteksPenerimaanBahan)
	rec, decoded := doUpload(t, r, body, ct)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := decoded["error"].(map[string]any)["code"]; got != "UNAUTHENTICATED" {
		t.Fatalf("code = %v, want UNAUTHENTICATED", got)
	}
}

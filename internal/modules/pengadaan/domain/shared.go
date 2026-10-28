package domain

import (
	"fmt"
	"strings"
	"time"
)

// Bahan mirrors the bahan table in db/schema.sql.
type Bahan struct {
	ID             int64
	Nama           string
	Kategori       string
	Satuan         string
	GramPerSatuan  float64
	MudahRusak     bool
	SuhuSimpanMaks *float64
	Aktif          bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Menu mirrors the menu table in db/schema.sql.
type Menu struct {
	ID            int64
	SPPGID        int64
	Tanggal       time.Time
	NamaMenu      string
	EnergiKkal    float64
	ProteinG      float64
	KarbohidratG  float64
	LemakG        float64
	TargetPorsi   int
	Catatan       *string
	DibuatOleh    int64
	Status        string
	DisetujuiOleh *int64
	DisetujuiAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

const (
	MenuStatusDraf      = "draf"
	MenuStatusDisetujui = "disetujui"
)

// MenuBahan mirrors the menu_bahan table.
type MenuBahan struct {
	ID           int64
	MenuID       int64
	BahanID      int64
	BahanNama    string
	GramPerPorsi float64
}

// MenuFilter scopes GET /menus.
type MenuFilter struct {
	Dari   time.Time
	Sampai time.Time
	Status *string
	SPPGID *int64
	Page   int
	Limit  int
}

// BahanFilter scopes GET /bahan.
type BahanFilter struct {
	Q        *string
	Kategori *string
	Aktif    *bool
	Page     int
	Limit    int
}

// Normalize applies defaults (page 1, limit 20, max 100).
func (f *BahanFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
}

// Offset returns the SQL offset.
func (f BahanFilter) Offset() int { return (f.Page - 1) * f.Limit }

// PurchaseOrder mirrors the purchase_order table.
type PurchaseOrder struct {
	ID              int64
	SPPGID          int64
	NomorPO         string
	PemasokID       int64
	SiklusID        *int64
	TglPO           time.Time
	TglKirimDiminta *time.Time
	Total           float64
	Status          string
	DibuatOleh      int64
	Catatan         *string
	DikirimOleh     *int64
	DikirimAt       *time.Time
	DitutupOleh     *int64
	DitutupAt       *time.Time
	AlasanTutup     *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

const (
	POStatusDraf             = "draf"
	POStatusDikirim          = "dikirim"
	POStatusDiterimaSebagian = "diterima_sebagian"
	POStatusSelesai          = "selesai"
	POStatusBatal            = "batal"
)

// POItem mirrors the po_item table.
type POItem struct {
	ID          int64
	POID        int64
	BahanID     int64
	QtyPesan    float64
	HargaSatuan float64
	Subtotal    float64
}

// PenerimaanBahan mirrors the penerimaan_bahan table.
type PenerimaanBahan struct {
	ID              int64
	POItemID        int64
	KodeLot         string
	WaktuTerima     time.Time
	QtyDiterima     float64
	QtyDitolak      float64
	AlasanTolak     *string
	SuhuTerima      *float64
	Kondisi         string
	TglKedaluwarsa  *time.Time
	FotoURL         string
	DiperiksaOleh   int64
	NilaiPotongNota float64
	SPPGID          int64
	ClientRef       string
	LolosSuhu       *bool
	Catatan         *string
	DikoreksiOleh   *int64
	DikoreksiAt     *time.Time
	CreatedAt       time.Time
}

const (
	KondisiBaik   = "baik"
	KondisiKurang = "kurang"
	KondisiRusak  = "rusak"
)

// validKondisi enumerates accepted kondisi values.
var validKondisi = map[string]bool{
	KondisiBaik:   true,
	KondisiKurang: true,
	KondisiRusak:  true,
}

// PenerimaanItem is one item in a penerimaan request (MVP-003.8).
type PenerimaanItem struct {
	POItemID        int64    `json:"po_item_id"`
	QtyDiterima     float64  `json:"qty_diterima"`
	QtyDitolak      float64  `json:"qty_ditolak"`
	AlasanTolak     *string  `json:"alasan_tolak,omitempty"`
	SuhuTerima      *float64 `json:"suhu_terima,omitempty"`
	Kondisi         string   `json:"kondisi"`
	TglKedaluwarsa  *string  `json:"tgl_kedaluwarsa,omitempty"` // YYYY-MM-DD
	FotoBerkasID    int64    `json:"foto_berkas_id"`
	LolosSuhu       *bool    `json:"lolos_suhu,omitempty"`
	Catatan         *string  `json:"catatan,omitempty"`
	NilaiPotongNota float64  `json:"nilai_potong_nota,omitempty"`
}

// CreatePenerimaanInput is the payload for creating receipt lots (MVP-003.8).
// ClientRef comes from the Idempotency-Key header (transport sets it).
type CreatePenerimaanInput struct {
	Items     []PenerimaanItem `json:"items"`
	ClientRef string           `json:"-"`
}

// ValidateCreatePenerimaanInput validates field-level rules for receipt lots.
// Cross-reference rules (PO ownership, status, SPPG scoping, berkas validity,
// temperature, expiry, tolerance) are enforced in the application service.
func ValidateCreatePenerimaanInput(in CreatePenerimaanInput) error {
	fields := map[string]string{}
	if len(in.Items) < 1 || len(in.Items) > 50 {
		fields["items"] = "must have 1-50 items"
	}
	seenItem := make(map[int64]bool)
	for _, item := range in.Items {
		if item.POItemID <= 0 {
			fields["items"] = "po_item_id is required for all items"
			break
		}
		if seenItem[item.POItemID] {
			fields["items"] = "duplicate po_item_id in items"
			break
		}
		seenItem[item.POItemID] = true
		if item.QtyDiterima < 0 {
			fields["items"] = "qty_diterima must be >= 0"
			break
		}
		if item.QtyDitolak < 0 {
			fields["items"] = "qty_ditolak must be >= 0"
			break
		}
		if item.QtyDiterima+item.QtyDitolak <= 0 {
			fields["items"] = "qty_diterima + qty_ditolak must be > 0"
			break
		}
		if !validKondisi[item.Kondisi] {
			fields["items"] = "kondisi must be baik, kurang, or rusak"
			break
		}
		if item.Kondisi == KondisiRusak && item.QtyDiterima > 0 {
			fields["items"] = "damaged item cannot be accepted (qty_diterima must be 0)"
			break
		}
		if item.Kondisi == KondisiKurang && item.Catatan == nil {
			fields["items"] = "catatan is required when kondisi is kurang"
			break
		}
		if item.QtyDitolak > 0 && item.AlasanTolak == nil {
			fields["items"] = "alasan_tolak is required when qty_ditolak > 0"
			break
		}
		if item.AlasanTolak != nil && len(strings.TrimSpace(*item.AlasanTolak)) < 3 {
			fields["items"] = "alasan_tolak must be 3-500 characters"
			break
		}
		if item.AlasanTolak != nil && len(strings.TrimSpace(*item.AlasanTolak)) > 500 {
			fields["items"] = "alasan_tolak must be 3-500 characters"
			break
		}
		if item.FotoBerkasID <= 0 {
			fields["items"] = "foto_berkas_id is required"
			break
		}
		if item.NilaiPotongNota < 0 {
			fields["items"] = "nilai_potong_nota must be >= 0"
			break
		}
		if item.SuhuTerima != nil && (*item.SuhuTerima < -30 || *item.SuhuTerima > 60) {
			fields["items"] = "suhu_terima must be between -30 and 60"
			break
		}
		if item.TglKedaluwarsa != nil {
			if _, err := time.Parse("2006-01-02", *item.TglKedaluwarsa); err != nil {
				fields["items"] = "tgl_kedaluwarsa must use YYYY-MM-DD"
				break
			}
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// PenerimaanListItem is a summary row for GET /penerimaan list view.
type PenerimaanListItem struct {
	ID              int64
	KodeLot         string
	NomorPO         string
	PemasokNama     string
	BahanID         int64
	BahanNama       string
	BahanSatuan     string
	BahanMudahRusak bool
	WaktuTerima     time.Time
	QtyDiterima     float64
	QtyDitolak      float64
	QtyTerpakai     float64
	QtySisa         float64
	Kondisi         string
	SuhuTerima      *float64
	LolosSuhu       *bool
	TglKedaluwarsa  *time.Time
	NilaiPotongNota float64
}

// PenerimaanDetail is the full detail of one receipt lot (MVP-003.9).
type PenerimaanDetail struct {
	ID              int64
	POID            int64
	POItemID        int64
	NomorPO         string
	BahanID         int64
	BahanNama       string
	BahanSatuan     string
	KodeLot         string
	WaktuTerima     time.Time
	QtyDiterima     float64
	QtyDitolak      float64
	AlasanTolak     *string
	SuhuTerima      *float64
	LolosSuhu       *bool
	Kondisi         string
	TglKedaluwarsa  *time.Time
	FotoURL         string
	DiperiksaOleh   int64
	DiperiksaNama   string
	NilaiPotongNota float64
	SPPGID          int64
	ClientRef       string
	Catatan         *string
	DikoreksiOleh   *int64
	DikoreksiAt     *time.Time
	CreatedAt       time.Time
	BatchQtyDipakai float64
	POStatus        string
	PemasokID       int64
	PemasokNama     string
	PemasokNoHP     string
}

// ValidatePenerimaanFilter validates the penerimaan list filter.
func ValidatePenerimaanFilter(f PenerimaanFilter) error {
	fields := map[string]string{}
	if f.Kondisi != nil && !validKondisi[*f.Kondisi] {
		fields["kondisi"] = "must be baik, kurang, or rusak"
	}
	if f.Dari != nil && f.Sampai != nil {
		if f.Sampai.Before(*f.Dari) {
			fields["sampai"] = "must not be before dari"
		}
		days := f.Sampai.Sub(*f.Dari).Hours() / 24
		if days > 31 {
			fields["sampai"] = "range must not exceed 31 days"
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// KoreksiPenerimaanInput is the payload for correcting a receipt lot (MVP-003.10).
// Only qty_diterima, qty_ditolak, alasan_tolak, kondisi, tgl_kedaluwarsa, catatan
// can be corrected. Evidence fields (suhu_terima, lolos_suhu, foto_url,
// waktu_terima, diperiksa_oleh, kode_lot) cannot be changed.
// Alasan is the mandatory correction reason (10-500 characters).
type KoreksiPenerimaanInput struct {
	Alasan          *string  `json:"alasan"`
	QtyDiterima     *float64 `json:"qty_diterima,omitempty"`
	QtyDitolak      *float64 `json:"qty_ditolak,omitempty"`
	AlasanTolak     *string  `json:"alasan_tolak,omitempty"`
	Kondisi         *string  `json:"kondisi,omitempty"`
	TglKedaluwarsa  *string  `json:"tgl_kedaluwarsa,omitempty"` // YYYY-MM-DD
	Catatan         *string  `json:"catatan,omitempty"`
	NilaiPotongNota float64  `json:"nilai_potong_nota,omitempty"`
}

// ValidateKoreksiPenerimaanInput validates field-level rules for koreksi.
// At least one correctable field must be provided. Alasan is mandatory.
func ValidateKoreksiPenerimaanInput(in KoreksiPenerimaanInput) error {
	fields := map[string]string{}

	// Alasan is mandatory, 10-500 characters.
	if in.Alasan == nil {
		fields["alasan"] = "required"
	} else {
		trimmed := strings.TrimSpace(*in.Alasan)
		if len([]rune(trimmed)) < 10 || len([]rune(trimmed)) > 500 {
			fields["alasan"] = "must be 10-500 characters"
		}
	}

	// At least one correctable field must be provided.
	hasField := false
	if in.QtyDiterima != nil {
		hasField = true
		if *in.QtyDiterima < 0 {
			fields["qty_diterima"] = "must be >= 0"
		}
	}
	if in.QtyDitolak != nil {
		hasField = true
		if *in.QtyDitolak < 0 {
			fields["qty_ditolak"] = "must be >= 0"
		}
	}
	if in.AlasanTolak != nil {
		hasField = true
		if len(strings.TrimSpace(*in.AlasanTolak)) < 3 || len(strings.TrimSpace(*in.AlasanTolak)) > 500 {
			fields["alasan_tolak"] = "must be 3-500 characters"
		}
	}
	if in.Kondisi != nil {
		hasField = true
		if !validKondisi[*in.Kondisi] {
			fields["kondisi"] = "must be baik, kurang, or rusak"
		}
	}
	if in.TglKedaluwarsa != nil {
		hasField = true
		if _, err := time.Parse("2006-01-02", *in.TglKedaluwarsa); err != nil {
			fields["tgl_kedaluwarsa"] = "must use YYYY-MM-DD"
		}
	}
	if in.Catatan != nil {
		hasField = true
	}

	if !hasField {
		fields["items"] = "at least one correctable field must be provided"
	}

	// Cross-field: qty_ditolak > 0 requires alasan_tolak.
	if in.QtyDitolak != nil && *in.QtyDitolak > 0 && in.AlasanTolak == nil {
		fields["alasan_tolak"] = "required when qty_ditolak > 0"
	}

	// Cross-field: kondisi rusak with qty_diterima > 0 is not allowed.
	if in.Kondisi != nil && *in.Kondisi == KondisiRusak && in.QtyDiterima != nil && *in.QtyDiterima > 0 {
		fields["qty_diterima"] = "damaged item cannot be accepted (qty_diterima must be 0)"
	}

	// Cross-field: kondisi kurang requires catatan.
	if in.Kondisi != nil && *in.Kondisi == KondisiKurang && in.Catatan == nil {
		fields["catatan"] = "required when kondisi is kurang"
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// SiklusDana status constants
const (
	SiklusStatusDraf       = "draf"
	SiklusStatusDiajukan   = "diajukan"
	SiklusStatusDicairkan  = "dicairkan"
	SiklusStatusDilaporkan = "dilaporkan"
	SiklusStatusDitutup    = "ditutup"
)

// ===== Purchase Order types =====

// POItemInput is the input for a PO item.
type POItemInput struct {
	BahanID     int64   `json:"bahan_id"`
	QtyPesan    float64 `json:"qty_pesan"`
	HargaSatuan float64 `json:"harga_satuan"`
}

// CreatePOInput is the input for creating a purchase order.
type CreatePOInput struct {
	PemasokID       int64         `json:"pemasok_id"`
	TglKirimDiminta string        `json:"tgl_kirim_diminta"` // YYYY-MM-DD
	SiklusID        *int64        `json:"siklus_id,omitempty"`
	MenuIDs         []int64       `json:"menu_ids,omitempty"`
	Catatan         string        `json:"catatan"`
	Items           []POItemInput `json:"items"`
}

// UpdatePOInput is the PATCH input. All fields optional;
// absent keys keep current values. ItemsSet/MenuIDsSet/SiklusIDSet mark key
// presence so `[]`/`null` can clear a list/link.
type UpdatePOInput struct {
	PemasokID       *int64
	TglKirimDiminta *string // YYYY-MM-DD
	SiklusID        *int64
	SiklusIDSet     bool
	MenuIDs         []int64
	MenuIDsSet      bool
	Catatan         *string
	Items           []POItemInput
	ItemsSet        bool
}

// POFilter scopes GET /purchase-orders.
type POFilter struct {
	Status         []string
	PemasokID      *int64
	TglKirimDari   *string
	TglKirimSampai *string
	Q              *string
	SPPGID         *int64 // only for admin/pengawas
	Page           int
	Limit          int
}

// Normalize applies defaults (page 1, limit 20, max 100).
func (f *POFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
}

// Offset returns the SQL offset.
func (f POFilter) Offset() int { return (f.Page - 1) * f.Limit }

// POListItem is a summary item for list view.
type POListItem struct {
	ID                  int64
	NomorPO             string
	PemasokID           int64
	PemasokNama         string
	TglPO               time.Time
	TglKirimDiminta     *time.Time
	Status              string
	Total               float64
	Terlambat           bool
	JumlahItem          int
	JumlahItemTerpenuhi int
}

// PODetail is the detailed view of a purchase order.
type PODetail struct {
	PurchaseOrder
	PemasokNama string
	Items       []PODetailItem
	RekapNilai  PODetailRekap
	Penerimaan  []PODetailPenerimaan
	Menu        []PODetailMenu
}

// PODetailItem is an item in PO detail with rekap data.
type PODetailItem struct {
	ID              int64
	BahanID         int64
	BahanNama       string
	BahanSatuan     string
	BahanMudahRusak bool
	QtyPesan        float64
	HargaSatuan     float64
	Subtotal        float64
	QtyDiterima     float64
	QtyDitolak      float64
	QtyKurang       float64
	Terpenuhi       bool
	NilaiPotongNota float64
	NilaiTagihan    float64
}

// PODetailRekap is the value summary for a PO.
type PODetailRekap struct {
	TotalPesanan      float64
	TotalNilaiDikirim float64
	TotalPotongNota   float64
	TotalTagihan      float64
}

// PODetailPenerimaan is a receipt lot in PO detail.
type PODetailPenerimaan struct {
	ID            int64
	KodeLot       string
	POItemID      int64
	WaktuTerima   time.Time
	QtyDiterima   float64
	QtyDitolak    float64
	Kondisi       string
	DiperiksaOleh int64
}

// PODetailMenu is a linked menu in PO detail.
type PODetailMenu struct {
	ID       int64
	Tanggal  time.Time
	NamaMenu string
}

// POCreateResult is the result of creating a PO.
type POCreateResult struct {
	PO       *PurchaseOrder
	Items    []POItem
	Warnings []string
}

// ValidateUpdatePOInput checks the PATCH envelope shape.
// Field-level rules are re-run on the merged input with updated rules.
func ValidateUpdatePOInput(in UpdatePOInput) error {
	fields := map[string]string{}
	for _, item := range in.Items {
		if item.BahanID <= 0 {
			fields["items"] = "bahan_id is required for all items"
			break
		}
		if item.QtyPesan <= 0 {
			fields["items"] = "qty_pesan must be > 0"
			break
		}
		if item.HargaSatuan <= 0 || item.HargaSatuan > 100000000 {
			fields["items"] = "harga_satuan must be > 0 and <= 100000000"
			break
		}
	}
	if in.TglKirimDiminta != nil {
		if _, err := time.Parse("2006-01-02", strings.TrimSpace(*in.TglKirimDiminta)); err != nil {
			fields["tgl_kirim_diminta"] = "use YYYY-MM-DD"
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// ValidateCreatePOInput validates the create PO input. today must be the
// caller's "truncated to day" clock value so tests and day-boundary semantics
// stay deterministic.
func ValidateCreatePOInput(in CreatePOInput, today time.Time) error {
	fields := map[string]string{}
	if in.PemasokID <= 0 {
		fields["pemasok_id"] = "required"
	}
	if in.TglKirimDiminta == "" {
		fields["tgl_kirim_diminta"] = "required (YYYY-MM-DD)"
	} else {
		t, err := time.Parse("2006-01-02", in.TglKirimDiminta)
		if err != nil {
			fields["tgl_kirim_diminta"] = "use YYYY-MM-DD"
		} else if t.Before(today) {
			fields["tgl_kirim_diminta"] = "cannot be before today"
		}
	}
	if len(in.Items) < 1 || len(in.Items) > 50 {
		fields["items"] = "must have 1-50 items"
	}
	seenBahan := make(map[int64]bool)
	for _, item := range in.Items {
		if item.BahanID <= 0 {
			fields["items"] = "bahan_id is required for all items"
			break
		}
		if seenBahan[item.BahanID] {
			fields["items"] = "duplicate bahan_id in items"
			break
		}
		seenBahan[item.BahanID] = true
		if item.QtyPesan <= 0 {
			fields["items"] = "qty_pesan must be > 0"
			break
		}
		if item.HargaSatuan <= 0 || item.HargaSatuan > 100000000 {
			fields["items"] = "harga_satuan must be > 0 and <= 100000000"
			break
		}
	}
	if len(in.MenuIDs) > 14 {
		fields["menu_ids"] = "max 14 menus"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// allowedPOStatuses enumerates po_status enum labels accepted by PO filters.
var allowedPOStatuses = map[string]bool{
	POStatusDraf: true, POStatusDikirim: true, POStatusDiterimaSebagian: true,
	POStatusSelesai: true, POStatusBatal: true,
}

// ValidatePOFilter validates the PO filter.
func ValidatePOFilter(f POFilter) error {
	fields := map[string]string{}
	for _, st := range f.Status {
		if !allowedPOStatuses[st] {
			fields["status"] = "unknown status"
			break
		}
	}
	if f.TglKirimDari != nil && f.TglKirimSampai != nil {
		dari, err1 := time.Parse("2006-01-02", *f.TglKirimDari)
		sampai, err2 := time.Parse("2006-01-02", *f.TglKirimSampai)
		if err1 != nil {
			fields["tgl_kirim_dari"] = "use YYYY-MM-DD"
		}
		if err2 != nil {
			fields["tgl_kirim_sampai"] = "use YYYY-MM-DD"
		}
		if err1 == nil && err2 == nil {
			if sampai.Before(dari) {
				fields["tgl_kirim_sampai"] = "must not be before tgl_kirim_dari"
			}
			days := sampai.Sub(dari).Hours() / 24
			if days > 31 {
				fields["tgl_kirim_sampai"] = "range must not exceed 31 days"
			}
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// ValidateAlasan checks a mandatory reason's trimmed length (min..max inclusive).
func ValidateAlasan(field, alasan string, min, max int) error {
	trimmed := strings.TrimSpace(alasan)
	if len([]rune(trimmed)) < min || len([]rune(trimmed)) > max {
		return &ValidationError{Fields: map[string]string{field: fmt.Sprintf("must be %d-%d characters", min, max)}}
	}
	return nil
}

// FormatMoney formats a float64 as string with 2 decimal places.
func FormatMoney(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

// POItemRekap is the rekap view for a po_item.
type POItemRekap struct {
	POItemID        int64
	QtyDiterima     float64
	QtyDitolak      float64
	NilaiPotongNota float64
	Terpenuhi       bool
	NilaiTagihan    float64
}

// PenerimaanFilter scopes GET /penerimaan.
type PenerimaanFilter struct {
	Dari      *time.Time
	Sampai    *time.Time
	BahanID   *int64
	PemasokID *int64
	POID      *int64
	Kondisi   *string
	KodeLot   *string
	Tersedia  *bool
	SPPGID    *int64
	Page      int
	Limit     int
}

func (f *PenerimaanFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
}

func (f PenerimaanFilter) Offset() int { return (f.Page - 1) * f.Limit }

// Berkas mirrors the berkas table.
type Berkas struct {
	ID           int64
	SPPGID       int64
	ObjectKey    string
	MimeType     string
	UkuranByte   int
	SHA256       string
	Konteks      string
	DiunggahOleh int64
	DiunggahAt   time.Time
	RefTabel     *string
	RefID        *int64
	DipakaiAt    *time.Time
}

// SiklusDana mirrors the siklus_dana table.
type SiklusDana struct {
	ID             int64
	SPPGID         int64
	PeriodeMulai   time.Time
	PeriodeSelesai time.Time
	DanaDiajukan   float64
	DanaDiterima   *float64
	SisaSiklusLalu float64
	Status         string
	DiajukanAt     *time.Time
	DicairkanAt    *time.Time
	DilaporkanAt   *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

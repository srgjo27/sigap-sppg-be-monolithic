package domain

import (
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

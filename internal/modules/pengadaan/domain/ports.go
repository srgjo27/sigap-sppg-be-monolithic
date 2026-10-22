package domain

import (
	"context"
	"time"
)

// PurchaseOrderRepository is the persistence port for purchase orders.
type PurchaseOrderRepository interface {
	ListBySPPGAndStatus(ctx context.Context, sppgID int64, statuses []string) ([]PurchaseOrder, error)
	ListItemsByPOIDs(ctx context.Context, poIDs []int64) (map[int64][]POItem, error)
	ListPOMenus(ctx context.Context, menuIDs []int64) (map[int64][]int64, error)
	// Additional methods for full PO lifecycle
	Create(ctx context.Context, po *PurchaseOrder, items []POItem, menuIDs []int64) (*PurchaseOrder, []POItem, error)
	FindByID(ctx context.Context, id int64) (*PurchaseOrder, error)
	Update(ctx context.Context, po *PurchaseOrder, items *[]POItem, menuIDs *[]int64) (*PurchaseOrder, []POItem, error)
	Delete(ctx context.Context, id int64) error
	SetStatus(ctx context.Context, id int64, status string, actorID int64, alasan *string) (*PurchaseOrder, error)
}

// PenerimaanRepository is the persistence port for penerimaan.
type PenerimaanRepository interface {
	Create(ctx context.Context, lots []*PenerimaanBahan, berkasIDs []int64) ([]*PenerimaanBahan, error)
	ListBySPPG(ctx context.Context, sppgID int64, filter PenerimaanFilter) ([]PenerimaanBahan, int, error)
	FindByID(ctx context.Context, id int64) (*PenerimaanBahan, error)
	Update(ctx context.Context, pb *PenerimaanBahan) (*PenerimaanBahan, error)
	GetPOItemRekap(ctx context.Context, poItemIDs []int64) (map[int64]POItemRekap, error)
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

// BerkasRepository is the persistence port for berkas.
type BerkasRepository interface {
	Create(ctx context.Context, b *Berkas) (*Berkas, error)
	FindByID(ctx context.Context, id int64) (*Berkas, error)
	MarkUsed(ctx context.Context, id int64, refTabel string, refID int64) error
	ListUnusedOlderThan(ctx context.Context, olderThan time.Time) ([]Berkas, error)
	Delete(ctx context.Context, id int64) error
}

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

// SiklusRepository is the persistence port for siklus_dana.
type SiklusRepository interface {
	FindByID(ctx context.Context, id int64) (*SiklusDana, error)
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

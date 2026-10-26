package application

import (
	"context"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// PemasokRepository is the persistence port for master pemasok.
type PemasokRepository interface {
	Create(ctx context.Context, p *domain.Pemasok) (*domain.Pemasok, error)
	FindByID(ctx context.Context, id int64) (*domain.Pemasok, error)
	FindByNameInsensitive(ctx context.Context, sppgID int64, nama string) (*domain.Pemasok, error)
	Update(ctx context.Context, p *domain.Pemasok) (*domain.Pemasok, error)
	List(ctx context.Context, sppgID int64, filter domain.PemasokFilter) ([]domain.Pemasok, int, error)
	// GetKinerja computes performance metrics for the last 90 days.
	GetKinerja(ctx context.Context, sppgID, pemasokID int64) (*domain.Kinerja, error)
}

// BahanRepository is the persistence port for master bahan (reused from menu module).
type BahanRepository interface {
	FindByID(ctx context.Context, id int64) (*domain.Bahan, error)
	List(ctx context.Context, filter domain.BahanFilter) ([]domain.Bahan, int, error)
}

// MenuRepository is the persistence port for menu (reused from menu module).
type MenuRepository interface {
	ListByScope(ctx context.Context, sppgID *int64, dari, sampai time.Time, status *string) ([]domain.Menu, error)
	ListItems(ctx context.Context, menuIDs []int64) (map[int64][]domain.MenuBahan, error)
	FindByID(ctx context.Context, id int64) (*domain.Menu, error)
	FindByIDs(ctx context.Context, ids []int64) ([]*domain.Menu, error)
}

// SiklusRepository is the persistence port for siklus_dana.
type SiklusRepository interface {
	FindByID(ctx context.Context, id int64) (*domain.SiklusDana, error)
}

// PurchaseOrderRepository is the persistence port for purchase orders.
type PurchaseOrderRepository interface {
	ListBySPPGAndStatus(ctx context.Context, sppgID int64, statuses []string) ([]domain.PurchaseOrder, error)
	ListItemsByPOIDs(ctx context.Context, poIDs []int64) (map[int64][]domain.POItem, error)
	ListPOMenus(ctx context.Context, menuIDs []int64) (map[int64][]int64, error) // menu_id -> po_ids
	// MVP-003.3 & MVP-003.4
	Create(ctx context.Context, po *domain.PurchaseOrder, items []domain.POItem, menuIDs []int64) (*domain.PurchaseOrder, []domain.POItem, error)
	FindByID(ctx context.Context, id int64) (*domain.PurchaseOrder, error)
	List(ctx context.Context, sppgID int64, filter domain.POFilter) ([]domain.POListItem, int, error)
	GetDetail(ctx context.Context, id int64) (*domain.PODetail, error)
	GetPOItemRekap(ctx context.Context, poItemIDs []int64) (map[int64]domain.POItemRekap, error)
	// For PO number generation
	Exec(ctx context.Context, query string, args ...any) error
	QueryRow(ctx context.Context, query string, args ...any) interface{ Scan(dest ...any) error }
}

// SPPGProvider resolves SPPG info.
type SPPGProvider interface {
	GetKode(ctx context.Context, id int64) (string, error)
}

// AuditRepository appends audit entries.
type AuditRepository interface {
	Append(ctx context.Context, e *authdomain.AuditEntry) error
}

// NotificationRepository delivers notifications.
type NotificationRepository interface {
	NotifySPPGRoles(ctx context.Context, sppgID int64, roles []string, jenis, judul, isi, refTabel string, refID int64) error
}

// Clock abstracts time for tests.
type Clock func() time.Time

// Service orchestrates pengadaan use cases.
type Service struct {
	pemasok PemasokRepository
	bahan   BahanRepository
	menus   MenuRepository
	siklus  SiklusRepository
	po      PurchaseOrderRepository
	sppg    SPPGProvider
	audits  AuditRepository
	notifs  NotificationRepository
	clock   Clock
}

// Deps wires the service without a framework.
type Deps struct {
	Pemasok PemasokRepository
	Bahan   BahanRepository
	Menus   MenuRepository
	Siklus  SiklusRepository
	PO      PurchaseOrderRepository
	SPPG    SPPGProvider
	Audits  AuditRepository
	Notifs  NotificationRepository
	Clock   Clock
}

// New builds a Service with sensible defaults.
func New(d Deps) *Service {
	clock := d.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Service{
		pemasok: d.Pemasok,
		bahan:   d.Bahan,
		menus:   d.Menus,
		siklus:  d.Siklus,
		po:      d.PO,
		sppg:    d.SPPG,
		audits:  d.Audits,
		notifs:  d.Notifs,
		clock:   clock,
	}
}

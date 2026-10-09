package application

import (
	"context"
	"time"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
)

// BahanRepository is the persistence port for master bahan.
type BahanRepository interface {
	Create(ctx context.Context, b *domain.Bahan) (*domain.Bahan, error)
	FindByID(ctx context.Context, id int64) (*domain.Bahan, error)
	FindByNameInsensitive(ctx context.Context, nama string) (*domain.Bahan, error)
	Update(ctx context.Context, b *domain.Bahan) (*domain.Bahan, error)
	List(ctx context.Context, filter domain.BahanFilter) ([]domain.Bahan, int, error)
}

// MenuRepository persists menus atomically with their composition.
type MenuRepository interface {
	// CreateMenu must store menu and items in one transaction.
	CreateMenu(ctx context.Context, m *domain.Menu, items []domain.MenuBahan) (*domain.Menu, []domain.MenuBahan, error)
	ExistsBySPPGDate(ctx context.Context, sppgID int64, tanggal time.Time) (bool, error)
}

// SPPGProvider resolves SPPG capacity for warnings.
type SPPGProvider interface {
	// GetCapacity returns (capacity, found, error). Capacity may be nil when unset.
	GetCapacity(ctx context.Context, sppgID int64) (*int, bool, error)
}

// SekolahProvider sums recipients for default target_porsi.
type SekolahProvider interface {
	SumRecipients(ctx context.Context, sppgID int64) (int, error)
}

// AuditRepository appends audit entries. Reuses auth audit shape.
type AuditRepository interface {
	Append(ctx context.Context, e *authdomain.AuditEntry) error
}

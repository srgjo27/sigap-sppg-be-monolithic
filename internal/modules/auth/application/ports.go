package application

import (
	"context"
	"time"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// UserRepository is the persistence port for user accounts.
type UserRepository interface {
	CreateUser(ctx context.Context, u *domain.User) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id int64) (*domain.User, error)
	Update(ctx context.Context, u *domain.User) (*domain.User, error)
	EmailExists(ctx context.Context, email string, excludeID *int64) (bool, error)
}

// RefreshTokenRepository stores hashed refresh tokens.
type RefreshTokenRepository interface {
	StoreRefresh(ctx context.Context, t *domain.RefreshToken) (*domain.RefreshToken, error)
	FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error)
	Revoke(ctx context.Context, hash string, now time.Time) error
	RevokeAllForUser(ctx context.Context, userID int64, now time.Time) error
}

// AuditRepository appends and lists audit entries.
type AuditRepository interface {
	Append(ctx context.Context, e *domain.AuditEntry) error
	List(ctx context.Context, filter domain.AuditFilter, scopeSPPGID *int64) ([]domain.AuditEntry, int, error)
}

// SPPGChecker verifies sppg existence.
type SPPGChecker interface {
	Exists(ctx context.Context, id int64) (bool, error)
}

// SekolahChecker verifies sekolah existence and ownership.
type SekolahChecker interface {
	// FindOwner returns (found, sppgID, error).
	FindOwner(ctx context.Context, sekolahID int64) (bool, int64, error)
}

// TokenIssuer issues and parses access tokens.
type TokenIssuer interface {
	IssueAccess(user *domain.User, ttl time.Duration) (token string, expiresAt time.Time, err error)
	ParseAccess(token string) (*domain.Claims, error)
}

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

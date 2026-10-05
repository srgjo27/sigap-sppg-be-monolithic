package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// Postgres implements all persistence ports using pgx.
// Schema source of truth: db/schema.sql (users, refresh_token, audit_log).
type Postgres struct {
	Pool *pgxpool.Pool
}

// NewPostgres builds stores around an existing pool.
func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{Pool: pool}
}

const userColumns = `id, sppg_id, sekolah_id, nama, email, no_hp, password_hash, peran, aktif, last_login_at, gagal_login, terkunci_sampai, wajib_ganti_password, created_at, updated_at`

func scanUser(row interface{ Scan(...any) error }) (*domain.User, error) {
	var u domain.User
	var sppgID, sekolahID sql.NullInt64
	var noHP, passwordHash, peran sql.NullString
	var nama, email sql.NullString
	var aktif, wajib sql.NullBool
	var lastLogin, lockedUntil, createdAt, updatedAt sql.NullTime
	var gagalLogin sql.NullInt32
	var id sql.NullInt64
	if err := row.Scan(&id, &sppgID, &sekolahID, &nama, &email, &noHP, &passwordHash, &peran, &aktif, &lastLogin, &gagalLogin, &lockedUntil, &wajib, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	u.ID = id.Int64
	if sppgID.Valid {
		v := sppgID.Int64
		u.SPPGID = &v
	}
	if sekolahID.Valid {
		v := sekolahID.Int64
		u.SekolahID = &v
	}
	u.Nama = nama.String
	u.Email = email.String
	if noHP.Valid {
		v := noHP.String
		u.NoHP = &v
	}
	u.PasswordHash = passwordHash.String
	u.Peran = domain.Role(peran.String)
	u.Aktif = aktif.Bool
	if lastLogin.Valid {
		v := lastLogin.Time
		u.LastLoginAt = &v
	}
	if gagalLogin.Valid {
		u.GagalLogin = int(gagalLogin.Int32)
	}
	if lockedUntil.Valid {
		v := lockedUntil.Time
		u.TerkunciSampai = &v
	}
	u.WajibGantiPassword = wajib.Bool
	if createdAt.Valid {
		u.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		u.UpdatedAt = updatedAt.Time
	}
	return &u, nil
}

// CreateUser inserts a user.
func (p *Postgres) CreateUser(ctx context.Context, u *domain.User) (*domain.User, error) {
	const q = `INSERT INTO users (sppg_id, sekolah_id, nama, email, no_hp, password_hash, peran, aktif, wajib_ganti_password, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7::user_role,$8,$9,now(),now())
		RETURNING ` + userColumns
	row := p.Pool.QueryRow(ctx, q,
		nullableInt(u.SPPGID), nullableInt(u.SekolahID), u.Nama, u.Email,
		nullableStr(u.NoHP), u.PasswordHash, string(u.Peran), u.Aktif, u.WajibGantiPassword,
	)
	created, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return created, nil
}

// FindByEmail looks up by case-insensitive email.
func (p *Postgres) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE lower(email)=lower($1) LIMIT 1`
	row := p.Pool.QueryRow(ctx, q, email)
	u, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

// FindByID looks up by id.
func (p *Postgres) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE id=$1 LIMIT 1`
	row := p.Pool.QueryRow(ctx, q, id)
	u, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

// Update persists a user.
func (p *Postgres) Update(ctx context.Context, u *domain.User) (*domain.User, error) {
	const q = `UPDATE users SET sppg_id=$2, sekolah_id=$3, nama=$4, no_hp=$5, password_hash=$6, peran=$7::user_role,
		aktif=$8, last_login_at=$9, gagal_login=$10, terkunci_sampai=$11, wajib_ganti_password=$12, updated_at=now()
		WHERE id=$1 RETURNING ` + userColumns
	row := p.Pool.QueryRow(ctx, q, u.ID,
		nullableInt(u.SPPGID), nullableInt(u.SekolahID), u.Nama, nullableStr(u.NoHP),
		u.PasswordHash, string(u.Peran), u.Aktif, nullableTime(u.LastLoginAt),
		u.GagalLogin, nullableTime(u.TerkunciSampai), u.WajibGantiPassword,
	)
	updated, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("update user: %w", err)
	}
	return updated, nil
}

// EmailExists checks uniqueness excluding an id.
func (p *Postgres) EmailExists(ctx context.Context, email string, excludeID *int64) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM users WHERE lower(email)=lower($1) AND ($2::bigint IS NULL OR id<>$2))`
	var exists bool
	if err := p.Pool.QueryRow(ctx, q, email, nullableInt(excludeID)).Scan(&exists); err != nil {
		return false, fmt.Errorf("check email: %w", err)
	}
	return exists, nil
}

// StoreRefresh inserts a hashed refresh token.
func (p *Postgres) StoreRefresh(ctx context.Context, t *domain.RefreshToken) (*domain.RefreshToken, error) {
	const q = `INSERT INTO refresh_token (user_id, token_hash, user_agent, ip_address, expires_at, created_at)
		VALUES ($1,$2,$3,$4::inet,$5,now()) RETURNING id, created_at`
	var id int64
	var created time.Time
	if err := p.Pool.QueryRow(ctx, q, t.UserID, t.TokenHash, nullableStr(t.UserAgent), nullableStr(t.IPAddress), t.ExpiresAt).Scan(&id, &created); err != nil {
		return nil, fmt.Errorf("insert refresh: %w", err)
	}
	cp := *t
	cp.ID = id
	cp.CreatedAt = created
	return &cp, nil
}

// FindByHash finds a refresh token.
func (p *Postgres) FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	const q = `SELECT id, user_id, token_hash, user_agent, ip_address::text, expires_at, revoked_at, created_at FROM refresh_token WHERE token_hash=$1 LIMIT 1`
	var t domain.RefreshToken
	var ua, ip sql.NullString
	var revoked sql.NullTime
	var created, expires time.Time
	var id, userID int64
	var tokenHash string
	if err := p.Pool.QueryRow(ctx, q, hash).Scan(&id, &userID, &tokenHash, &ua, &ip, &expires, &revoked, &created); err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find refresh: %w", err)
	}
	t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.CreatedAt = id, userID, tokenHash, expires, created
	if ua.Valid {
		v := ua.String
		t.UserAgent = &v
	}
	if ip.Valid {
		v := ip.String
		t.IPAddress = &v
	}
	if revoked.Valid {
		v := revoked.Time
		t.RevokedAt = &v
	}
	return &t, nil
}

// Revoke marks a refresh token revoked.
func (p *Postgres) Revoke(ctx context.Context, hash string, now time.Time) error {
	const q = `UPDATE refresh_token SET revoked_at=$2 WHERE token_hash=$1 AND revoked_at IS NULL`
	if _, err := p.Pool.Exec(ctx, q, hash, now); err != nil {
		return fmt.Errorf("revoke refresh: %w", err)
	}
	return nil
}

// RevokeAllForUser revokes all active tokens for a user.
func (p *Postgres) RevokeAllForUser(ctx context.Context, userID int64, now time.Time) error {
	const q = `UPDATE refresh_token SET revoked_at=$2 WHERE user_id=$1 AND revoked_at IS NULL`
	if _, err := p.Pool.Exec(ctx, q, userID, now); err != nil {
		return fmt.Errorf("revoke all refresh: %w", err)
	}
	return nil
}

// Append writes an audit entry.
func (p *Postgres) Append(ctx context.Context, e *domain.AuditEntry) error {
	const q = `INSERT INTO audit_log (user_id, aksi, tabel, record_id, data_lama, data_baru, ip_address)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7::inet)`
	if _, err := p.Pool.Exec(ctx, q,
		nullableInt(e.UserID), e.Aksi, e.Tabel, nullableInt(e.RecordID),
		nullableStr(e.DataLama), nullableStr(e.DataBaru), nullableStr(e.IPAddress),
	); err != nil {
		return fmt.Errorf("append audit: %w", err)
	}
	return nil
}

// List returns paginated audit entries with optional SPPG scoping.
func (p *Postgres) List(ctx context.Context, filter domain.AuditFilter, scopeSPPGID *int64) ([]domain.AuditEntry, int, error) {
	filter.Normalize()
	where := `WHERE ($5::bigint IS NULL OR u.sppg_id=$5) AND ($1::text IS NULL OR a.aksi=$1) AND ($2::text IS NULL OR a.tabel=$2) AND ($3::bigint IS NULL OR a.user_id=$3)`
	countQ := `SELECT COUNT(*) FROM audit_log a LEFT JOIN users u ON u.id=a.user_id ` + where
	var total int
	if err := p.Pool.QueryRow(ctx, countQ, nullableStr(filter.Aksi), nullableStr(filter.Tabel), nullableInt(filter.UserID), nil, nullableInt(scopeSPPGID)).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit: %w", err)
	}
	var listQ = `SELECT a.id, a.user_id, a.aksi, a.tabel, a.record_id, a.data_lama::text, a.data_baru::text, a.ip_address::text, a.created_at
		FROM audit_log a LEFT JOIN users u ON u.id=a.user_id ` + where + ` ORDER BY a.id DESC LIMIT $6 OFFSET $7`
	rows, err := p.Pool.Query(ctx, listQ,
		nullableStr(filter.Aksi), nullableStr(filter.Tabel), nullableInt(filter.UserID), nil, nullableInt(scopeSPPGID), filter.Limit, filter.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()
	var out []domain.AuditEntry
	for rows.Next() {
		var e domain.AuditEntry
		var userID, recordID sql.NullInt64
		var dataLama, dataBaru, ip sql.NullString
		if err := rows.Scan(&e.ID, &userID, &e.Aksi, &e.Tabel, &recordID, &dataLama, &dataBaru, &ip, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit: %w", err)
		}
		if userID.Valid {
			v := userID.Int64
			e.UserID = &v
		}
		if recordID.Valid {
			v := recordID.Int64
			e.RecordID = &v
		}
		if dataLama.Valid {
			v := dataLama.String
			e.DataLama = &v
		}
		if dataBaru.Valid {
			v := dataBaru.String
			e.DataBaru = &v
		}
		if ip.Valid {
			v := ip.String
			e.IPAddress = &v
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows audit: %w", err)
	}
	if out == nil {
		out = []domain.AuditEntry{}
	}
	return out, total, nil
}

// Exists checks sppg presence.
func (p *Postgres) Exists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	if err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sppg WHERE id=$1)`, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("check sppg: %w", err)
	}
	return exists, nil
}

// FindOwner returns sekolah ownership.
func (p *Postgres) FindOwner(ctx context.Context, sekolahID int64) (bool, int64, error) {
	var sppgID int64
	if err := p.Pool.QueryRow(ctx, `SELECT sppg_id FROM sekolah WHERE id=$1`, sekolahID).Scan(&sppgID); err != nil {
		if isNoRows(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("find sekolah: %w", err)
	}
	return true, sppgID, nil
}

func nullableInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableTime(p *time.Time) any {
	if p == nil {
		return nil
	}
	return *p
}

func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	// pgx returns "no rows in result set".
	return err.Error() == "no rows in result set"
}

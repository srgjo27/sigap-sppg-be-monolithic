package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// Clock abstracts time for tests.
type Clock func() time.Time

// AccountLockedError carries the unlock time for HTTP 423 responses.
type AccountLockedError struct {
	Until time.Time
}

func (e *AccountLockedError) Error() string { return domain.ErrAccountLocked.Error() }

// Service orchestrates MVP-001 use cases.
type Service struct {
	users      UserRepository
	refresh    RefreshTokenRepository
	audits     AuditRepository
	sppg       SPPGChecker
	sekolah    SekolahChecker
	tokens     TokenIssuer
	hasher     PasswordHasher
	clock      Clock
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// Deps wires the service without a framework.
type Deps struct {
	Users      UserRepository
	Refresh    RefreshTokenRepository
	Audits     AuditRepository
	SPPG       SPPGChecker
	Sekolah    SekolahChecker
	Tokens     TokenIssuer
	Hasher     PasswordHasher
	Clock      Clock
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// New builds a Service with sensible defaults.
func New(d Deps) *Service {
	clock := d.Clock
	if clock == nil {
		clock = time.Now
	}
	accessTTL := d.AccessTTL
	if accessTTL == 0 {
		accessTTL = domain.AccessTokenTTL
	}
	refreshTTL := d.RefreshTTL
	if refreshTTL == 0 {
		refreshTTL = domain.RefreshTokenTTL
	}
	return &Service{
		users:      d.Users,
		refresh:    d.Refresh,
		audits:     d.Audits,
		sppg:       d.SPPG,
		sekolah:    d.Sekolah,
		tokens:     d.Tokens,
		hasher:     d.Hasher,
		clock:      clock,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// CreateUserResult carries the created user plus a generated password (once).
type CreateUserResult struct {
	User              *domain.User
	GeneratedPassword *string
}

// CreateUser implements MVP-001.1.
func (s *Service) CreateUser(ctx context.Context, actor *domain.Claims, in domain.CreateUserInput, ip *string) (*CreateUserResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	actorRole := actor.Role
	if !actorRole.CanManageUsers() {
		s.deny(ctx, &actor.UserID, "users: role not allowed", ip)
		return nil, domain.ErrForbidden
	}
	if actorRole == domain.RoleKepalaSPPG {
		if domain.KepalaManagedForbidden(in.Peran) {
			s.deny(ctx, &actor.UserID, "users: kepala cannot manage role", ip)
			return nil, domain.ErrForbidden
		}
		if actor.SPPGID == nil || in.SPPGID == nil || *in.SPPGID != *actor.SPPGID {
			s.deny(ctx, &actor.UserID, "users: cross-sppg forbidden", ip)
			return nil, domain.ErrForbidden
		}
	}
	if err := domain.ValidateCreate(in); err != nil {
		return nil, err
	}
	email := domain.NormalizeEmail(in.Email)
	exists, err := s.users.EmailExists(ctx, email, nil)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if exists {
		return nil, domain.ErrEmailExists
	}
	if err := s.checkScopeRefs(ctx, in.SPPGID, in.SekolahID, in.Peran); err != nil {
		return nil, err
	}

	plain := ""
	if in.PasswordAwal != nil && *in.PasswordAwal != "" {
		plain = *in.PasswordAwal
	} else {
		generated, err := domain.GenerateRandomPassword()
		if err != nil {
			return nil, err
		}
		plain = generated
	}
	hash, err := s.hasher.Hash(plain)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	now := s.clock()
	u := &domain.User{
		SPPGID:             in.SPPGID,
		SekolahID:          in.SekolahID,
		Nama:               in.Nama,
		Email:              email,
		NoHP:               copyStrPtr(in.NoHP),
		PasswordHash:       hash,
		Peran:              in.Peran,
		Aktif:              true,
		WajibGantiPassword: true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	created, err := s.users.CreateUser(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	_ = s.audits.Append(ctx, &domain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      domain.AuditCreate,
		Tabel:     domain.AuditTableUsers,
		RecordID:  &created.ID,
		DataBaru:  strPtr(mustJSON(map[string]any{"email": created.Email, "peran": string(created.Peran)})),
		IPAddress: ip,
		CreatedAt: now,
	})
	res := &CreateUserResult{User: created}
	if in.PasswordAwal == nil || *in.PasswordAwal == "" {
		res.GeneratedPassword = &plain
	}
	return res, nil
}

// LoginResult is the MVP-001.2 success payload.
type LoginResult struct {
	AccessToken        string
	RefreshToken       string
	ExpiresIn          int64
	WajibGantiPassword bool
	User               *domain.User
}

// Login implements MVP-001.2 with lockout and audit.
func (s *Service) Login(ctx context.Context, email, password string, ip *string, userAgent *string) (*LoginResult, error) {
	now := s.clock()
	normalized := domain.NormalizeEmail(email)
	if normalized == "" || password == "" {
		return nil, &domain.ValidationError{Fields: map[string]string{"email": "required", "password": "required"}}
	}
	u, err := s.users.FindByEmail(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	if u == nil {
		_ = s.audits.Append(ctx, &domain.AuditEntry{
			Aksi:      domain.AuditLogin,
			Tabel:     domain.AuditTableAuth,
			DataBaru:  strPtr(mustJSON(map[string]any{"success": false, "email": normalized})),
			IPAddress: ip,
			CreatedAt: now,
		})
		return nil, domain.ErrInvalidCredentials
	}
	if u.IsLocked(now) {
		return nil, &AccountLockedError{Until: *u.TerkunciSampai}
	}
	if !u.Aktif {
		_ = s.audits.Append(ctx, &domain.AuditEntry{
			UserID:    &u.ID,
			Aksi:      domain.AuditLogin,
			Tabel:     domain.AuditTableAuth,
			RecordID:  &u.ID,
			DataBaru:  strPtr(mustJSON(map[string]any{"success": false, "reason": "inactive"})),
			IPAddress: ip,
			CreatedAt: now,
		})
		return nil, domain.ErrAccountInactive
	}
	if err := s.hasher.Compare(u.PasswordHash, password); err != nil {
		u.RecordFailedLogin(now)
		_, _ = s.users.Update(ctx, u)
		_ = s.audits.Append(ctx, &domain.AuditEntry{
			UserID:    &u.ID,
			Aksi:      domain.AuditLogin,
			Tabel:     domain.AuditTableAuth,
			RecordID:  &u.ID,
			DataBaru:  strPtr(mustJSON(map[string]any{"success": false})),
			IPAddress: ip,
			CreatedAt: now,
		})
		return nil, domain.ErrInvalidCredentials
	}
	u.RecordSuccessfulLogin(now)
	updated, err := s.users.Update(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("persist login: %w", err)
	}
	access, expiresAt, err := s.tokens.IssueAccess(updated, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("issue access: %w", err)
	}
	plain, hash, err := domain.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	_, err = s.refresh.StoreRefresh(ctx, &domain.RefreshToken{
		UserID:    updated.ID,
		TokenHash: hash,
		UserAgent: userAgent,
		IPAddress: ip,
		ExpiresAt: now.Add(s.refreshTTL),
		CreatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("store refresh: %w", err)
	}
	_ = s.audits.Append(ctx, &domain.AuditEntry{
		UserID:    &updated.ID,
		Aksi:      domain.AuditLogin,
		Tabel:     domain.AuditTableAuth,
		RecordID:  &updated.ID,
		DataBaru:  strPtr(mustJSON(map[string]any{"success": true})),
		IPAddress: ip,
		CreatedAt: now,
	})
	return &LoginResult{
		AccessToken:        access,
		RefreshToken:       plain,
		ExpiresIn:          int64(time.Until(expiresAt).Seconds()),
		WajibGantiPassword: updated.WajibGantiPassword,
		User:               updated,
	}, nil
}

// RefreshResult carries rotated tokens (MVP-001.3).
type RefreshResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

// Refresh rotates a refresh token (MVP-001.3).
// Reuse of a revoked token triggers theft detection: all sessions revoked.
func (s *Service) Refresh(ctx context.Context, plainRefresh string, ip *string, userAgent *string) (*RefreshResult, error) {
	now := s.clock()
	if plainRefresh == "" {
		return nil, domain.ErrRefreshInvalid
	}
	hash := domain.HashRefreshToken(plainRefresh)
	stored, err := s.refresh.FindByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("find refresh: %w", err)
	}
	if stored == nil {
		return nil, domain.ErrRefreshInvalid
	}
	if stored.RevokedAt != nil {
		_ = s.refresh.RevokeAllForUser(ctx, stored.UserID, now)
		_ = s.audits.Append(ctx, &domain.AuditEntry{
			UserID:    &stored.UserID,
			Aksi:      domain.AuditRefresh,
			Tabel:     domain.AuditTableAuth,
			RecordID:  &stored.UserID,
			DataBaru:  strPtr(mustJSON(map[string]any{"reused_revoked": true})),
			IPAddress: ip,
			CreatedAt: now,
		})
		return nil, domain.ErrRefreshRevoked
	}
	if now.After(stored.ExpiresAt) {
		return nil, domain.ErrRefreshExpired
	}
	u, err := s.users.FindByID(ctx, stored.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	if u == nil {
		return nil, domain.ErrNotFound
	}
	if !u.Aktif {
		return nil, domain.ErrAccountInactive
	}
	if u.WajibGantiPassword {
		s.deny(ctx, &u.ID, "refresh: password change required", ip)
		return nil, domain.ErrMustChangePassword
	}
	if u.IsLocked(now) {
		return nil, &AccountLockedError{Until: *u.TerkunciSampai}
	}
	if err := s.refresh.Revoke(ctx, hash, now); err != nil {
		return nil, fmt.Errorf("revoke old refresh: %w", err)
	}
	newPlain, newHash, err := domain.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	if _, err := s.refresh.StoreRefresh(ctx, &domain.RefreshToken{
		UserID:    u.ID,
		TokenHash: newHash,
		UserAgent: userAgent,
		IPAddress: ip,
		ExpiresAt: now.Add(s.refreshTTL),
		CreatedAt: now,
	}); err != nil {
		return nil, fmt.Errorf("store refresh: %w", err)
	}
	access, expiresAt, err := s.tokens.IssueAccess(u, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("issue access: %w", err)
	}
	_ = s.audits.Append(ctx, &domain.AuditEntry{
		UserID:    &u.ID,
		Aksi:      domain.AuditRefresh,
		Tabel:     domain.AuditTableAuth,
		RecordID:  &u.ID,
		IPAddress: ip,
		CreatedAt: now,
	})
	return &RefreshResult{
		AccessToken:  access,
		RefreshToken: newPlain,
		ExpiresIn:    int64(time.Until(expiresAt).Seconds()),
	}, nil
}

// Logout revokes a refresh token (MVP-001.3). Idempotent.
func (s *Service) Logout(ctx context.Context, plainRefresh string, ip *string) error {
	now := s.clock()
	if plainRefresh == "" {
		return nil
	}
	hash := domain.HashRefreshToken(plainRefresh)
	stored, err := s.refresh.FindByHash(ctx, hash)
	if err != nil {
		return fmt.Errorf("find refresh: %w", err)
	}
	if stored == nil {
		return nil
	}
	if err := s.refresh.Revoke(ctx, hash, now); err != nil {
		return fmt.Errorf("revoke refresh: %w", err)
	}
	_ = s.audits.Append(ctx, &domain.AuditEntry{
		UserID:    &stored.UserID,
		Aksi:      domain.AuditLogout,
		Tabel:     domain.AuditTableAuth,
		RecordID:  &stored.UserID,
		IPAddress: ip,
		CreatedAt: now,
	})
	return nil
}

// LogoutAll revokes every session of a user (MVP-001.3 ?all=true).
func (s *Service) LogoutAll(ctx context.Context, userID int64, ip *string) error {
	now := s.clock()
	if err := s.refresh.RevokeAllForUser(ctx, userID, now); err != nil {
		return fmt.Errorf("revoke all refresh: %w", err)
	}
	_ = s.audits.Append(ctx, &domain.AuditEntry{
		UserID:    &userID,
		Aksi:      domain.AuditLogout,
		Tabel:     domain.AuditTableAuth,
		RecordID:  &userID,
		DataBaru:  strPtr(mustJSON(map[string]any{"all": true})),
		IPAddress: ip,
		CreatedAt: now,
	})
	return nil
}

// Profile is the enriched MVP-001.4 current-user payload.
type Profile struct {
	User        *domain.User
	SPPG        *domain.SPPGInfo
	Sekolah     *domain.SekolahInfo
	Permissions []string
}

// Me returns the active profile plus permissions (MVP-001.4).
// Data comes from the database so role/status changes apply immediately.
func (s *Service) Me(ctx context.Context, userID int64) (*Profile, error) {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	if u == nil {
		return nil, domain.ErrNotFound
	}
	if !u.Aktif {
		return nil, domain.ErrAccountInactive
	}
	p := &Profile{User: u, Permissions: u.Peran.Permissions()}
	if u.SPPGID != nil {
		if info, err := s.sppg.GetSPPG(ctx, *u.SPPGID); err == nil {
			p.SPPG = info
		} else {
			return nil, fmt.Errorf("get sppg: %w", err)
		}
	}
	if u.SekolahID != nil {
		if info, err := s.sekolah.GetSekolah(ctx, *u.SekolahID); err == nil {
			p.Sekolah = info
		} else {
			return nil, fmt.Errorf("get sekolah: %w", err)
		}
	}
	return p, nil
}

// UpdateUserResult carries the updated user plus a reset password (once).
type UpdateUserResult struct {
	User         *domain.User
	TempPassword *string
}

// UpdateUser implements MVP-001.5.
func (s *Service) UpdateUser(ctx context.Context, actor *domain.Claims, targetID int64, in domain.UpdateUserInput, ip *string) (*UpdateUserResult, error) {
	if actor == nil {
		return nil, domain.ErrUnauthorized
	}
	if !actor.Role.CanManageUsers() {
		s.deny(ctx, &actor.UserID, "users: role not allowed", ip)
		return nil, domain.ErrForbidden
	}
	if err := domain.ValidateUpdate(in); err != nil {
		return nil, err
	}
	target, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	if target == nil {
		return nil, domain.ErrNotFound
	}
	if actor.Role == domain.RoleKepalaSPPG {
		if domain.KepalaManagedForbidden(target.Peran) {
			s.deny(ctx, &actor.UserID, "users: kepala cannot manage role", ip)
			return nil, domain.ErrForbidden
		}
		if in.Peran != nil && domain.KepalaManagedForbidden(*in.Peran) {
			s.deny(ctx, &actor.UserID, "users: kepala cannot assign role", ip)
			return nil, domain.ErrForbidden
		}
		if actor.SPPGID == nil || target.SPPGID == nil || *target.SPPGID != *actor.SPPGID {
			s.deny(ctx, &actor.UserID, "users: cross-sppg forbidden", ip)
			return nil, domain.ErrForbidden
		}
		// Kepala cannot move users outside their SPPG.
		if in.SPPGID != nil && *in.SPPGID != *actor.SPPGID {
			s.deny(ctx, &actor.UserID, "users: cross-sppg move forbidden", ip)
			return nil, domain.ErrForbidden
		}
	}
	oldPeran, oldSPPG, oldSekolah, oldAktif := target.Peran, target.SPPGID, target.SekolahID, target.Aktif
	oldNama, oldNoHP := target.Nama, target.NoHP

	// A user cannot deactivate or demote itself (MVP-001.5).
	if actor.UserID == target.ID {
		if in.Aktif != nil && !*in.Aktif {
			s.deny(ctx, &actor.UserID, "users: self deactivation forbidden", ip)
			return nil, domain.ErrSelfModification
		}
		if in.Peran != nil && *in.Peran != target.Peran {
			s.deny(ctx, &actor.UserID, "users: self demotion forbidden", ip)
			return nil, domain.ErrSelfModification
		}
	}

	if in.Nama != nil {
		target.Nama = *in.Nama
	}
	if in.NoHP != nil {
		if *in.NoHP == "" {
			target.NoHP = nil
		} else {
			target.NoHP = copyStrPtr(in.NoHP)
		}
	}
	if in.Peran != nil {
		target.Peran = *in.Peran
	}
	if in.SPPGID != nil {
		if *in.SPPGID == 0 {
			target.SPPGID = nil
		} else {
			v := *in.SPPGID
			target.SPPGID = &v
		}
	}
	if in.SekolahID != nil {
		if *in.SekolahID == 0 {
			target.SekolahID = nil
		} else {
			v := *in.SekolahID
			target.SekolahID = &v
		}
	}
	if in.Aktif != nil {
		target.Aktif = *in.Aktif
	}
	resetPassword := in.ResetPassword != nil && *in.ResetPassword
	var tempPassword *string
	if resetPassword {
		generated, err := domain.GenerateRandomPassword()
		if err != nil {
			return nil, err
		}
		hash, err := s.hasher.Hash(generated)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		target.PasswordHash = hash
		target.WajibGantiPassword = true
		tempPassword = &generated
	}

	// Re-validate resulting scope shape.
	shape := domain.CreateUserInput{
		Nama:      target.Nama,
		Email:     target.Email,
		NoHP:      target.NoHP,
		Peran:     target.Peran,
		SPPGID:    target.SPPGID,
		SekolahID: target.SekolahID,
	}
	// Bypass password check for updates.
	pw := ""
	shape.PasswordAwal = &pw
	if err := domain.ValidateCreate(shape); err != nil {
		return nil, err
	}
	if err := s.checkScopeRefs(ctx, target.SPPGID, target.SekolahID, target.Peran); err != nil {
		return nil, err
	}
	// Every SPPG keeps at least one active kepala_sppg (MVP-001.5).
	if oldPeran == domain.RoleKepalaSPPG && oldAktif && oldSPPG != nil {
		leaving := !target.Aktif || target.Peran != domain.RoleKepalaSPPG || !equalInt64Ptr(target.SPPGID, oldSPPG)
		if leaving {
			remaining, err := s.users.CountActiveKepala(ctx, *oldSPPG, &target.ID)
			if err != nil {
				return nil, fmt.Errorf("count kepala: %w", err)
			}
			if remaining == 0 {
				return nil, domain.ErrLastKepalaRequired
			}
		}
	}
	now := s.clock()
	target.UpdatedAt = now
	updated, err := s.users.Update(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	// Revoke sessions when access scope changes, on deactivation, or on reset.
	if !updated.Aktif || updated.Peran != oldPeran || !equalInt64Ptr(updated.SPPGID, oldSPPG) || !equalInt64Ptr(updated.SekolahID, oldSekolah) || oldAktif != updated.Aktif || resetPassword {
		_ = s.refresh.RevokeAllForUser(ctx, updated.ID, now)
	}
	oldSnap := map[string]any{
		"nama":       oldNama,
		"no_hp":      nullableString(oldNoHP),
		"peran":      string(oldPeran),
		"sppg_id":    nullableInt64(oldSPPG),
		"sekolah_id": nullableInt64(oldSekolah),
		"aktif":      oldAktif,
	}
	newSnap := map[string]any{
		"nama":       updated.Nama,
		"no_hp":      nullableString(updated.NoHP),
		"peran":      string(updated.Peran),
		"sppg_id":    nullableInt64(updated.SPPGID),
		"sekolah_id": nullableInt64(updated.SekolahID),
		"aktif":      updated.Aktif,
	}
	if resetPassword {
		newSnap["reset_password"] = true
	}
	_ = s.audits.Append(ctx, &domain.AuditEntry{
		UserID:    &actor.UserID,
		Aksi:      domain.AuditUpdate,
		Tabel:     domain.AuditTableUsers,
		RecordID:  &updated.ID,
		DataLama:  strPtr(mustJSON(oldSnap)),
		DataBaru:  strPtr(mustJSON(newSnap)),
		IPAddress: ip,
		CreatedAt: now,
	})
	return &UpdateUserResult{User: updated, TempPassword: tempPassword}, nil
}

// ChangePassword implements MVP-001.6.
// Other sessions are revoked; the newest (current) session is kept.
func (s *Service) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string, ip *string) error {
	if !domain.ValidatePasswordPolicy(newPassword) {
		return &domain.ValidationError{Fields: map[string]string{"password_baru": "minimum 8 characters with letters and numbers"}}
	}
	if oldPassword == newPassword {
		return &domain.ValidationError{Fields: map[string]string{"password_baru": "must differ from password_lama"}}
	}
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}
	if u == nil {
		return domain.ErrNotFound
	}
	if !u.Aktif {
		return domain.ErrAccountInactive
	}
	if err := s.hasher.Compare(u.PasswordHash, oldPassword); err != nil {
		return domain.ErrOldPasswordMismatch
	}
	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := s.clock()
	u.PasswordHash = hash
	u.WajibGantiPassword = false
	u.UpdatedAt = now
	if _, err := s.users.Update(ctx, u); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	latest, err := s.refresh.FindLatestActive(ctx, u.ID)
	if err != nil {
		return fmt.Errorf("find latest refresh: %w", err)
	}
	if latest == nil {
		_ = s.refresh.RevokeAllForUser(ctx, u.ID, now)
	} else {
		_ = s.refresh.RevokeAllExcept(ctx, u.ID, latest.TokenHash, now)
	}
	_ = s.audits.Append(ctx, &domain.AuditEntry{
		UserID:    &u.ID,
		Aksi:      domain.AuditPasswd,
		Tabel:     domain.AuditTableUsers,
		RecordID:  &u.ID,
		IPAddress: ip,
		CreatedAt: now,
	})
	return nil
}

// AuditActor is the nested user reference in audit responses (MVP-001.7).
type AuditActor struct {
	ID    int64
	Nama  string
	Peran string
}

// AuditLogView pairs an entry with its actor reference.
type AuditLogView struct {
	Entry domain.AuditEntry
	Actor *AuditActor
}

// ListAuditLogs implements MVP-001.7 with role scoping.
func (s *Service) ListAuditLogs(ctx context.Context, actor *domain.Claims, filter domain.AuditFilter) ([]AuditLogView, int, error) {
	if actor == nil {
		return nil, 0, domain.ErrUnauthorized
	}
	if !actor.Role.CanViewAuditLogs() {
		s.deny(ctx, &actor.UserID, "audit-logs: role not allowed", nil)
		return nil, 0, domain.ErrForbidden
	}
	if err := filter.Validate(); err != nil {
		return nil, 0, err
	}
	filter.Normalize()
	var scope *int64
	if actor.Role != domain.RoleAdmin {
		if actor.SPPGID == nil {
			s.deny(ctx, &actor.UserID, "audit-logs: missing sppg scope", nil)
			return nil, 0, domain.ErrForbidden
		}
		scope = actor.SPPGID
	}
	entries, total, err := s.audits.List(ctx, filter, scope)
	if err != nil {
		return nil, 0, err
	}
	views := make([]AuditLogView, 0, len(entries))
	cache := map[int64]*AuditActor{}
	for _, e := range entries {
		v := AuditLogView{Entry: e}
		if e.UserID != nil {
			actorRef, ok := cache[*e.UserID]
			if !ok {
				u, err := s.users.FindByID(ctx, *e.UserID)
				if err != nil {
					return nil, 0, fmt.Errorf("find audit actor: %w", err)
				}
				if u != nil {
					actorRef = &AuditActor{ID: u.ID, Nama: u.Nama, Peran: string(u.Peran)}
					cache[u.ID] = actorRef
				} else {
					cache[*e.UserID] = nil
				}
			}
			v.Actor = actorRef
		}
		views = append(views, v)
	}
	return views, total, nil
}

// LogDenial records a 403 denial (MVP-001.8, aksi DENY). Best-effort.
func (s *Service) LogDenial(ctx context.Context, userID *int64, reason string, ip *string) {
	s.deny(ctx, userID, reason, ip)
}

func (s *Service) deny(ctx context.Context, userID *int64, reason string, ip *string) {
	if userID == nil {
		return
	}
	_ = s.audits.Append(ctx, &domain.AuditEntry{
		UserID:    userID,
		Aksi:      domain.AuditDeny,
		Tabel:     domain.AuditTableAuth,
		DataBaru:  strPtr(mustJSON(map[string]any{"reason": reason})),
		IPAddress: ip,
		CreatedAt: s.clock(),
	})
}

func (s *Service) checkScopeRefs(ctx context.Context, sppgID *int64, sekolahID *int64, peran domain.Role) error {
	if sppgID != nil {
		ok, err := s.sppg.Exists(ctx, *sppgID)
		if err != nil {
			return fmt.Errorf("check sppg: %w", err)
		}
		if !ok {
			return domain.ErrSPPGNotFound
		}
	}
	if peran == domain.RolePICsekolah {
		if sekolahID == nil {
			return &domain.ValidationError{Fields: map[string]string{"sekolah_id": "required for pic_sekolah"}}
		}
		found, owner, err := s.sekolah.FindOwner(ctx, *sekolahID)
		if err != nil {
			return fmt.Errorf("check sekolah: %w", err)
		}
		if !found {
			return domain.ErrSekolahNotFound
		}
		if sppgID == nil || owner != *sppgID {
			return domain.ErrSekolahScopeMismatch
		}
		return nil
	}
	if sekolahID != nil {
		found, owner, err := s.sekolah.FindOwner(ctx, *sekolahID)
		if err != nil {
			return fmt.Errorf("check sekolah: %w", err)
		}
		if !found {
			return domain.ErrSekolahNotFound
		}
		if sppgID == nil || owner != *sppgID {
			return domain.ErrSekolahScopeMismatch
		}
	}
	return nil
}

func copyStrPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func strPtr(s string) *string { return &s }

func nullableString(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func mustJSON(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func equalInt64Ptr(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

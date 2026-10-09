package domain

import "time"

// Audit action codes (aksi VARCHAR(10) in schema).
const (
	AuditCreate  = "create"
	AuditUpdate  = "update"
	AuditLogin   = "login"
	AuditLogout  = "logout"
	AuditRefresh = "refresh"
	AuditPasswd  = "passwd"
	AuditDeny    = "DENY"
)

// Audit tables.
const (
	AuditTableUsers = "users"
	AuditTableAuth  = "auth"
)

// AuditEntry mirrors the audit_log table.
type AuditEntry struct {
	ID        int64
	UserID    *int64
	Aksi      string
	Tabel     string
	RecordID  *int64
	DataLama  *string
	DataBaru  *string
	IPAddress *string
	CreatedAt time.Time
}

// AuditFilter scopes GET /audit-logs (MVP-001.7).
type AuditFilter struct {
	Aksi     *string
	Tabel    *string
	UserID   *int64
	RecordID *int64
	Dari     *time.Time
	Sampai   *time.Time
	Page     int
	Limit    int
}

// Validate rejects reversed date ranges (400).
func (f AuditFilter) Validate() error {
	if f.Dari != nil && f.Sampai != nil && f.Sampai.Before(*f.Dari) {
		return &ValidationError{Fields: map[string]string{"sampai": "must not be before dari"}}
	}
	return nil
}

// Normalize applies defaults (page 1, limit 20, max 100).
func (f *AuditFilter) Normalize() {
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
func (f AuditFilter) Offset() int { return (f.Page - 1) * f.Limit }

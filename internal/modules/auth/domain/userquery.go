package domain

import "strings"

// Sort options for GET /users (MVP-002.8).
const (
	UserSortNama         = "nama"
	UserSortMinusCreated = "-created_at"
	UserSortMinusLogin   = "-last_login_at"
)

// UserFilter scopes GET /users (MVP-002.8).
type UserFilter struct {
	Q         *string
	Peran     []Role
	Aktif     *bool
	SPPGID    *int64
	NullSPPG  bool
	SekolahID *int64
	Sort      string
	Page      int
	Limit     int
}

// UserScope enforces MVP-002.8 visibility.
// A nil SPPGID lists across all SPPG (admin); otherwise results are
// restricted to one SPPG. ExcludePrivileged hides admin and pengawas
// accounts from kepala_sppg results.
type UserScope struct {
	SPPGID            *int64
	ExcludePrivileged bool
}

// Normalize applies defaults (sort nama, page 1, limit 20).
func (f *UserFilter) Normalize() {
	if f.Sort == "" {
		f.Sort = UserSortNama
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
}

// Validate rejects bad search, role, sort, and pagination input (400).
func (f UserFilter) Validate() error {
	fields := map[string]string{}
	if f.Q != nil {
		if n := len([]rune(strings.TrimSpace(*f.Q))); n < 2 {
			fields["q"] = "minimum 2 characters"
		}
	}
	for _, r := range f.Peran {
		if !r.Valid() {
			fields["peran"] = "unknown role"
			break
		}
	}
	switch f.Sort {
	case "", UserSortNama, UserSortMinusCreated, UserSortMinusLogin:
	default:
		fields["sort"] = "unknown sort"
	}
	if f.Limit > 100 {
		fields["per_page"] = "max 100"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// Offset returns the SQL offset.
func (f UserFilter) Offset() int { return (f.Page - 1) * f.Limit }

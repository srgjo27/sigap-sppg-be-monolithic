package domain

// Role represents the user_role enum in db/schema.sql.
type Role string

const (
	RoleAdmin             Role = "admin"
	RoleKepalaSPPG        Role = "kepala_sppg"
	RoleAhliGizi          Role = "ahli_gizi"
	RoleAkuntan           Role = "akuntan"
	RolePetugasDapur      Role = "petugas_dapur"
	RolePetugasDistribusi Role = "petugas_distribusi"
	RolePICsekolah        Role = "pic_sekolah"
	RolePengawas          Role = "pengawas"
)

// AllRoles lists every valid role.
func AllRoles() []Role {
	return []Role{
		RoleAdmin,
		RoleKepalaSPPG,
		RoleAhliGizi,
		RoleAkuntan,
		RolePetugasDapur,
		RolePetugasDistribusi,
		RolePICsekolah,
		RolePengawas,
	}
}

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin,
		RoleKepalaSPPG,
		RoleAhliGizi,
		RoleAkuntan,
		RolePetugasDapur,
		RolePetugasDistribusi,
		RolePICsekolah,
		RolePengawas:
		return true
	default:
		return false
	}
}

// CanManageUsers reports whether the role may create/update user accounts
// (MVP-001.1, MVP-001.5).
func (r Role) CanManageUsers() bool {
	return r == RoleAdmin || r == RoleKepalaSPPG
}

// CanViewAuditLogs reports whether the role may read audit logs (MVP-001.7).
func (r Role) CanViewAuditLogs() bool {
	return r == RoleAdmin || r == RoleKepalaSPPG || r == RolePengawas
}

// KepalaManagedForbidden reports whether kepala_sppg is forbidden from
// managing a target role (cannot create admin or pengawas).
func KepalaManagedForbidden(target Role) bool {
	return target == RoleAdmin || target == RolePengawas
}

// Permissions describes coarse capabilities returned by GET /auth/me.
func (r Role) Permissions() []string {
	switch r {
	case RoleAdmin:
		return []string{"users:manage", "audit:read", "data:all"}
	case RoleKepalaSPPG:
		return []string{"users:manage:sppg", "audit:read:sppg", "data:sppg"}
	case RolePengawas:
		return []string{"audit:read:sppg", "data:read:sppg"}
	case RolePICsekolah:
		return []string{"data:sekolah"}
	default:
		return []string{"data:sppg"}
	}
}

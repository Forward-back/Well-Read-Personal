package types

// TenantRole represents the single owner role of the personal edition.
//
// The personal edition has exactly one authenticated principal (the owner), so
// the type survives only as an identity marker for context and API payloads;
// it never gates business access.
type TenantRole string

const (
	// TenantRoleOwner is the only role: the single user owns every knowledge
	// base, skill, session and configuration entry.
	TenantRoleOwner TenantRole = "owner"
)

var tenantRoleLevel = map[TenantRole]int{
	TenantRoleOwner: 100,
}

// IsValid reports whether r is the owner role.
func (r TenantRole) IsValid() bool {
	_, ok := tenantRoleLevel[r]
	return ok
}

// Level returns the numeric privilege level of the role. Unknown roles
// return 0.
func (r TenantRole) Level() int {
	return tenantRoleLevel[r]
}

// Membership is the login-response-friendly projection of the owner's single
// workspace. It carries identity only, never a capability set.
type Membership struct {
	TenantID   uint64     `json:"tenant_id"`
	TenantName string     `json:"tenant_name"`
	Role       TenantRole `json:"role"`
}

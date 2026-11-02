package types

// ACLScopeMode controls how the engine applies a retrieval scope.
type ACLScopeMode string

const (
	ACLScopeAll   ACLScopeMode = "all"
	ACLScopeAllow ACLScopeMode = "allow"
	ACLScopeDeny  ACLScopeMode = "deny"
)

// ACLScope is the engine-facing retrieval scope. The personal edition always
// computes the caller's own tenant-wide corpus.
type ACLScope struct {
	Mode    ACLScopeMode `json:"mode"`
	DocIDs  []string     `json:"document_ids,omitempty"`
	Version int64        `json:"scope_version"`
}

// TenantWideScope is the retrieval scope for the single personal owner: every
// document in the requested knowledge bases is readable.
func TenantWideScope() *ACLScope {
	return &ACLScope{Mode: ACLScopeAll}
}

// Valid reports whether the scope is safe to apply. Unknown modes and allow/
// deny scopes without document IDs are invalid.
func (s *ACLScope) Valid() bool {
	if s == nil {
		return false
	}
	switch s.Mode {
	case ACLScopeAll:
		return true
	case ACLScopeAllow, ACLScopeDeny:
		return len(s.DocIDs) > 0 || s.Mode == ACLScopeAllow
	default:
		return false
	}
}

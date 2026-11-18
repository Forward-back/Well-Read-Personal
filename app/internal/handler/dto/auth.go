package dto

import "gaze/internal/types"

// AuthLoginResponse is the HTTP-safe login response shape.
type AuthLoginResponse struct {
	Success      bool          `json:"success"`
	Message      string        `json:"message,omitempty"`
	User         *types.User   `json:"user,omitempty"`
	Tenant       *types.Tenant `json:"tenant,omitempty"`
	Token        string        `json:"token,omitempty"`
	RefreshToken string        `json:"refresh_token,omitempty"`
}

// AuthOIDCCallbackResponse is the HTTP-safe OIDC callback payload shape.
type AuthOIDCCallbackResponse struct {
	Success      bool               `json:"success"`
	Message      string             `json:"message,omitempty"`
	User         *types.User        `json:"user,omitempty"`
	Tenant       *TenantResponse    `json:"tenant,omitempty"`
	Memberships  []types.Membership `json:"memberships"`
	Caps         []string           `json:"caps"`
	Token        string             `json:"token,omitempty"`
	RefreshToken string             `json:"refresh_token,omitempty"`
	IsNewUser    bool               `json:"is_new_user,omitempty"`
}

// userForAuthResponse maps an uploaded avatar into the legacy avatar field so
// login clients receive it immediately; the raw avatar_data field stays hidden.
func userForAuthResponse(user *types.User) *types.User {
	if user == nil {
		return nil
	}
	clone := *user
	if clone.AvatarData != "" {
		clone.Avatar = clone.AvatarData
	}
	return &clone
}

// NewAuthLoginResponse converts a service-layer login response for HTTP output.
func NewAuthLoginResponse(resp *types.LoginResponse) *AuthLoginResponse {
	if resp == nil {
		return nil
	}
	return &AuthLoginResponse{
		Success:      resp.Success,
		Message:      resp.Message,
		User:         userForAuthResponse(resp.User),
		Tenant:       resp.Tenant,
		Token:        resp.Token,
		RefreshToken: resp.RefreshToken,
	}
}

// NewAuthOIDCCallbackResponse converts an OIDC callback response for HTTP output.
func NewAuthOIDCCallbackResponse(resp *types.OIDCCallbackResponse) *AuthOIDCCallbackResponse {
	if resp == nil {
		return nil
	}
	var role types.TenantRole
	if resp.Tenant != nil {
		role = membershipRoleForTenant(resp.Memberships, resp.Tenant.ID)
	}
	return &AuthOIDCCallbackResponse{
		Success:      resp.Success,
		Message:      resp.Message,
		User:         userForAuthResponse(resp.User),
		Tenant:       NewTenantResponseWithRole(resp.Tenant, role),
		Memberships:  resp.Memberships,
		Token:        resp.Token,
		RefreshToken: resp.RefreshToken,
		IsNewUser:    resp.IsNewUser,
	}
}

func membershipRoleForTenant(memberships []types.Membership, tenantID uint64) types.TenantRole {
	for _, m := range memberships {
		if m.TenantID == tenantID && m.Role.IsValid() {
			return m.Role
		}
	}
	return ""
}

package dto

import (
	"context"

	"gaze/internal/types"
)

// RoleFromContext returns the caller's tenant role from ctx.
func RoleFromContext(ctx context.Context) types.TenantRole {
	return types.TenantRoleFromContext(ctx)
}

// CanViewIntegrationSecrets is true for the owner and for API keys with full
// tenant access or the manage_tenant_settings capability.
func CanViewIntegrationSecrets(ctx context.Context) bool {
	if RoleFromContext(ctx).IsValid() {
		return true
	}
	return apiKeyCanManageIntegrationSecrets(ctx)
}

func apiKeyCanManageIntegrationSecrets(ctx context.Context) bool {
	scope, ok := types.TenantAPIKeyScopeFromContext(ctx)
	if !ok {
		return false
	}
	if scope.FullAccess {
		return true
	}
	return scope.HasCapability(types.APIKeyCapabilityManageTenantSettings)
}

// RoleCanViewTenantAPIKey is true for the owner (the only role).
func RoleCanViewTenantAPIKey(role types.TenantRole) bool {
	return role.IsValid()
}

// CanViewTenantAPIKey is true for the owner (the only role).
func CanViewTenantAPIKey(ctx context.Context) bool {
	return RoleCanViewTenantAPIKey(RoleFromContext(ctx))
}

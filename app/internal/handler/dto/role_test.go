package dto

import (
	"context"
	"testing"

	"gaze/internal/types"
)

// The personal edition has exactly one principal: the owner sees integration
// configuration and secrets. Tenant-role denial paths were removed with the
// enterprise role hierarchy.
func TestCanViewIntegrationSecretsOwner(t *testing.T) {
	ctx := context.WithValue(context.Background(), types.TenantRoleContextKey, types.TenantRoleOwner)
	if !CanViewIntegrationSecrets(ctx) {
		t.Fatal("owner should view integration secrets")
	}
}

// A missing role still resolves to the single owner principal.
func TestCanViewIntegrationSecretsDefaultsToOwner(t *testing.T) {
	if !CanViewIntegrationSecrets(context.Background()) {
		t.Fatal("missing role should resolve to the owner principal")
	}
}

func TestCanViewTenantAPIKeyOwner(t *testing.T) {
	ctx := context.WithValue(context.Background(), types.TenantRoleContextKey, types.TenantRoleOwner)
	if !CanViewTenantAPIKey(ctx) {
		t.Fatal("owner should view the tenant API key metadata")
	}
}

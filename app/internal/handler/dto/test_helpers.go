package dto

import (
	"context"

	"gaze/internal/types"
)

func adminContext() context.Context {
	ctx := context.Background()
	return context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleOwner)
}

func viewerContext() context.Context {
	ctx := context.Background()
	return context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleOwner)
}

func ownerContext() context.Context {
	ctx := context.Background()
	return context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleOwner)
}

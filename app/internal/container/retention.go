package container

import (
	"context"
	"time"

	"gaze/internal/logger"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

// startRetentionService runs daily housekeeping for the Agent workspace: it
// expires stale confirmation tasks and purges recycle-bin documents older than
// the 30-day retention window.
func startRetentionService(
	docs interfaces.AgentDocumentService,
	pending interfaces.AgentPendingService,
	tenantRepo interfaces.TenantRepository,
) {
	if docs == nil || pending == nil || tenantRepo == nil {
		return
	}
	go func() {
		run := func() {
			ctx := logger.CloneContext(context.Background())
			now := time.Now()
			if count, err := pending.PurgeExpired(ctx, now); err != nil {
				logger.Warnf(ctx, "[retention] expire pending tasks failed: %v", err)
			} else if count > 0 {
				logger.Infof(ctx, "[retention] expired %d agent pending task(s)", count)
			}
			tenants, err := tenantRepo.ListTenants(ctx)
			if err != nil {
				logger.Warnf(ctx, "[retention] list tenants failed: %v", err)
				return
			}
			for _, tenant := range tenants {
				if tenant == nil {
					continue
				}
				tenantCtx := context.WithValue(ctx, types.TenantIDContextKey, tenant.ID)
				tenantCtx = context.WithValue(tenantCtx, types.TenantInfoContextKey, tenant)
				count, err := docs.PurgeTrashExpired(tenantCtx, types.KnowledgeTrashKeepDays)
				if err != nil {
					logger.Warnf(tenantCtx, "[retention] purge trash failed for tenant %d: %v", tenant.ID, err)
					continue
				}
				if count > 0 {
					logger.Infof(tenantCtx, "[retention] purged %d trashed knowledge document(s) for tenant %d", count, tenant.ID)
				}
			}
		}
		time.Sleep(2 * time.Minute)
		run()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			run()
		}
	}()
}

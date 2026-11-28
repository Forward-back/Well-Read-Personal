package interfaces

import (
	"context"
	"time"

	"gaze/internal/types"
)

// AgentPendingRepository persists L2/L3 dry-run tasks and document lineage.
type AgentPendingRepository interface {
	Create(ctx context.Context, task *types.AgentPendingTask) error
	Get(ctx context.Context, tenantID uint64, id string) (*types.AgentPendingTask, error)
	Save(ctx context.Context, task *types.AgentPendingTask) error
	CreateLineage(ctx context.Context, rows []*types.DocLineage) error
	ListLineageByDestination(ctx context.Context, tenantID uint64, knowledgeID string) ([]*types.DocLineage, error)
	// ListExpired returns pending tasks past their expires_at.
	ListExpired(ctx context.Context, before time.Time, limit int) ([]*types.AgentPendingTask, error)
}

// AgentPendingService plans and executes confirmed structural changes.
type AgentPendingService interface {
	Plan(ctx context.Context, req *types.AgentPendingPlanRequest) (*types.AgentPendingTask, error)
	Get(ctx context.Context, id string) (*types.AgentPendingView, error)
	Confirm(ctx context.Context, id string, req *types.AgentConfirmRequest) (*types.AgentPendingExecuteResult, error)
	Reject(ctx context.Context, id string, req *types.AgentConfirmRequest) (*types.AgentPendingView, error)
	PurgeExpired(ctx context.Context, before time.Time) (int, error)
}

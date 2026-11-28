package repository

import (
	"context"
	"errors"
	"time"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"gorm.io/gorm"
)

var ErrAgentPendingNotFound = errors.New("agent pending task not found")

type agentPendingRepository struct {
	db *gorm.DB
}

// NewAgentPendingRepository creates the pending-task and lineage repository.
func NewAgentPendingRepository(db *gorm.DB) interfaces.AgentPendingRepository {
	return &agentPendingRepository{db: db}
}

func (r *agentPendingRepository) Create(ctx context.Context, task *types.AgentPendingTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *agentPendingRepository) Get(
	ctx context.Context, tenantID uint64, id string,
) (*types.AgentPendingTask, error) {
	var task types.AgentPendingTask
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAgentPendingNotFound
	}
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *agentPendingRepository) Save(ctx context.Context, task *types.AgentPendingTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}

func (r *agentPendingRepository) CreateLineage(ctx context.Context, rows []*types.DocLineage) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(rows, 50).Error
}

func (r *agentPendingRepository) ListLineageByDestination(
	ctx context.Context, tenantID uint64, knowledgeID string,
) ([]*types.DocLineage, error) {
	var rows []*types.DocLineage
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND dst_knowledge_id = ?", tenantID, knowledgeID).
		Order("created_at ASC").
		Find(&rows).Error
	return rows, err
}

func (r *agentPendingRepository) ListExpired(
	ctx context.Context, before time.Time, limit int,
) ([]*types.AgentPendingTask, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows []*types.AgentPendingTask
	err := r.db.WithContext(ctx).
		Where("status = ? AND expires_at < ?", types.AgentPendingStatusPending, before).
		Order("expires_at ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

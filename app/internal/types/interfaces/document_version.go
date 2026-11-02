package interfaces

import (
	"context"

	"gaze/internal/types"
)

// DocumentVersionRepository persists immutable document snapshots and applies
// rollback transactions.
type DocumentVersionRepository interface {
	CreateDocVersion(ctx context.Context, version *types.DocVersion, chunks []*types.DocVersionChunk) error
	GetLatestDocVersion(ctx context.Context, tenantID uint64, knowledgeID string) (*types.DocVersion, error)
	ListDocVersions(ctx context.Context, tenantID uint64, knowledgeID string) ([]*types.DocVersion, error)
	GetDocVersion(ctx context.Context, tenantID uint64, knowledgeID string, versionID string) (*types.DocVersion, []*types.DocVersionChunk, error)
	ApplyDocVersionRollback(ctx context.Context, payload *types.DocVersionRollbackTx) error
}

// DocumentVersionService snapshots documents and performs immutable rollback.
type DocumentVersionService interface {
	EnsureInitialVersion(ctx context.Context, tenantID uint64, kbID, knowledgeID, actorID string) (*types.DocVersion, error)
	CreateVersion(ctx context.Context, tenantID uint64, kbID, knowledgeID, sourceType, sourceID, title, description, actorID string) (*types.DocVersion, error)
	ListVersions(ctx context.Context, tenantID uint64, kbID, knowledgeID string) ([]*types.DocVersion, error)
	LatestVersion(ctx context.Context, tenantID uint64, knowledgeID string) (*types.DocVersion, error)
	GetVersion(ctx context.Context, tenantID uint64, kbID, knowledgeID, versionID string) (*types.DocVersion, []*types.DocVersionChunk, error)
	CompareVersions(ctx context.Context, tenantID uint64, kbID, knowledgeID, fromVersionID, toVersionID string) (*types.DocVersionCompareResponse, error)
	Rollback(ctx context.Context, tenantID uint64, kbID, knowledgeID, versionID, actorID, comment string) (*types.DocVersion, error)
	RecordManualEdit(ctx context.Context, tenantID uint64, kbID, knowledgeID, actorID string) error
}

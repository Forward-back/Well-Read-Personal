package repository

import (
	"context"
	"errors"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrDocVersionNotFound = errors.New("document version not found")

type documentVersionRepository struct {
	db *gorm.DB
}

// NewDocumentVersionRepository creates the document snapshot repository.
func NewDocumentVersionRepository(db *gorm.DB) interfaces.DocumentVersionRepository {
	return &documentVersionRepository{db: db}
}

func (r *documentVersionRepository) CreateDocVersion(
	ctx context.Context, version *types.DocVersion, chunks []*types.DocVersionChunk,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(version).Error; err != nil {
			return err
		}
		if len(chunks) > 0 {
			return tx.CreateInBatches(chunks, 100).Error
		}
		return nil
	})
}

func (r *documentVersionRepository) GetLatestDocVersion(
	ctx context.Context, tenantID uint64, knowledgeID string,
) (*types.DocVersion, error) {
	var version types.DocVersion
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_id = ?", tenantID, knowledgeID).
		Order("version_seq DESC").
		First(&version).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDocVersionNotFound
		}
		return nil, err
	}
	return &version, nil
}

func (r *documentVersionRepository) ListDocVersions(
	ctx context.Context, tenantID uint64, knowledgeID string,
) ([]*types.DocVersion, error) {
	var versions []*types.DocVersion
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_id = ?", tenantID, knowledgeID).
		Order("version_seq DESC").
		Find(&versions).Error
	return versions, err
}

func (r *documentVersionRepository) GetDocVersion(
	ctx context.Context, tenantID uint64, knowledgeID string, versionID string,
) (*types.DocVersion, []*types.DocVersionChunk, error) {
	var version types.DocVersion
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_id = ? AND id = ?", tenantID, knowledgeID, versionID).
		First(&version).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrDocVersionNotFound
		}
		return nil, nil, err
	}
	var chunks []*types.DocVersionChunk
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND version_id = ?", tenantID, versionID).
		Order("chunk_index ASC").
		Find(&chunks).Error; err != nil {
		return nil, nil, err
	}
	return &version, chunks, nil
}

func (r *documentVersionRepository) ApplyDocVersionRollback(
	ctx context.Context, payload *types.DocVersionRollbackTx,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := payload.Now
		for _, chunk := range payload.Chunks {
			updates := map[string]interface{}{
				"content":          chunk.Content,
				"is_enabled":       chunk.IsEnabled,
				"content_revision": chunk.ContentRevision,
				"last_editor_id":   chunk.LastEditorID,
				"updated_at":       now,
				"deleted_at":       nil,
			}
			if err := tx.Model(&types.Chunk{}).
				Where("id = ? AND tenant_id = ?", chunk.ID, chunk.TenantID).
				Updates(updates).Error; err != nil {
				return err
			}
		}
		if len(payload.NewChunks) > 0 {
			for _, chunk := range payload.NewChunks {
				updates := map[string]interface{}{
					"knowledge_base_id": chunk.KnowledgeBaseID,
					"knowledge_id":      chunk.KnowledgeID,
					"content":           chunk.Content,
					"source_content":    chunk.SourceContent,
					"chunk_index":       chunk.ChunkIndex,
					"is_enabled":        chunk.IsEnabled,
					"chunk_type":        chunk.ChunkType,
					"index_status":      chunk.IndexStatus,
					"content_revision":  chunk.ContentRevision,
					"last_editor_id":    chunk.LastEditorID,
					"updated_at":        now,
					"deleted_at":        nil,
				}
				res := tx.Unscoped().Model(&types.Chunk{}).
					Where("id = ? AND tenant_id = ?", chunk.ID, chunk.TenantID).
					Updates(updates)
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected == 0 {
					if err := tx.Create(chunk).Error; err != nil {
						return err
					}
				}
			}
		}
		if len(payload.Revisions) > 0 {
			if err := tx.Create(payload.Revisions).Error; err != nil {
				return err
			}
		}
		if len(payload.DisableChunkIDs) > 0 {
			if err := tx.Model(&types.Chunk{}).
				Where("tenant_id = ? AND knowledge_id = ? AND id IN ? AND deleted_at IS NULL",
					payload.Version.TenantID, payload.KnowledgeID, payload.DisableChunkIDs).
				Update("deleted_at", now).Error; err != nil {
				return err
			}
		}
		if payload.Version != nil {
			if err := createSnapshotTx(tx, payload.Version); err != nil {
				return err
			}
		}
		if payload.MarkEngineSyncPending {
			if err := tx.Model(&types.Knowledge{}).
				Where("tenant_id = ? AND id = ?", payload.Version.TenantID, payload.KnowledgeID).
				Updates(map[string]interface{}{
					"engine_sync_status":       types.EngineSyncStatusPending,
					"engine_sync_requested_at": now,
					"engine_sync_error":        "",
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func createSnapshotTx(tx *gorm.DB, version *types.DocVersion) error {
	var chunks []*types.Chunk
	if err := tx.Where("tenant_id = ? AND knowledge_id = ? AND deleted_at IS NULL",
		version.TenantID, version.KnowledgeID).
		Order("chunk_index ASC").
		Find(&chunks).Error; err != nil {
		return err
	}
	version.ChunkCount = len(chunks)
	if err := tx.Create(version).Error; err != nil {
		return err
	}
	snaps := make([]*types.DocVersionChunk, 0, len(chunks))
	for _, chunk := range chunks {
		snaps = append(snaps, &types.DocVersionChunk{
			ID:         uuid.NewString(),
			TenantID:   version.TenantID,
			VersionID:  version.ID,
			ChunkID:    chunk.ID,
			ChunkIndex: chunk.ChunkIndex,
			Content:    chunk.Content,
			IsEnabled:  chunk.IsEnabled,
		})
	}
	if len(snaps) == 0 {
		return nil
	}
	return tx.CreateInBatches(snaps, 100).Error
}

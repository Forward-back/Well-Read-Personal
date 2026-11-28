package repository

import (
	"context"
	"testing"
	"time"

	"gaze/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Minimal knowledges mirror for engine-sync status assertions; the full
// knowledge model carries JSON columns this test does not need.
type testKnowledgeSyncRow struct {
	ID                    string `gorm:"primaryKey"`
	TenantID              uint64
	EngineSyncStatus      string
	EngineSyncRequestedAt *time.Time
	EngineSyncError       string
	UpdatedAt             time.Time
	DeletedAt             gorm.DeletedAt
}

func (testKnowledgeSyncRow) TableName() string { return "knowledges" }

func newDocContentReplaceFixture(t *testing.T) (*documentVersionRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&testKnowledgeSyncRow{},
		&types.Chunk{},
		&types.DocVersion{},
		&types.DocVersionChunk{},
	))
	return &documentVersionRepository{db: db}, db
}

func TestApplyDocContentReplaceSwapsChunksAndSnapshotsTextOnly(t *testing.T) {
	repo, db := newDocContentReplaceFixture(t)
	ctx := context.Background()
	now := time.Now()
	tenant := uint64(1)
	knowledgeID := "doc-1"

	oldText := &types.Chunk{
		ID: "old-text", TenantID: tenant, KnowledgeBaseID: "kb-1", KnowledgeID: knowledgeID,
		Content: "old body", ChunkIndex: 0, IsEnabled: true, ChunkType: types.ChunkTypeText,
	}
	oldParent := &types.Chunk{
		ID: "old-parent", TenantID: tenant, KnowledgeBaseID: "kb-1", KnowledgeID: knowledgeID,
		Content: "parent body", ChunkIndex: 0, IsEnabled: true, ChunkType: types.ChunkTypeParentText,
	}
	require.NoError(t, db.Create(oldText).Error)
	require.NoError(t, db.Create(oldParent).Error)
	require.NoError(t, db.Create(&testKnowledgeSyncRow{
		ID: knowledgeID, TenantID: tenant, EngineSyncStatus: types.EngineSyncStatusSynced,
	}).Error)

	sessionID := "session-1"
	toolName := "edit_document_section"
	version := &types.DocVersion{
		ID: "ver-2", TenantID: tenant, KnowledgeBaseID: "kb-1", KnowledgeID: knowledgeID,
		VersionSeq: 2, VersionLabel: "V1.2", SourceType: types.DocVersionSourceAgentEdit,
		SourceID: sessionID, Title: "Agent 编辑", CreatedAt: now,
		SessionID: &sessionID, ToolName: &toolName,
	}
	newChunks := []*types.Chunk{
		{
			ID: "new-text-1", TenantID: tenant, KnowledgeBaseID: "kb-1", KnowledgeID: knowledgeID,
			Content: "new body one", ChunkIndex: 0, IsEnabled: true,
			ChunkType: types.ChunkTypeText, IndexStatus: "ready",
		},
		{
			ID: "new-text-2", TenantID: tenant, KnowledgeBaseID: "kb-1", KnowledgeID: knowledgeID,
			Content: "new body two", ChunkIndex: 1, IsEnabled: true,
			ChunkType: types.ChunkTypeText, IndexStatus: "ready",
		},
		{
			ID: "new-parent", TenantID: tenant, KnowledgeBaseID: "kb-1", KnowledgeID: knowledgeID,
			Content: "parent body", ChunkIndex: 0, IsEnabled: true,
			ChunkType: types.ChunkTypeParentText, IndexStatus: "ready",
		},
	}
	err := repo.ApplyDocContentReplace(ctx, &types.DocContentReplaceTx{
		Version: version, TenantID: tenant, KnowledgeID: knowledgeID,
		NewChunks: newChunks, MarkEngineSyncPending: true, Now: now,
	})
	require.NoError(t, err)

	var liveChunks []*types.Chunk
	require.NoError(t, db.Where("knowledge_id = ? AND deleted_at IS NULL", knowledgeID).
		Order("chunk_index ASC").Find(&liveChunks).Error)
	require.Len(t, liveChunks, 3)
	for _, chunk := range liveChunks {
		require.NotEqual(t, "old-text", chunk.ID)
		require.NotEqual(t, "old-parent", chunk.ID)
	}

	var snapshots []*types.DocVersionChunk
	require.NoError(t, db.Where("version_id = ?", version.ID).Order("chunk_index ASC").Find(&snapshots).Error)
	require.Len(t, snapshots, 2, "snapshots must contain text chunks only")
	require.EqualValues(t, 2, version.ChunkCount)

	var storedVersion types.DocVersion
	require.NoError(t, db.Where("id = ?", version.ID).First(&storedVersion).Error)
	require.NotNil(t, storedVersion.SessionID)
	require.Equal(t, sessionID, *storedVersion.SessionID)
	require.NotNil(t, storedVersion.ToolName)
	require.Equal(t, toolName, *storedVersion.ToolName)

	var knowledgeRow testKnowledgeSyncRow
	require.NoError(t, db.Where("id = ?", knowledgeID).First(&knowledgeRow).Error)
	require.Equal(t, types.EngineSyncStatusPending, knowledgeRow.EngineSyncStatus)
}

func TestApplyDocContentReplaceRollsBackOnChunkInsertError(t *testing.T) {
	repo, db := newDocContentReplaceFixture(t)
	ctx := context.Background()
	now := time.Now()
	tenant := uint64(1)
	knowledgeID := "doc-2"

	old := &types.Chunk{
		ID: "keep-me", TenantID: tenant, KnowledgeBaseID: "kb-1", KnowledgeID: knowledgeID,
		Content: "old body", ChunkIndex: 0, IsEnabled: true, ChunkType: types.ChunkTypeText,
	}
	require.NoError(t, db.Create(old).Error)
	// Duplicate primary key forces the transaction to fail after the soft delete.
	duplicate := []*types.Chunk{
		{ID: "dup", TenantID: tenant, KnowledgeBaseID: "kb-1", KnowledgeID: knowledgeID,
			Content: "a", ChunkIndex: 0, IsEnabled: true, ChunkType: types.ChunkTypeText},
		{ID: "dup", TenantID: tenant, KnowledgeBaseID: "kb-1", KnowledgeID: knowledgeID,
			Content: "b", ChunkIndex: 1, IsEnabled: true, ChunkType: types.ChunkTypeText},
	}
	version := &types.DocVersion{
		ID: "ver-bad", TenantID: tenant, KnowledgeBaseID: "kb-1", KnowledgeID: knowledgeID,
		VersionSeq: 2, VersionLabel: "V1.2", SourceType: types.DocVersionSourceAgentEdit,
	}
	err := repo.ApplyDocContentReplace(ctx, &types.DocContentReplaceTx{
		Version: version, TenantID: tenant, KnowledgeID: knowledgeID, NewChunks: duplicate, Now: now,
	})
	require.Error(t, err)

	var live []*types.Chunk
	require.NoError(t, db.Where("knowledge_id = ? AND deleted_at IS NULL", knowledgeID).Find(&live).Error)
	require.Len(t, live, 1)
	require.Equal(t, "keep-me", live[0].ID)
	var versions int64
	require.NoError(t, db.Model(&types.DocVersion{}).Where("knowledge_id = ?", knowledgeID).Count(&versions).Error)
	require.Zero(t, versions)
}

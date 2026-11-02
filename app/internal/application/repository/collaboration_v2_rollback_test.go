package repository

import (
	"context"
	"testing"
	"time"

	"gaze/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestApplyDocVersionRollbackRestoresSoftDeletedChunk(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:rollback-restore?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&types.Chunk{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewDocumentVersionRepository(db)
	ctx := context.Background()
	now := time.Now()
	softDeleted := &types.Chunk{
		ID: "chunk-1", TenantID: 1, KnowledgeBaseID: "kb-1", KnowledgeID: "doc-1",
		Content: "old", ChunkType: types.ChunkTypeText, ChunkIndex: 2,
		DeletedAt: gorm.DeletedAt{Time: now, Valid: true},
	}
	if err := db.Create(softDeleted).Error; err != nil {
		t.Fatalf("seed soft-deleted chunk: %v", err)
	}

	restored := &types.Chunk{
		ID: "chunk-1", TenantID: 1, KnowledgeBaseID: "kb-1", KnowledgeID: "doc-1",
		Content: "restored", SourceContent: "restored", ChunkType: types.ChunkTypeText,
		ChunkIndex: 2, IsEnabled: true, IndexStatus: "processing",
	}
	if err := repo.ApplyDocVersionRollback(ctx, &types.DocVersionRollbackTx{
		KnowledgeID: "doc-1",
		NewChunks:   []*types.Chunk{restored},
		Now:         now,
	}); err != nil {
		t.Fatalf("ApplyDocVersionRollback: %v", err)
	}

	var got types.Chunk
	if err := db.Unscoped().Where("id = ?", "chunk-1").First(&got).Error; err != nil {
		t.Fatalf("load restored chunk: %v", err)
	}
	if got.DeletedAt.Valid {
		t.Fatal("expected soft-deleted chunk to be restored")
	}
	if got.Content != "restored" {
		t.Fatalf("content = %q, want restored", got.Content)
	}
	if got.ChunkIndex != 2 {
		t.Fatalf("chunk index = %d, want 2", got.ChunkIndex)
	}
}

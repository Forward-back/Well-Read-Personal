package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gaze/internal/application/repository"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/google/uuid"
)

type documentVersionService struct {
	repo      interfaces.DocumentVersionRepository
	chunkRepo interfaces.ChunkRepository
}

// NewDocumentVersionService creates the immutable document snapshot service.
func NewDocumentVersionService(
	repo interfaces.DocumentVersionRepository,
	chunkRepo interfaces.ChunkRepository,
) interfaces.DocumentVersionService {
	return &documentVersionService{repo: repo, chunkRepo: chunkRepo}
}

func versionLabel(seq int) string { return fmt.Sprintf("V1.%d", seq) }

func (s *documentVersionService) nextVersion(
	ctx context.Context, tenantID uint64, kbID, knowledgeID, sourceType, sourceID, title, description, actorID string,
) (*types.DocVersion, error) {
	seq := 0
	if latest, err := s.repo.GetLatestDocVersion(ctx, tenantID, knowledgeID); err == nil && latest != nil {
		seq = latest.VersionSeq + 1
	} else if !errors.Is(err, repository.ErrDocVersionNotFound) {
		return nil, err
	}
	return &types.DocVersion{
		ID:              uuid.NewString(),
		TenantID:        tenantID,
		KnowledgeBaseID: kbID,
		KnowledgeID:     knowledgeID,
		VersionSeq:      seq,
		VersionLabel:    versionLabel(seq),
		SourceType:      sourceType,
		SourceID:        sourceID,
		Title:           title,
		Description:     description,
		CreatedBy:       actorID,
		CreatedAt:       time.Now(),
	}, nil
}

func (s *documentVersionService) EnsureInitialVersion(
	ctx context.Context, tenantID uint64, kbID, knowledgeID, actorID string,
) (*types.DocVersion, error) {
	latest, err := s.repo.GetLatestDocVersion(ctx, tenantID, knowledgeID)
	if err == nil && latest != nil {
		return latest, nil
	}
	if err != nil && !errors.Is(err, repository.ErrDocVersionNotFound) {
		return nil, err
	}
	version, err := s.CreateVersion(ctx, tenantID, kbID, knowledgeID,
		types.DocVersionSourceUpload, "", "初始导入", "文档基线快照", actorID)
	if err != nil {
		// A concurrent caller may have created V1.0 first.
		if again, getErr := s.repo.GetLatestDocVersion(ctx, tenantID, knowledgeID); getErr == nil && again != nil {
			return again, nil
		}
		return nil, err
	}
	return version, nil
}

func (s *documentVersionService) CreateVersion(
	ctx context.Context, tenantID uint64, kbID, knowledgeID, sourceType, sourceID, title, description, actorID string,
) (*types.DocVersion, error) {
	version, err := s.nextVersion(ctx, tenantID, kbID, knowledgeID, sourceType, sourceID, title, description, actorID)
	if err != nil {
		return nil, err
	}
	chunks, err := s.chunkRepo.ListChunksByKnowledgeID(ctx, tenantID, knowledgeID)
	if err != nil {
		return nil, err
	}
	snaps := make([]*types.DocVersionChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		snaps = append(snaps, &types.DocVersionChunk{
			ID:         uuid.NewString(),
			TenantID:   tenantID,
			VersionID:  version.ID,
			ChunkID:    chunk.ID,
			ChunkIndex: chunk.ChunkIndex,
			Content:    chunk.Content,
			IsEnabled:  chunk.IsEnabled,
		})
	}
	version.ChunkCount = len(snaps)
	if err := s.repo.CreateDocVersion(ctx, version, snaps); err != nil {
		return nil, err
	}
	return version, nil
}

func (s *documentVersionService) ListVersions(
	ctx context.Context, tenantID uint64, kbID, knowledgeID string,
) ([]*types.DocVersion, error) {
	if _, err := s.EnsureInitialVersion(ctx, tenantID, kbID, knowledgeID, ""); err != nil {
		return nil, err
	}
	versions, err := s.repo.ListDocVersions(ctx, tenantID, knowledgeID)
	if err != nil {
		return nil, err
	}
	for i, version := range versions {
		version.IsCurrent = i == 0
	}
	return versions, nil
}

// LatestVersion returns the current version head without creating a baseline.
// Citation binding runs on the answer path, where a snapshot write would be
// both surprising and expensive; documents that never had a version stay
// unlabelled until someone opens their version history.
func (s *documentVersionService) LatestVersion(
	ctx context.Context, tenantID uint64, knowledgeID string,
) (*types.DocVersion, error) {
	version, err := s.repo.GetLatestDocVersion(ctx, tenantID, knowledgeID)
	if err != nil {
		if errors.Is(err, repository.ErrDocVersionNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return version, nil
}

func (s *documentVersionService) GetVersion(
	ctx context.Context, tenantID uint64, kbID, knowledgeID, versionID string,
) (*types.DocVersion, []*types.DocVersionChunk, error) {
	if _, err := s.EnsureInitialVersion(ctx, tenantID, kbID, knowledgeID, ""); err != nil {
		return nil, nil, err
	}
	version, chunks, err := s.repo.GetDocVersion(ctx, tenantID, knowledgeID, versionID)
	if err != nil {
		return nil, nil, err
	}
	if latest, err := s.repo.GetLatestDocVersion(ctx, tenantID, knowledgeID); err == nil && latest != nil {
		version.IsCurrent = latest.ID == version.ID
	}
	return version, chunks, nil
}

func (s *documentVersionService) CompareVersions(
	ctx context.Context, tenantID uint64, kbID, knowledgeID, fromVersionID, toVersionID string,
) (*types.DocVersionCompareResponse, error) {
	from, fromChunks, err := s.GetVersion(ctx, tenantID, kbID, knowledgeID, fromVersionID)
	if err != nil {
		return nil, err
	}
	to, toChunks, err := s.GetVersion(ctx, tenantID, kbID, knowledgeID, toVersionID)
	if err != nil {
		return nil, err
	}
	return &types.DocVersionCompareResponse{
		From:       from,
		To:         to,
		FromChunks: fromChunks,
		ToChunks:   toChunks,
	}, nil
}

func (s *documentVersionService) Rollback(
	ctx context.Context, tenantID uint64, kbID, knowledgeID, versionID, actorID, comment string,
) (*types.DocVersion, error) {
	target, snapshot, err := s.GetVersion(ctx, tenantID, kbID, knowledgeID, versionID)
	if err != nil {
		return nil, err
	}
	if target.IsCurrent {
		return nil, fmt.Errorf("当前版本无需回滚")
	}
	current, err := s.chunkRepo.ListChunksByKnowledgeID(ctx, tenantID, knowledgeID)
	if err != nil {
		return nil, err
	}
	currentByID := make(map[string]*types.Chunk, len(current))
	for _, chunk := range current {
		currentByID[chunk.ID] = chunk
	}
	now := time.Now()
	updates := make([]*types.Chunk, 0, len(snapshot))
	newChunks := make([]*types.Chunk, 0)
	revisions := make([]*types.ChunkRevision, 0)
	snapshotIDs := make(map[string]struct{}, len(snapshot))

	for _, snap := range snapshot {
		snapshotIDs[snap.ChunkID] = struct{}{}
		live := currentByID[snap.ChunkID]
		if live == nil {
			newChunks = append(newChunks, &types.Chunk{
				ID:              snap.ChunkID,
				TenantID:        tenantID,
				KnowledgeBaseID: kbID,
				KnowledgeID:     knowledgeID,
				Content:         snap.Content,
				SourceContent:   snap.Content,
				ChunkIndex:      snap.ChunkIndex,
				IsEnabled:       snap.IsEnabled,
				ChunkType:       types.ChunkTypeText,
				IndexStatus:     "processing",
				LastEditorID:    actorID,
				UpdatedAt:       now,
				CreatedAt:       now,
			})
			continue
		}
		if live.Content == snap.Content && live.IsEnabled == snap.IsEnabled {
			continue
		}
		revisions = append(revisions, &types.ChunkRevision{
			ID:              uuid.NewString(),
			TenantID:        live.TenantID,
			KnowledgeBaseID: live.KnowledgeBaseID,
			KnowledgeID:     live.KnowledgeID,
			ChunkID:         live.ID,
			Revision:        live.ContentRevision,
			Content:         live.Content,
			IsEnabled:       live.IsEnabled,
			EditorID:        live.LastEditorID,
			EditSource:      string(types.DocVersionSourceRollback),
			EditedAt:        live.UpdatedAt,
			CreatedAt:       now,
		})
		live.Content = snap.Content
		live.IsEnabled = snap.IsEnabled
		live.ContentRevision++
		live.LastEditorID = actorID
		live.UpdatedAt = now
		live.IndexStatus = "processing"
		updates = append(updates, live)
	}

	disable := make([]string, 0)
	for _, chunk := range current {
		if _, ok := snapshotIDs[chunk.ID]; ok {
			continue
		}
		disable = append(disable, chunk.ID)
	}

	version, err := s.nextVersion(ctx, tenantID, kbID, knowledgeID,
		types.DocVersionSourceRollback, target.ID,
		fmt.Sprintf("回滚至 %s", target.VersionLabel), comment, actorID)
	if err != nil {
		return nil, err
	}
	tx := &types.DocVersionRollbackTx{
		Version:               version,
		KnowledgeID:           knowledgeID,
		Chunks:                updates,
		NewChunks:             newChunks,
		Revisions:             revisions,
		DisableChunkIDs:       disable,
		MarkEngineSyncPending: true,
		Now:                   now,
	}
	if err := s.repo.ApplyDocVersionRollback(ctx, tx); err != nil {
		return nil, err
	}
	return version, nil
}

func (s *documentVersionService) RecordManualEdit(
	ctx context.Context, tenantID uint64, kbID, knowledgeID, actorID string,
) error {
	_, err := s.CreateVersion(ctx, tenantID, kbID, knowledgeID,
		types.DocVersionSourceManualEdit, "", "手动编辑", "", actorID)
	return err
}

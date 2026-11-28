package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gaze/internal/application/service/retriever"
	"gaze/internal/logger"
	"gaze/internal/types"
	werrors "gaze/internal/errors"
)

// CommitAgentMarkdown replaces a document's full markdown, re-chunks it and
// snapshots an audited version (used by confirmed L2/L3 plans and revert).
func (s *knowledgeService) CommitAgentMarkdown(
	ctx context.Context, req *types.AgentMarkdownCommitRequest,
) (*types.DocVersion, error) {
	if req == nil || strings.TrimSpace(req.KnowledgeID) == "" {
		return nil, werrors.NewValidationError("缺少 knowledge_id")
	}
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	knowledge, kb, err := loadKnowledgeWrite(ctx, s.repo, s.kbService, req.KnowledgeID)
	if err != nil {
		return nil, err
	}
	if kb.Type == types.KnowledgeBaseTypeFAQ || knowledge.Type == types.KnowledgeTypeFAQ {
		return nil, werrors.NewValidationError("FAQ 知识库不支持文档编辑")
	}
	markdown := strings.TrimSpace(strings.ReplaceAll(req.Markdown, "\r\n", "\n")) + "\n"
	if markdown == "\n" {
		return nil, werrors.NewValidationError("文档内容不能为空")
	}
	if len([]rune(markdown)) > manualContentMaxLength {
		return nil, werrors.NewValidationError(fmt.Sprintf("文档超过长度限制（最多%d个字符）", manualContentMaxLength))
	}
	if expected := strings.TrimSpace(req.ExpectedVersionID); expected != "" {
		latest, _ := s.docVersions.LatestVersion(ctx, tenantID, knowledge.ID)
		if latest == nil || latest.ID != expected {
			label := "无"
			if latest != nil {
				label = latest.VersionLabel
			}
			return nil, werrors.NewValidationError(fmt.Sprintf("文档已被修改，当前版本 %s，请重新预览", label))
		}
	}
	parsed, parents := splitAgentMarkdown(kb, knowledge, markdown)
	now := time.Now()
	rows, _ := buildChunkRowsFromParsed(knowledge, parsed, parents, nil, now)
	sourceType := req.SourceType
	if sourceType == "" {
		sourceType = types.DocVersionSourceAgentEdit
	}
	version, err := s.docVersions.CommitAgentContent(ctx, &types.AgentContentCommit{
		TenantID:        tenantID,
		KnowledgeBaseID: knowledge.KnowledgeBaseID,
		KnowledgeID:     knowledge.ID,
		ActorID:         req.ActorID,
		SourceType:      sourceType,
		SourceID:        req.SessionID,
		Title:           req.Title,
		Description:     req.Description,
		Audit: types.DocVersionAudit{
			SessionID:     optionalString(req.SessionID),
			MessageID:     optionalString(req.MessageID),
			ToolName:      optionalString(req.ToolName),
			PendingTaskID: optionalString(req.PendingTaskID),
		},
		NewChunks:             rows,
		MarkEngineSyncPending: true,
	})
	if err != nil {
		return nil, err
	}
	s.publishKnowledgeChange(ctx, tenantID, knowledge.ID)
	return version, nil
}

// TrashDocuments moves documents to the recycle bin: soft-delete the row and
// drop engine/vector entries, but keep chunks, files and tags so restore can
// rebuild without the original upload.
func (s *knowledgeService) TrashDocuments(ctx context.Context, knowledgeIDs []string) ([]string, error) {
	ids := wsUniqueStrings(knowledgeIDs)
	if len(ids) == 0 {
		return nil, werrors.NewValidationError("请选择要删除的文档")
	}
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	knowledgeList, err := loadKnowledgeWriteBatch(ctx, s.repo, s.kbService, ids)
	if err != nil {
		return nil, err
	}
	for _, knowledge := range knowledgeList {
		s.deleteFromEngine(ctx, knowledge.ID)
	}
	s.removeKnowledgeFromVectorIndex(ctx, knowledgeList)
	if err := s.repo.DeleteKnowledgeList(ctx, tenantID, ids); err != nil {
		return nil, err
	}
	logger.Infof(ctx, "Trashed %d knowledge documents", len(ids))
	return ids, nil
}

// RestoreDocuments clears the trash flag and re-enqueues processing so chunks
// and the RAG engine entry are rebuilt from the preserved source.
func (s *knowledgeService) RestoreDocuments(ctx context.Context, knowledgeIDs []string) ([]string, error) {
	ids := wsUniqueStrings(knowledgeIDs)
	if len(ids) == 0 {
		return nil, werrors.NewValidationError("请选择要恢复的文档")
	}
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	restored := make([]string, 0, len(ids))
	for _, id := range ids {
		row, err := s.repo.GetKnowledgeUnscoped(ctx, tenantID, id)
		if err != nil || row == nil {
			return restored, werrors.NewNotFoundError("文档不存在：" + id)
		}
		if !row.DeletedAt.Valid {
			continue
		}
		if err := s.repo.RestoreKnowledge(ctx, tenantID, id); err != nil {
			return restored, err
		}
		if _, err := s.ReparseKnowledge(ctx, id, nil); err != nil {
			logger.Warnf(ctx, "restore knowledge %s reparse enqueue failed: %v", id, err)
		}
		restored = append(restored, id)
	}
	return restored, nil
}

// PurgeDocuments permanently removes trashed documents and their resources.
func (s *knowledgeService) PurgeDocuments(ctx context.Context, knowledgeIDs []string) (int, error) {
	ids := wsUniqueStrings(knowledgeIDs)
	if len(ids) == 0 {
		return 0, nil
	}
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return 0, err
	}
	purged := 0
	for _, id := range ids {
		row, err := s.repo.GetKnowledgeUnscoped(ctx, tenantID, id)
		if err != nil || row == nil {
			continue
		}
		if row.DeletedAt.Valid != true {
			return purged, werrors.NewValidationError("仅回收站中的文档可以彻底删除：" + id)
		}
		if kb, kbErr := s.kbService.GetKnowledgeBaseByID(ctx, row.KnowledgeBaseID); kbErr == nil && kb != nil {
			if row.FilePath != "" {
				if fileSvc := s.resolveFileService(ctx, kb); fileSvc != nil {
					if err := fileSvc.DeleteFile(ctx, row.FilePath); err != nil {
						logger.Warnf(ctx, "purge knowledge %s file delete failed: %v", id, err)
					}
				}
			}
		}
		if err := s.chunkRepo.DeleteChunksByKnowledgeID(ctx, tenantID, id); err != nil {
			logger.Warnf(ctx, "purge knowledge %s chunk delete failed: %v", id, err)
		}
		if err := s.repo.DeleteKnowledgeTagRelations(ctx, id); err != nil {
			logger.Warnf(ctx, "purge knowledge %s tag delete failed: %v", id, err)
		}
		if err := s.repo.HardDeleteKnowledge(ctx, tenantID, id); err != nil {
			return purged, err
		}
		purged++
	}
	return purged, nil
}

// ListTrash returns recycle-bin rows with their purge deadline.
func (s *knowledgeService) ListTrash(
	ctx context.Context, kbID, keyword string, page *types.Pagination,
) (*types.KnowledgeTrashList, error) {
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	if page == nil || page.GetPageSize() <= 0 {
		page = &types.Pagination{Page: 1, PageSize: 20}
	}
	rows, total, err := s.repo.ListTrashedKnowledge(ctx, tenantID, kbID, keyword, page)
	if err != nil {
		return nil, err
	}
	items := make([]types.KnowledgeTrashItem, 0, len(rows))
	for _, row := range rows {
		deletedAt := row.DeletedAt.Time
		items = append(items, types.KnowledgeTrashItem{
			ID:              row.ID,
			Title:           row.Title,
			FileName:        row.FileName,
			KnowledgeBaseID: row.KnowledgeBaseID,
			FileType:        row.FileType,
			DeletedAt:       deletedAt,
			PurgeAfter:      deletedAt.AddDate(0, 0, types.KnowledgeTrashKeepDays),
			Size:            row.StorageSize,
		})
	}
	return &types.KnowledgeTrashList{Total: total, Items: items}, nil
}

// PurgeTrashExpired permanently removes trash older than keepDays.
func (s *knowledgeService) PurgeTrashExpired(ctx context.Context, keepDays int) (int, error) {
	if keepDays <= 0 {
		keepDays = types.KnowledgeTrashKeepDays
	}
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return 0, err
	}
	page := &types.Pagination{Page: 1, PageSize: 200}
	rows, _, err := s.repo.ListTrashedKnowledge(ctx, tenantID, "", "", page)
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	ids := make([]string, 0)
	for _, row := range rows {
		if row.DeletedAt.Valid && row.DeletedAt.Time.Before(cutoff) {
			ids = append(ids, row.ID)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return s.PurgeDocuments(ctx, ids)
}

// removeKnowledgeFromVectorIndex is best-effort: trash must not block on
// embedding store availability because the row is hidden app-side anyway.
func (s *knowledgeService) removeKnowledgeFromVectorIndex(ctx context.Context, knowledgeList []*types.Knowledge) {
	tenantInfo, _ := types.TenantInfoFromContext(ctx)
	if tenantInfo == nil {
		return
	}
	for _, knowledge := range knowledgeList {
		if strings.TrimSpace(knowledge.EmbeddingModelID) == "" {
			continue
		}
		kb, err := s.kbService.GetKnowledgeBaseByID(ctx, knowledge.KnowledgeBaseID)
		if err != nil || kb == nil || !kb.NeedsEmbeddingModel() {
			continue
		}
		embeddingModel, err := s.modelService.GetEmbeddingModel(ctx, kb.EmbeddingModelID)
		if err != nil {
			continue
		}
		engine, err := retriever.CreateRetrieveEngineForKB(ctx, s.retrieveEngine, s.ownership, tenantInfo.ID, kb.VectorStoreID)
		if err != nil {
			continue
		}
		if err := engine.DeleteByKnowledgeIDList(ctx, []string{knowledge.ID}, embeddingModel.GetDimensions(), knowledge.Type); err != nil {
			logger.Warnf(ctx, "trash knowledge %s vector cleanup failed: %v", knowledge.ID, err)
		}
	}
}


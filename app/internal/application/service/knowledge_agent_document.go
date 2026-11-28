package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gaze/internal/infrastructure/chunker"
	"gaze/internal/types"
	secutils "gaze/internal/utils"
	werrors "gaze/internal/errors"
)

// GetDocumentContent renders a document as markdown, live or at one version.
func (s *knowledgeService) GetDocumentContent(
	ctx context.Context, knowledgeID, versionID string,
) (*types.DocumentContentView, error) {
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	knowledge, err := s.repo.GetKnowledgeByID(ctx, tenantID, knowledgeID)
	if err != nil || knowledge == nil {
		return nil, werrors.NewNotFoundError("文档不存在")
	}
	view := &types.DocumentContentView{
		KnowledgeID:      knowledge.ID,
		KnowledgeBaseID:  knowledge.KnowledgeBaseID,
		Title:            knowledge.Title,
		FileName:         knowledge.FileName,
		EngineSyncStatus: knowledge.EngineSyncStatus,
	}
	if strings.TrimSpace(versionID) != "" {
		version, chunks, err := s.docVersions.GetVersion(
			ctx, tenantID, knowledge.KnowledgeBaseID, knowledge.ID, versionID,
		)
		if err != nil {
			return nil, werrors.NewNotFoundError("文档版本不存在")
		}
		parts := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			if text := strings.TrimSpace(chunk.Content); text != "" {
				parts = append(parts, text)
			}
		}
		view.Markdown = strings.Join(parts, "\n\n")
		view.ChunkCount = len(chunks)
		view.Version = version
	} else {
		chunks, err := s.chunkRepo.ListChunksByKnowledgeID(ctx, tenantID, knowledgeID)
		if err != nil {
			return nil, err
		}
		markdown, exact := assembleChunksMarkdown(chunks)
		if !exact {
			markdown = joinTextChunks(chunks)
		}
		view.Markdown = markdown
		view.ChunkCount = len(chunks)
		if latest, err := s.docVersions.LatestVersion(ctx, tenantID, knowledgeID); err == nil {
			view.Version = latest
		}
	}
	view.CharCount = len([]rune(view.Markdown))
	return view, nil
}

// CommitAgentEdit applies one heading-scoped L1 edit: re-chunk the full
// markdown, replace the chunk set, snapshot the new version and trigger ingest.
func (s *knowledgeService) CommitAgentEdit(
	ctx context.Context, req *types.AgentDocumentEditRequest,
) (*types.AgentDocumentEditResult, error) {
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
	if knowledge.ParseStatus == types.ParseStatusPending ||
		knowledge.ParseStatus == types.ParseStatusProcessing ||
		knowledge.ParseStatus == types.ParseStatusFinalizing ||
		knowledge.ParseStatus == types.ParseStatusDeleting {
		return nil, werrors.NewValidationError("文档正在解析中，暂不能编辑")
	}

	chunks, err := s.chunkRepo.ListChunksByKnowledgeID(ctx, tenantID, knowledge.ID)
	if err != nil {
		return nil, err
	}
	markdown, exact := assembleChunksMarkdown(chunks)
	if strings.TrimSpace(markdown) == "" {
		return nil, werrors.NewValidationError("文档没有可编辑的正文")
	}
	if !exact {
		return nil, werrors.NewValidationError("文档分块偏移不一致，请先在知识库重新解析后再编辑")
	}

	previous, _ := s.docVersions.LatestVersion(ctx, tenantID, knowledge.ID)
	if expected := strings.TrimSpace(req.ExpectedVersionID); expected != "" {
		if previous == nil || previous.ID != expected {
			currentLabel := "无"
			if previous != nil {
				currentLabel = fmt.Sprintf("%s（%s）", previous.VersionLabel, previous.ID)
			}
			return nil, werrors.NewValidationError(
				fmt.Sprintf("文档已被修改，当前版本 %s，请重新读取后再编辑", currentLabel),
			)
		}
	}

	section, _, err := locateMarkdownSection(markdown, req.Heading, req.HeadingLevel, req.Occurrence)
	if err != nil {
		return nil, werrors.NewValidationError(err.Error())
	}
	updated, err := applySectionEdit(markdown, section, req.Action, req.Content)
	if err != nil {
		return nil, werrors.NewValidationError(err.Error())
	}
	if len([]rune(updated)) > manualContentMaxLength {
		return nil, werrors.NewValidationError(
			fmt.Sprintf("编辑后文档超过长度限制（最多%d个字符）", manualContentMaxLength),
		)
	}
	if strings.TrimSpace(updated) == "" {
		return nil, werrors.NewValidationError("编辑后文档不能为空")
	}

	parsed, parents := splitAgentMarkdown(kb, knowledge, updated)
	now := time.Now()
	rows, _ := buildChunkRowsFromParsed(knowledge, parsed, parents, nil, now)
	audit := types.DocVersionAudit{
		SessionID: optionalString(req.SessionID),
		MessageID: optionalString(req.MessageID),
		ToolName:  optionalString(req.ToolName),
	}
	description := strings.TrimSpace(req.Summary)
	if description == "" {
		description = fmt.Sprintf("Agent %s 章节「%s」", req.Action, section.Title)
	}
	version, err := s.docVersions.CommitAgentContent(ctx, &types.AgentContentCommit{
		TenantID:              tenantID,
		KnowledgeBaseID:       knowledge.KnowledgeBaseID,
		KnowledgeID:           knowledge.ID,
		ActorID:               req.ActorID,
		SourceType:            types.DocVersionSourceAgentEdit,
		SourceID:              req.SessionID,
		Title:                 "Agent 编辑：" + section.Title,
		Description:           description,
		Audit:                 audit,
		NewChunks:             rows,
		MarkEngineSyncPending: true,
	})
	if err != nil {
		return nil, err
	}

	s.publishKnowledgeChange(ctx, tenantID, knowledge.ID)
	added, removed := estimateChangedLines(markdown, updated)
	syncStatus := knowledge.EngineSyncStatus
	if refreshed, err := s.repo.GetKnowledgeByID(ctx, tenantID, knowledge.ID); err == nil && refreshed != nil {
		syncStatus = refreshed.EngineSyncStatus
	}
	result := &types.AgentDocumentEditResult{
		DocumentID:          knowledge.ID,
		Title:               knowledge.Title,
		SectionHeading:      section.Title,
		Action:              req.Action,
		VersionID:           version.ID,
		VersionLabel:        version.VersionLabel,
		ChangedLinesAdded:   added,
		ChangedLinesRemoved: removed,
		EngineSyncStatus:    syncStatus,
	}
	if previous != nil {
		result.PreviousVersionID = previous.ID
		result.PreviousVersionLabel = previous.VersionLabel
	}
	return result, nil
}

// CreateAgentDocument creates a markdown document synchronously, processes it
// into chunks, records the baseline version and triggers ingest.
func (s *knowledgeService) CreateAgentDocument(
	ctx context.Context, req *types.AgentDocumentCreateRequest,
) (*types.AgentDocumentCreateResult, error) {
	if req == nil || strings.TrimSpace(req.KnowledgeBaseID) == "" {
		return nil, werrors.NewValidationError("缺少 knowledge_base_id")
	}
	cleanContent := secutils.CleanMarkdown(req.Content)
	if strings.TrimSpace(cleanContent) == "" {
		return nil, werrors.NewValidationError("文档内容不能为空")
	}
	if len([]rune(cleanContent)) > manualContentMaxLength {
		return nil, werrors.NewValidationError(fmt.Sprintf("内容长度超出限制（最多%d个字符）", manualContentMaxLength))
	}
	safeTitle, ok := secutils.ValidateInput(req.Title)
	if !ok {
		return nil, werrors.NewValidationError("标题包含非法字符或超出长度限制")
	}
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, req.KnowledgeBaseID)
	if err != nil {
		return nil, err
	}
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	if kb == nil || kb.TenantID != tenantID {
		return nil, werrors.NewNotFoundError("知识库不存在")
	}
	if kb.Type == types.KnowledgeBaseTypeFAQ {
		return nil, werrors.NewValidationError("FAQ 知识库不支持新建 Markdown 文档")
	}
	if _, err := requireKBWrite(ctx, kb); err != nil {
		return nil, err
	}
	if err := s.checkStorageEngineConfigured(ctx, kb); err != nil {
		return nil, err
	}
	ctx = ensureTenantInfoContext(ctx, s, tenantID)

	now := time.Now()
	title := safeTitle
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprintf("Agent 文档-%s", now.Format("20060102-150405"))
	}
	folderPath := ""
	folderID := ""
	if s.folderRepo != nil {
		createdBy, _ := types.UserIDFromContext(ctx)
		if path := strings.TrimSpace(req.FolderID); path != "" {
			folder, err := s.folderRepo.GetByID(ctx, tenantID, kb.ID, path)
			if err != nil || folder == nil {
				return nil, werrors.NewValidationError("目标文件夹不存在")
			}
			folderPath = folder.Path
			createdBy = folder.CreatedBy
		}
		folder, err := s.folderRepo.EnsurePath(ctx, tenantID, kb.ID, folderPath, createdBy)
		if err != nil {
			return nil, err
		}
		folderID = folder.ID
		folderPath = folder.Path
	}

	knowledge := &types.Knowledge{
		TenantID:         tenantID,
		KnowledgeBaseID:  kb.ID,
		Type:             types.KnowledgeTypeManual,
		Channel:          defaultChannel(""),
		Title:            title,
		Source:           types.KnowledgeTypeManual,
		ParseStatus:      types.ParseStatusPending,
		EnableStatus:     "enabled",
		FileName:         ensureManualFileName(title),
		FileType:         types.KnowledgeTypeManual,
		FolderPath:       folderPath,
		FolderID:         folderID,
		EmbeddingModelID: kb.EmbeddingModelID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if uid, ok := types.UserIDFromContext(ctx); ok && !types.IsSyntheticUserID(uid) {
		knowledge.CreatedBy = uid
	}
	meta := types.NewManualKnowledgeMetadata(cleanContent, types.ManualKnowledgeStatusPublish, 1)
	if err := knowledge.SetManualMetadata(meta); err != nil {
		return nil, err
	}
	knowledge.EnsureManualDefaults()
	if err := s.repo.CreateKnowledge(ctx, knowledge); err != nil {
		return nil, err
	}
	s.bindContentResources(ctx, tenantID, knowledge.ID, cleanContent)

	s.triggerManualProcessingWithOptions(ctx, kb, knowledge, cleanContent, true, true)
	refreshed, err := s.repo.GetKnowledgeByID(ctx, tenantID, knowledge.ID)
	if err != nil || refreshed == nil {
		return nil, werrors.NewInternalServerError("文档创建后读取失败")
	}
	if refreshed.ParseStatus == types.ParseStatusFailed {
		return nil, werrors.NewInternalServerError(
			fmt.Sprintf("文档切分失败：%s", strings.TrimSpace(refreshed.ErrorMessage)),
		)
	}

	version, err := s.docVersions.CreateVersionWithAudit(
		ctx, tenantID, kb.ID, knowledge.ID,
		types.DocVersionSourceAgentCreate, req.SessionID,
		"初始创建", "Agent 创建文档", req.ActorID,
		types.DocVersionAudit{
			SessionID: optionalString(req.SessionID),
			MessageID: optionalString(req.MessageID),
			ToolName:  optionalString(req.ToolName),
		},
	)
	if err != nil {
		return nil, err
	}
	s.publishKnowledgeChange(ctx, tenantID, knowledge.ID)
	syncStatus := refreshed.EngineSyncStatus
	if latest, err := s.repo.GetKnowledgeByID(ctx, tenantID, knowledge.ID); err == nil && latest != nil {
		syncStatus = latest.EngineSyncStatus
	}
	return &types.AgentDocumentCreateResult{
		DocumentID:       knowledge.ID,
		Title:            knowledge.Title,
		KnowledgeBaseID:  kb.ID,
		FolderID:         folderID,
		VersionID:        version.ID,
		VersionLabel:     version.VersionLabel,
		EngineSyncStatus: syncStatus,
	}, nil
}

// splitAgentMarkdown runs the document's configured chunking strategy over
// markdown without image resolution (agent edits carry text only).
func splitAgentMarkdown(
	kb *types.KnowledgeBase, knowledge *types.Knowledge, markdown string,
) ([]types.ParsedChunk, []types.ParsedParentChunk) {
	processOverrides, _ := knowledge.ProcessOverrides()
	eff := ResolveProcessConfig(kb, processOverrides)
	clean := chunker.NormalizeLineEndings(markdown)
	chunkCfg := buildSplitterConfigFromChunking(eff.ChunkingConfig)
	if eff.ChunkingConfig.EnableParentChild {
		parentCfg, childCfg := buildParentChildConfigs(eff.ChunkingConfig, chunkCfg)
		result := chunker.SplitParentChild(clean, parentCfg, childCfg)
		parsed := make([]types.ParsedChunk, len(result.Children))
		for i, c := range result.Children {
			parsed[i] = types.ParsedChunk{
				Content:       c.Content,
				ContextHeader: c.ContextHeader,
				Seq:           c.Seq,
				Start:         c.Start,
				End:           c.End,
				ParentIndex:   c.ParentIndex,
			}
		}
		parents := make([]types.ParsedParentChunk, len(result.Parents))
		for i, p := range result.Parents {
			parents[i] = types.ParsedParentChunk{Content: p.Content, Seq: p.Seq, Start: p.Start, End: p.End}
		}
		return parsed, parents
	}
	split := chunker.Split(clean, chunkCfg)
	parsed := make([]types.ParsedChunk, len(split))
	for i, c := range split {
		parsed[i] = types.ParsedChunk{
			Content:       c.Content,
			ContextHeader: c.ContextHeader,
			Seq:           c.Seq,
			Start:         c.Start,
			End:           c.End,
		}
	}
	return parsed, nil
}

func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// publishKnowledgeChange triggers ingest without the request cancellation
// semantics of the SSE context.
func (s *knowledgeService) publishKnowledgeChange(ctx context.Context, tenantID uint64, knowledgeID string) {
	if s.engineSync == nil {
		return
	}
	s.engineSync.PublishKnowledgeChange(context.WithoutCancel(ctx), tenantID, knowledgeID)
}

func ensureTenantInfoContext(ctx context.Context, s *knowledgeService, tenantID uint64) context.Context {
	if _, ok := ctx.Value(types.TenantInfoContextKey).(*types.Tenant); ok {
		return ctx
	}
	if s.tenantRepo == nil {
		return ctx
	}
	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil || tenant == nil {
		return ctx
	}
	return context.WithValue(ctx, types.TenantInfoContextKey, tenant)
}

package service

import (
	"context"
	"fmt"
	"strings"

	"gaze/internal/models/chat"
	"gaze/internal/types"
)

// workspaceTurnBackend implements agenttools.WorkspaceToolBackend for one
// turn: it owns the session scope and enforces the per-turn L1 edit budget.
type workspaceTurnBackend struct {
	svc       *agentWorkspaceService
	scope     types.AgentWorkspaceScope
	turn      *types.AgentWorkspaceTurnContext
	chatModel chat.Chat
}

func (b *workspaceTurnBackend) ListWorkspaceDocuments(
	ctx context.Context, req types.ListDocumentsRequest,
) (*types.WorkspaceDocumentList, error) {
	kbIDs := b.scope.KnowledgeBaseIDs
	if requested := strings.TrimSpace(req.KnowledgeBaseID); requested != "" {
		if !wsContainsString(b.scope.KnowledgeBaseIDs, requested) {
			return nil, fmt.Errorf("知识库不在会话范围内")
		}
		kbIDs = []string{requested}
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 30
	}
	if limit > 50 {
		limit = 50
	}
	filter := types.KnowledgeListFilter{Keyword: strings.TrimSpace(req.Keyword)}
	if folderID := strings.TrimSpace(req.FolderID); folderID != "" {
		filter.FolderIDs = []string{folderID}
	}
	documents := make([]types.WorkspaceDocumentSummary, 0, limit)
	for _, kbID := range kbIDs {
		rows, _, err := b.svc.knowledgeRepo.ListPagedKnowledgeByKnowledgeBaseID(
			ctx, b.scope.TenantID, kbID, &types.Pagination{Page: 1, PageSize: 200}, filter,
		)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if len(b.scope.KnowledgeIDs) > 0 && !wsContainsString(b.scope.KnowledgeIDs, row.ID) {
				continue
			}
			documents = append(documents, types.WorkspaceDocumentSummary{
				ID:              row.ID,
				Title:           row.Title,
				FileName:        row.FileName,
				KnowledgeBaseID: row.KnowledgeBaseID,
				FolderID:        row.FolderID,
				ParseStatus:     row.ParseStatus,
				SyncStatus:      row.EngineSyncStatus,
			})
			if len(documents) >= limit {
				break
			}
		}
		if len(documents) >= limit {
			break
		}
	}
	return &types.WorkspaceDocumentList{Count: len(documents), Documents: documents}, nil
}

func (b *workspaceTurnBackend) GetWorkspaceDocument(
	ctx context.Context, req types.GetDocumentRequest,
) (*types.WorkspaceDocumentContentResult, error) {
	knowledgeID := strings.TrimSpace(req.KnowledgeID)
	if knowledgeID == "" {
		return nil, fmt.Errorf("缺少 knowledge_id")
	}
	view, err := b.svc.agentDocs.GetDocumentContent(ctx, knowledgeID, "")
	if err != nil {
		return nil, err
	}
	if !b.docInScope(view) {
		return nil, fmt.Errorf("文档不在会话范围内")
	}
	maxChars := req.MaxChars
	if maxChars <= 0 {
		maxChars = 30000
	}
	if maxChars > 60000 {
		maxChars = 60000
	}
	markdown := view.Markdown
	truncated := false
	if runeLen := len([]rune(markdown)); runeLen > maxChars {
		markdown = string([]rune(markdown)[:maxChars]) + "\n\n…（已截断）"
		truncated = true
	}
	result := &types.WorkspaceDocumentContentResult{
		DocumentID:       view.KnowledgeID,
		Title:            view.Title,
		EngineSyncStatus: view.EngineSyncStatus,
		ChunkCount:       view.ChunkCount,
		CharCount:        view.CharCount,
		Truncated:        truncated,
		Markdown:         markdown,
	}
	if view.Version != nil {
		result.VersionID = view.Version.ID
		result.VersionLabel = view.Version.VersionLabel
	}
	return result, nil
}

func (b *workspaceTurnBackend) SearchWorkspaceDocuments(
	ctx context.Context, req types.SearchDocumentsRequest,
) (*types.WorkspaceSearchResult, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("query 不能为空")
	}
	kbIDs := b.scope.KnowledgeBaseIDs
	if len(req.KnowledgeBaseIDs) > 0 {
		for _, id := range req.KnowledgeBaseIDs {
			if !wsContainsString(b.scope.KnowledgeBaseIDs, id) {
				return nil, fmt.Errorf("知识库不在会话范围内：%s", id)
			}
		}
		kbIDs = wsUniqueStrings(req.KnowledgeBaseIDs)
	}
	docIDs := b.scope.KnowledgeIDs
	if len(req.DocumentIDs) > 0 {
		if len(b.scope.KnowledgeIDs) > 0 {
			for _, id := range req.DocumentIDs {
				if !wsContainsString(b.scope.KnowledgeIDs, id) {
					return nil, fmt.Errorf("文档不在会话范围内：%s", id)
				}
			}
		}
		docIDs = wsUniqueStrings(req.DocumentIDs)
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	if b.svc.ragEngine == nil {
		return nil, fmt.Errorf("检索服务未配置")
	}
	var results []*types.SearchResult
	var err error
	if len(docIDs) > 0 {
		results, err = b.svc.ragEngine.SearchDocuments(ctx, []string{query}, kbIDs, docIDs, limit)
	} else {
		results, err = b.svc.ragEngine.Search(ctx, []string{query}, kbIDs, limit)
	}
	if err != nil {
		return nil, err
	}
	hits := make([]types.WorkspaceSearchHit, 0, len(results))
	for _, item := range results {
		if item == nil {
			continue
		}
		hits = append(hits, types.WorkspaceSearchHit{
			DocumentID:    item.KnowledgeID,
			DocumentTitle: item.KnowledgeTitle,
			ChunkIndex:    item.ChunkIndex,
			Score:         item.Score,
			Excerpt:       wsTruncateRunes(strings.TrimSpace(item.Content), 300),
		})
	}
	return &types.WorkspaceSearchResult{Count: len(hits), SearchSource: "knowledge", Results: hits}, nil
}

func (b *workspaceTurnBackend) ExtractWorkspaceKnowledgePoints(
	ctx context.Context, req types.ExtractKnowledgePointsRequest,
) (*types.WorkspaceKnowledgePoints, error) {
	view, err := b.svc.agentDocs.GetDocumentContent(ctx, strings.TrimSpace(req.KnowledgeID), "")
	if err != nil {
		return nil, err
	}
	if !b.docInScope(view) {
		return nil, fmt.Errorf("文档不在会话范围内")
	}
	maxPoints := req.MaxPoints
	if maxPoints <= 0 {
		maxPoints = 8
	}
	if maxPoints > 20 {
		maxPoints = 20
	}
	focus := strings.TrimSpace(req.Focus)
	prompt := fmt.Sprintf("请从下面的文档中提炼最多 %d 条知识点，每条一行，以「- 」开头，不要编号，不要额外解释。", maxPoints)
	if focus != "" {
		prompt += "\n提炼侧重点：" + focus
	}
	prompt += "\n\n文档《" + view.Title + "》正文：\n" + wsTruncateRunes(view.Markdown, 30000)
	response, err := b.chatModel.Chat(ctx, []chat.Message{
		{Role: "user", Content: prompt},
	}, &chat.ChatOptions{Temperature: 0.2})
	if err != nil {
		return nil, err
	}
	points := make([]string, 0, maxPoints)
	for _, line := range strings.Split(response.Content, "\n") {
		cleaned := strings.TrimSpace(line)
		cleaned = strings.TrimLeft(cleaned, "-*•0123456789.、 ")
		cleaned = strings.TrimSpace(cleaned)
		if cleaned == "" {
			continue
		}
		points = append(points, cleaned)
		if len(points) >= maxPoints {
			break
		}
	}
	if len(points) == 0 {
		return nil, fmt.Errorf("未能从文档提炼出知识点")
	}
	return &types.WorkspaceKnowledgePoints{DocumentID: view.KnowledgeID, Title: view.Title, Points: points}, nil
}

func (b *workspaceTurnBackend) EditWorkspaceDocumentSection(
	ctx context.Context, req types.AgentDocumentEditRequest,
) (*types.AgentDocumentEditResult, error) {
	if !b.docInScopeByID(req.KnowledgeID) {
		return nil, fmt.Errorf("文档不在会话范围内")
	}
	if err := b.reserveEdit(); err != nil {
		return nil, err
	}
	req.SessionID = b.turn.SessionID
	req.MessageID = b.turn.MessageID
	req.ToolName = types.AgentToolEditDocumentSection
	req.ActorID = b.scope.UserID
	if req.ActorID == "" {
		req.ActorID, _ = types.UserIDFromContext(ctx)
	}
	result, err := b.svc.agentDocs.CommitAgentEdit(ctx, &req)
	return result, err
}

func (b *workspaceTurnBackend) CreateWorkspaceDocument(
	ctx context.Context, req types.AgentDocumentCreateRequest,
) (*types.AgentDocumentCreateResult, error) {
	kbID := strings.TrimSpace(req.KnowledgeBaseID)
	if kbID == "" && len(b.scope.KnowledgeBaseIDs) > 0 {
		kbID = b.scope.KnowledgeBaseIDs[0]
	}
	if !wsContainsString(b.scope.KnowledgeBaseIDs, kbID) {
		return nil, fmt.Errorf("知识库不在会话范围内")
	}
	if err := b.reserveEdit(); err != nil {
		return nil, err
	}
	req.KnowledgeBaseID = kbID
	req.SessionID = b.turn.SessionID
	req.MessageID = b.turn.MessageID
	req.ToolName = types.AgentToolCreateDocument
	req.ActorID = b.scope.UserID
	if req.ActorID == "" {
		req.ActorID, _ = types.UserIDFromContext(ctx)
	}
	return b.svc.agentDocs.CreateAgentDocument(ctx, &req)
}

func (b *workspaceTurnBackend) reserveEdit() error {
	if b.turn == nil {
		return fmt.Errorf("缺少回合上下文")
	}
	if b.turn.EditCount >= agentWorkspaceMaxEditsPerTurn {
		return fmt.Errorf("本回合已达到 %d 次编辑上限，请分多轮继续", agentWorkspaceMaxEditsPerTurn)
	}
	b.turn.EditCount++
	return nil
}

func (b *workspaceTurnBackend) docInScope(view *types.DocumentContentView) bool {
	if view == nil {
		return false
	}
	if len(b.scope.KnowledgeIDs) > 0 {
		return wsContainsString(b.scope.KnowledgeIDs, view.KnowledgeID)
	}
	return wsContainsString(b.scope.KnowledgeBaseIDs, view.KnowledgeBaseID)
}

func (b *workspaceTurnBackend) docInScopeByID(knowledgeID string) bool {
	if strings.TrimSpace(knowledgeID) == "" {
		return false
	}
	if len(b.scope.KnowledgeIDs) > 0 {
		return wsContainsString(b.scope.KnowledgeIDs, knowledgeID)
	}
	// Without a document whitelist, membership is checked by the write service
	// against the KB scope; the KB check happens in CommitAgentEdit.
	return true
}

func wsTruncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}

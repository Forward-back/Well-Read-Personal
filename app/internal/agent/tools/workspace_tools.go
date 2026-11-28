package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gaze/internal/types"
)

// WorkspaceToolBackend executes Agent-workspace document operations. Scope and
// turn are supplied by the caller; tool arguments never widen them.
type WorkspaceToolBackend interface {
	ListWorkspaceDocuments(ctx context.Context, req types.ListDocumentsRequest) (*types.WorkspaceDocumentList, error)
	GetWorkspaceDocument(ctx context.Context, req types.GetDocumentRequest) (*types.WorkspaceDocumentContentResult, error)
	SearchWorkspaceDocuments(ctx context.Context, req types.SearchDocumentsRequest) (*types.WorkspaceSearchResult, error)
	ExtractWorkspaceKnowledgePoints(ctx context.Context, req types.ExtractKnowledgePointsRequest) (*types.WorkspaceKnowledgePoints, error)
	EditWorkspaceDocumentSection(ctx context.Context, req types.AgentDocumentEditRequest) (*types.AgentDocumentEditResult, error)
	CreateWorkspaceDocument(ctx context.Context, req types.AgentDocumentCreateRequest) (*types.AgentDocumentCreateResult, error)
	// CreateWorkspacePending stores a dry-run L2/L3 plan and returns its pending
	// task; nothing is written to documents until the owner confirms.
	CreateWorkspacePending(ctx context.Context, req *types.AgentPendingPlanRequest) (*types.AgentPendingTask, error)
}

// WorkspaceTools is the registry-facing bundle of workspace tools.
type WorkspaceTools struct {
	ListDocuments          *ListDocumentsTool
	GetDocument            *GetDocumentTool
	SearchDocuments        *SearchDocumentsTool
	ExtractKnowledgePoints *ExtractKnowledgePointsTool
	EditDocumentSection    *EditDocumentSectionTool
	CreateDocument         *CreateDocumentTool
	SplitDocument          *SplitDocumentTool
	MergeDocuments         *MergeDocumentsTool
	DeleteDocument         *DeleteDocumentTool
	RestoreDocument        *RestoreDocumentTool
	OverwriteDocument      *OverwriteDocumentTool
	RevertDocument         *RevertDocumentTool
	scope                  types.AgentWorkspaceScope
	turn                   *types.AgentWorkspaceTurnContext
}

// NewWorkspaceTools builds the workspace tools bound to one backend and scope.
func NewWorkspaceTools(
	backend WorkspaceToolBackend,
	scope types.AgentWorkspaceScope,
	turn *types.AgentWorkspaceTurnContext,
) *WorkspaceTools {
	return &WorkspaceTools{
		ListDocuments:          &ListDocumentsTool{BaseTool: newListDocumentsBase(), backend: backend},
		GetDocument:            &GetDocumentTool{BaseTool: newGetDocumentBase(), backend: backend},
		SearchDocuments:        &SearchDocumentsTool{BaseTool: newSearchDocumentsBase(), backend: backend},
		ExtractKnowledgePoints: &ExtractKnowledgePointsTool{BaseTool: newExtractKnowledgePointsBase(), backend: backend},
		EditDocumentSection:    &EditDocumentSectionTool{BaseTool: newEditDocumentSectionBase(), backend: backend},
		CreateDocument:         &CreateDocumentTool{BaseTool: newCreateDocumentBase(), backend: backend},
		SplitDocument:          &SplitDocumentTool{BaseTool: newSplitDocumentBase(), backend: backend, bundleScope: scope, bundleTurn: turn},
		MergeDocuments:         &MergeDocumentsTool{BaseTool: newMergeDocumentsBase(), backend: backend, bundleScope: scope, bundleTurn: turn},
		DeleteDocument:         &DeleteDocumentTool{BaseTool: newDeleteDocumentBase(), backend: backend, bundleScope: scope, bundleTurn: turn},
		RestoreDocument:        &RestoreDocumentTool{BaseTool: newRestoreDocumentBase(), backend: backend, bundleScope: scope, bundleTurn: turn},
		OverwriteDocument:      &OverwriteDocumentTool{BaseTool: newOverwriteDocumentBase(), backend: backend, bundleScope: scope, bundleTurn: turn},
		RevertDocument:         &RevertDocumentTool{BaseTool: newRevertDocumentBase(), backend: backend, bundleScope: scope, bundleTurn: turn},
		scope:                  scope,
		turn:                   turn,
	}
}

// RegisterWorkspaceTools registers every workspace tool on a registry.
func RegisterWorkspaceTools(registry *ToolRegistry, bundle *WorkspaceTools) {
	registry.RegisterTool(bundle.ListDocuments)
	registry.RegisterTool(bundle.GetDocument)
	registry.RegisterTool(bundle.SearchDocuments)
	registry.RegisterTool(bundle.ExtractKnowledgePoints)
	registry.RegisterTool(bundle.EditDocumentSection)
	registry.RegisterTool(bundle.CreateDocument)
	registry.RegisterTool(bundle.SplitDocument)
	registry.RegisterTool(bundle.MergeDocuments)
	registry.RegisterTool(bundle.DeleteDocument)
	registry.RegisterTool(bundle.RestoreDocument)
	registry.RegisterTool(bundle.OverwriteDocument)
	registry.RegisterTool(bundle.RevertDocument)
}

func failedToolResult(err error) *types.ToolResult {
	return &types.ToolResult{Success: false, Error: err.Error(), Output: "工具调用失败：" + err.Error()}
}

// ---- list_documents -------------------------------------------------------

type ListDocumentsTool struct {
	BaseTool
	backend WorkspaceToolBackend
}

func newListDocumentsBase() BaseTool {
	return NewBaseTool(types.AgentToolListDocuments,
		"列出当前会话范围内的知识库 Markdown 文档，可按文件夹或关键字过滤。",
		json.RawMessage(`{
			"type":"object",
			"properties":{
				"knowledge_base_id":{"type":"string","description":"限定知识库 ID，缺省列出会话全部知识库"},
				"folder_id":{"type":"string","description":"限定文件夹 ID"},
				"keyword":{"type":"string","description":"按标题/文件名关键字过滤"},
				"limit":{"type":"integer","minimum":1,"maximum":50,"description":"最多返回条数，默认 30"}
			}
		}`))
}

func (t *ListDocumentsTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var req types.ListDocumentsRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	result, err := t.backend.ListWorkspaceDocuments(ctx, req)
	if err != nil {
		return failedToolResult(err), nil
	}
	lines := make([]string, 0, len(result.Documents))
	for _, doc := range result.Documents {
		line := fmt.Sprintf("- %s（id=%s，状态=%s）", doc.Title, doc.ID, doc.ParseStatus)
		lines = append(lines, line)
	}
	output := fmt.Sprintf("共 %d 篇文档", result.Count)
	if len(lines) > 0 {
		output += "：\n" + joinLines(lines)
	}
	return &types.ToolResult{
		Success: true,
		Output:  output,
		Data:    map[string]interface{}{"documents": result.Documents, "count": result.Count},
	}, nil
}

// ---- get_document ---------------------------------------------------------

type GetDocumentTool struct {
	BaseTool
	backend WorkspaceToolBackend
}

func newGetDocumentBase() BaseTool {
	return NewBaseTool(types.AgentToolGetDocument,
		"读取指定文档的当前 Markdown 正文、版本与入库状态；编辑前必须先读。",
		json.RawMessage(`{
			"type":"object",
			"required":["knowledge_id"],
			"properties":{
				"knowledge_id":{"type":"string","description":"文档（knowledge）ID"},
				"max_chars":{"type":"integer","minimum":1000,"maximum":60000,"description":"返回字符上限，默认 30000"}
			}
		}`))
}

func (t *GetDocumentTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var req types.GetDocumentRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	result, err := t.backend.GetWorkspaceDocument(ctx, req)
	if err != nil {
		return failedToolResult(err), nil
	}
	header := fmt.Sprintf("文档《%s》（id=%s，版本=%s，入库=%s，字符数=%d）",
		result.Title, result.DocumentID, result.VersionLabel, result.EngineSyncStatus, result.CharCount)
	return &types.ToolResult{
		Success: true,
		Output:  header + "\n\n" + result.Markdown,
		Data: map[string]interface{}{
			"document_id":        result.DocumentID,
			"title":              result.Title,
			"version_id":         result.VersionID,
			"version_label":      result.VersionLabel,
			"engine_sync_status": result.EngineSyncStatus,
			"chunk_count":        result.ChunkCount,
			"char_count":         result.CharCount,
			"truncated":          result.Truncated,
			"versions":           result.Versions,
		},
	}, nil
}

// ---- search_documents -----------------------------------------------------

type SearchDocumentsTool struct {
	BaseTool
	backend WorkspaceToolBackend
}

func newSearchDocumentsBase() BaseTool {
	return NewBaseTool(types.AgentToolSearchDocuments,
		"在会话范围内的知识库中检索与问题相关的原文片段（返回 chunk 摘要）。",
		json.RawMessage(`{
			"type":"object",
			"required":["query"],
			"properties":{
				"query":{"type":"string","description":"检索问题或关键词"},
				"knowledge_base_ids":{"type":"array","items":{"type":"string"},"description":"限定知识库，缺省用会话范围"},
				"document_ids":{"type":"array","items":{"type":"string"},"description":"限定文档，缺省用会话范围"},
				"limit":{"type":"integer","minimum":1,"maximum":20,"description":"返回条数，默认 8"}
			}
		}`))
}

func (t *SearchDocumentsTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var req types.SearchDocumentsRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	result, err := t.backend.SearchWorkspaceDocuments(ctx, req)
	if err != nil {
		return failedToolResult(err), nil
	}
	lines := make([]string, 0, len(result.Results))
	for i, hit := range result.Results {
		lines = append(lines, fmt.Sprintf("%d. 《%s》(%s) 片段 %d：%s",
			i+1, hit.DocumentTitle, hit.DocumentID, hit.ChunkIndex, hit.Excerpt))
	}
	output := fmt.Sprintf("命中 %d 条", result.Count)
	if len(lines) > 0 {
		output += "：\n" + joinLines(lines)
	}
	return &types.ToolResult{
		Success: true,
		Output:  output,
		Data:    map[string]interface{}{"results": result.Results, "count": result.Count, "search_source": result.SearchSource},
	}, nil
}

// ---- extract_knowledge_points --------------------------------------------

type ExtractKnowledgePointsTool struct {
	BaseTool
	backend WorkspaceToolBackend
}

func newExtractKnowledgePointsBase() BaseTool {
	return NewBaseTool(types.AgentToolExtractKnowledgePoints,
		"读取指定文档并提炼知识点清单（只读，不写库）。",
		json.RawMessage(`{
			"type":"object",
			"required":["knowledge_id"],
			"properties":{
				"knowledge_id":{"type":"string","description":"文档 ID"},
				"focus":{"type":"string","description":"提炼侧重点，可选"},
				"max_points":{"type":"integer","minimum":1,"maximum":20,"description":"最多要点数，默认 8"}
			}
		}`))
}

func (t *ExtractKnowledgePointsTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var req types.ExtractKnowledgePointsRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	result, err := t.backend.ExtractWorkspaceKnowledgePoints(ctx, req)
	if err != nil {
		return failedToolResult(err), nil
	}
	lines := make([]string, 0, len(result.Points))
	for i, point := range result.Points {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, point))
	}
	return &types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("《%s》知识点：\n%s", result.Title, joinLines(lines)),
		Data:    map[string]interface{}{"document_id": result.DocumentID, "title": result.Title, "points": result.Points},
	}, nil
}

// ---- edit_document_section ------------------------------------------------

type EditDocumentSectionTool struct {
	BaseTool
	backend WorkspaceToolBackend
}

func newEditDocumentSectionBase() BaseTool {
	return NewBaseTool(types.AgentToolEditDocumentSection,
		"按标题定位章节并编辑文档（L1：自动提交新版本，可撤销）。"+
			"action 支持 replace/append/prepend/delete；标题必须与原文完全一致，重复标题用 heading_level 与 occurrence 区分。",
		json.RawMessage(`{
			"type":"object",
			"required":["knowledge_id","heading","action"],
			"properties":{
				"knowledge_id":{"type":"string","description":"文档 ID"},
				"heading":{"type":"string","description":"目标章节标题（不含 #）"},
				"heading_level":{"type":"integer","minimum":1,"maximum":6,"description":"标题层级，用于消歧"},
				"occurrence":{"type":"integer","minimum":1,"description":"同名标题第几处，默认 1"},
				"action":{"type":"string","enum":["replace","append","prepend","delete"],"description":"编辑动作"},
				"content":{"type":"string","description":"新正文（delete 时省略）"},
				"expected_version_id":{"type":"string","description":"读取时的版本 ID；不一致会拒绝编辑"},
				"tool_summary":{"type":"string","description":"一句变更说明，写入版本记录"}
			}
		}`))
}

func (t *EditDocumentSectionTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var req types.EditDocumentSectionArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	result, err := t.backend.EditWorkspaceDocumentSection(ctx, types.AgentDocumentEditRequest{
		KnowledgeID:       req.KnowledgeID,
		Heading:           req.Heading,
		HeadingLevel:      req.HeadingLevel,
		Occurrence:        req.Occurrence,
		Action:            req.Action,
		Content:           req.Content,
		ExpectedVersionID: req.ExpectedVersionID,
		Summary:           req.ToolSummary,
	})
	if err != nil {
		return failedToolResult(err), nil
	}
	output := fmt.Sprintf("已提交 %s：章节「%s」，版本 %s（+%d/-%d 行），入库状态 %s。",
		result.Action, result.SectionHeading, result.VersionLabel,
		result.ChangedLinesAdded, result.ChangedLinesRemoved, result.EngineSyncStatus)
	return &types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]interface{}{
			"document_id":            result.DocumentID,
			"title":                  result.Title,
			"section_heading":        result.SectionHeading,
			"action":                 result.Action,
			"version_id":             result.VersionID,
			"version_label":          result.VersionLabel,
			"previous_version_id":    result.PreviousVersionID,
			"previous_version_label": result.PreviousVersionLabel,
			"changed_lines_added":    result.ChangedLinesAdded,
			"changed_lines_removed":  result.ChangedLinesRemoved,
			"engine_sync_status":     result.EngineSyncStatus,
		},
	}, nil
}

// ---- create_document ------------------------------------------------------

type CreateDocumentTool struct {
	BaseTool
	backend WorkspaceToolBackend
}

func newCreateDocumentBase() BaseTool {
	return NewBaseTool(types.AgentToolCreateDocument,
		"在会话范围内的知识库中新建 Markdown 文档（L1：自动提交初始版本，可撤销）。",
		json.RawMessage(`{
			"type":"object",
			"required":["title","content"],
			"properties":{
				"knowledge_base_id":{"type":"string","description":"目标知识库；缺省用会话第一个知识库"},
				"folder_id":{"type":"string","description":"目标文件夹 ID，缺省根目录"},
				"title":{"type":"string","description":"文档标题"},
				"content":{"type":"string","description":"完整 Markdown 正文"}
			}
		}`))
}

func (t *CreateDocumentTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var req types.CreateDocumentArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	result, err := t.backend.CreateWorkspaceDocument(ctx, types.AgentDocumentCreateRequest{
		KnowledgeBaseID: req.KnowledgeBaseID,
		FolderID:        req.FolderID,
		Title:           req.Title,
		Content:         req.Content,
	})
	if err != nil {
		return failedToolResult(err), nil
	}
	output := fmt.Sprintf("已新建文档《%s》（id=%s），版本 %s，入库状态 %s。",
		result.Title, result.DocumentID, result.VersionLabel, result.EngineSyncStatus)
	return &types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]interface{}{
			"document_id":        result.DocumentID,
			"title":              result.Title,
			"knowledge_base_id":  result.KnowledgeBaseID,
			"folder_id":          result.FolderID,
			"version_id":         result.VersionID,
			"version_label":      result.VersionLabel,
			"engine_sync_status": result.EngineSyncStatus,
		},
	}, nil
}

func joinLines(lines []string) string {
	out := ""
	for i, line := range lines {
		if i > 0 {
			out += "\n"
		}
		out += line
	}
	return out
}

// ---- L2/L3 dry-run tools --------------------------------------------------

func pendingToolResult(task *types.AgentPendingTask) *types.ToolResult {
	diff := ""
	var wrapper map[string]interface{}
	if err := json.Unmarshal(task.DryRun, &wrapper); err == nil {
		if value, ok := wrapper["diff"].(string); ok {
			diff = value
		}
	}
	data := map[string]interface{}{
		"pending_task_id": task.ID,
		"risk_level":      task.RiskLevel,
		"tool_name":       task.ToolName,
		"status":          task.Status,
		"expires_at":      task.ExpiresAt.Format(time.RFC3339),
		"diff":            diff,
		"dry_run":         wrapper,
	}
	output := fmt.Sprintf("%s（%s）已生成预览：%s 请在待确认卡中确认或驳回，确认前不会写入任何文档。", task.ToolName, task.RiskLevel, diff)
	return &types.ToolResult{Success: true, Output: output, Data: data}
}

func submitPending(
	ctx context.Context,
	backend WorkspaceToolBackend,
	scope types.AgentWorkspaceScope,
	turn *types.AgentWorkspaceTurnContext,
	toolName, riskLevel string,
	fill func(*types.AgentPendingPlanRequest),
) (*types.ToolResult, error) {
	req := &types.AgentPendingPlanRequest{ToolName: toolName, RiskLevel: riskLevel, Scope: scope}
	if turn != nil {
		req.Turn = *turn
	}
	fill(req)
	task, err := backend.CreateWorkspacePending(ctx, req)
	if err != nil {
		return failedToolResult(err), nil
	}
	return pendingToolResult(task), nil
}

type SplitDocumentTool struct {
	BaseTool
	backend     WorkspaceToolBackend
	bundleScope types.AgentWorkspaceScope
	bundleTurn  *types.AgentWorkspaceTurnContext
}

func newSplitDocumentBase() BaseTool {
	return NewBaseTool(types.AgentToolSplitDocument,
		"按标题把文档拆成两篇新文档，原文档移入回收站（30 天内可恢复）。L2：先返回预览与 pending_task_id，用户确认后才执行。",
		json.RawMessage(`{
			"type":"object",
			"required":["knowledge_id","heading","new_title"],
			"properties":{
				"knowledge_id":{"type":"string"},
				"heading":{"type":"string","description":"从哪个标题开始拆出（含该章节及其子章节）"},
				"heading_level":{"type":"integer","minimum":1,"maximum":6},
				"new_title":{"type":"string","description":"拆出的新文档标题"},
				"expected_version_id":{"type":"string","description":"读取时的版本 ID，用于冲突检测"},
				"tool_summary":{"type":"string"}
			}
		}`))
}

func (t *SplitDocumentTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var payload types.SplitDocumentArgs
	if err := json.Unmarshal(args, &payload); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	return submitPending(ctx, t.backend, t.bundleScope, t.bundleTurn,
		types.AgentToolSplitDocument, types.AgentRiskL2, func(req *types.AgentPendingPlanRequest) {
			req.Split = &payload
		})
}

type MergeDocumentsTool struct {
	BaseTool
	backend     WorkspaceToolBackend
	bundleScope types.AgentWorkspaceScope
	bundleTurn  *types.AgentWorkspaceTurnContext
}

func newMergeDocumentsBase() BaseTool {
	return NewBaseTool(types.AgentToolMergeDocuments,
		"把同一知识库内的多篇文档合并为一篇新文档，源文档移入回收站（30 天内可恢复）。L2：先返回预览与 pending_task_id，用户确认后才执行。",
		json.RawMessage(`{
			"type":"object",
			"required":["knowledge_ids"],
			"properties":{
				"knowledge_ids":{"type":"array","items":{"type":"string"},"minItems":2},
				"title":{"type":"string","description":"合并后的文档标题，缺省用第一篇标题（合并）"},
				"expected_version_id":{"type":"string"},
				"tool_summary":{"type":"string"}
			}
		}`))
}

func (t *MergeDocumentsTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var payload types.MergeDocumentsArgs
	if err := json.Unmarshal(args, &payload); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	return submitPending(ctx, t.backend, t.bundleScope, t.bundleTurn,
		types.AgentToolMergeDocuments, types.AgentRiskL2, func(req *types.AgentPendingPlanRequest) {
			req.Merge = &payload
		})
}

type DeleteDocumentTool struct {
	BaseTool
	backend     WorkspaceToolBackend
	bundleScope types.AgentWorkspaceScope
	bundleTurn  *types.AgentWorkspaceTurnContext
}

func newDeleteDocumentBase() BaseTool {
	return NewBaseTool(types.AgentToolDeleteDocument,
		"把文档移入回收站（30 天内可在回收站恢复或彻底删除）。L3：先返回预览与 pending_task_id，用户二次确认后才执行。",
		json.RawMessage(`{
			"type":"object",
			"required":["knowledge_ids"],
			"properties":{
				"knowledge_ids":{"type":"array","items":{"type":"string"},"minItems":1},
				"tool_summary":{"type":"string"}
			}
		}`))
}

func (t *DeleteDocumentTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var payload types.DeleteDocumentsArgs
	if err := json.Unmarshal(args, &payload); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	return submitPending(ctx, t.backend, t.bundleScope, t.bundleTurn,
		types.AgentToolDeleteDocument, types.AgentRiskL3, func(req *types.AgentPendingPlanRequest) {
			req.Delete = &payload
		})
}

type RestoreDocumentTool struct {
	BaseTool
	backend     WorkspaceToolBackend
	bundleScope types.AgentWorkspaceScope
	bundleTurn  *types.AgentWorkspaceTurnContext
}

func newRestoreDocumentBase() BaseTool {
	return NewBaseTool(types.AgentToolRestoreDocument,
		"从回收站恢复文档。L3：先返回预览与 pending_task_id，用户二次确认后才执行。",
		json.RawMessage(`{
			"type":"object",
			"required":["knowledge_ids"],
			"properties":{
				"knowledge_ids":{"type":"array","items":{"type":"string"},"minItems":1},
				"tool_summary":{"type":"string"}
			}
		}`))
}

func (t *RestoreDocumentTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var payload types.RestoreDocumentsArgs
	if err := json.Unmarshal(args, &payload); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	return submitPending(ctx, t.backend, t.bundleScope, t.bundleTurn,
		types.AgentToolRestoreDocument, types.AgentRiskL3, func(req *types.AgentPendingPlanRequest) {
			req.Restore = &payload
		})
}

type OverwriteDocumentTool struct {
	BaseTool
	backend     WorkspaceToolBackend
	bundleScope types.AgentWorkspaceScope
	bundleTurn  *types.AgentWorkspaceTurnContext
}

func newOverwriteDocumentBase() BaseTool {
	return NewBaseTool(types.AgentToolOverwriteDocument,
		"用给定 Markdown 全文覆盖文档。L3：先返回行级变更预览与 pending_task_id，用户二次确认后才执行。",
		json.RawMessage(`{
			"type":"object",
			"required":["knowledge_id","content"],
			"properties":{
				"knowledge_id":{"type":"string"},
				"content":{"type":"string","description":"完整新正文"},
				"expected_version_id":{"type":"string"},
				"tool_summary":{"type":"string"}
			}
		}`))
}

func (t *OverwriteDocumentTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var payload types.OverwriteDocumentArgs
	if err := json.Unmarshal(args, &payload); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	return submitPending(ctx, t.backend, t.bundleScope, t.bundleTurn,
		types.AgentToolOverwriteDocument, types.AgentRiskL3, func(req *types.AgentPendingPlanRequest) {
			req.Overwrite = &payload
		})
}

type RevertDocumentTool struct {
	BaseTool
	backend     WorkspaceToolBackend
	bundleScope types.AgentWorkspaceScope
	bundleTurn  *types.AgentWorkspaceTurnContext
}

func newRevertDocumentBase() BaseTool {
	return NewBaseTool(types.AgentToolRevertDocument,
		"把文档前向回滚到指定历史版本（生成新的回滚版本，不移动指针）。L2：先返回预览与 pending_task_id，用户确认后才执行。",
		json.RawMessage(`{
			"type":"object",
			"required":["knowledge_id","target_version_id"],
			"properties":{
				"knowledge_id":{"type":"string"},
				"target_version_id":{"type":"string"},
				"tool_summary":{"type":"string"}
			}
		}`))
}

func (t *RevertDocumentTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var payload types.RevertDocumentArgs
	if err := json.Unmarshal(args, &payload); err != nil {
		return failedToolResult(fmt.Errorf("参数解析失败：%w", err)), nil
	}
	return submitPending(ctx, t.backend, t.bundleScope, t.bundleTurn,
		types.AgentToolRevertDocument, types.AgentRiskL2, func(req *types.AgentPendingPlanRequest) {
			req.Revert = &payload
		})
}

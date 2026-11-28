package tools

import (
	"context"
	"encoding/json"
	"fmt"

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
}

// WorkspaceTools is the registry-facing bundle of workspace tools.
type WorkspaceTools struct {
	ListDocuments          *ListDocumentsTool
	GetDocument            *GetDocumentTool
	SearchDocuments        *SearchDocumentsTool
	ExtractKnowledgePoints *ExtractKnowledgePointsTool
	EditDocumentSection    *EditDocumentSectionTool
	CreateDocument         *CreateDocumentTool
}

// NewWorkspaceTools builds the six workspace tools bound to one backend.
func NewWorkspaceTools(backend WorkspaceToolBackend) *WorkspaceTools {
	return &WorkspaceTools{
		ListDocuments:          &ListDocumentsTool{BaseTool: newListDocumentsBase(), backend: backend},
		GetDocument:            &GetDocumentTool{BaseTool: newGetDocumentBase(), backend: backend},
		SearchDocuments:        &SearchDocumentsTool{BaseTool: newSearchDocumentsBase(), backend: backend},
		ExtractKnowledgePoints: &ExtractKnowledgePointsTool{BaseTool: newExtractKnowledgePointsBase(), backend: backend},
		EditDocumentSection:    &EditDocumentSectionTool{BaseTool: newEditDocumentSectionBase(), backend: backend},
		CreateDocument:         &CreateDocumentTool{BaseTool: newCreateDocumentBase(), backend: backend},
	}
}

// RegisterWorkspaceTools registers all six tools on a registry.
func RegisterWorkspaceTools(registry *ToolRegistry, bundle *WorkspaceTools) {
	registry.RegisterTool(bundle.ListDocuments)
	registry.RegisterTool(bundle.GetDocument)
	registry.RegisterTool(bundle.SearchDocuments)
	registry.RegisterTool(bundle.ExtractKnowledgePoints)
	registry.RegisterTool(bundle.EditDocumentSection)
	registry.RegisterTool(bundle.CreateDocument)
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

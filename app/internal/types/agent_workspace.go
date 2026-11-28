package types

import "time"

// AgentWorkspaceSurface marks sessions owned by the Agent workspace. Chat and
// reading session lists exclude it.
const AgentWorkspaceSurface = "agent"

// Agent workspace tool names (risk level in parentheses).
const (
	AgentToolListDocuments          = "list_documents"           // L0
	AgentToolGetDocument            = "get_document"             // L0
	AgentToolSearchDocuments        = "search_documents"         // L0
	AgentToolExtractKnowledgePoints = "extract_knowledge_points" // L0
	AgentToolEditDocumentSection    = "edit_document_section"    // L1
	AgentToolCreateDocument         = "create_document"          // L1
	AgentToolSplitDocument          = "split_document"           // L2 (dry-run)
	AgentToolMergeDocuments         = "merge_documents"          // L2 (dry-run)
	AgentToolRevertDocument         = "revert_document"          // L2 (dry-run)
	AgentToolDeleteDocument         = "delete_document"          // L3 (dry-run)
	AgentToolRestoreDocument        = "restore_document"         // L3 (dry-run)
	AgentToolOverwriteDocument      = "overwrite_document"       // L3 (dry-run)
)

// AgentWorkspaceToolNames is the whitelist registered for workspace runs.
var AgentWorkspaceToolNames = []string{
	AgentToolListDocuments,
	AgentToolGetDocument,
	AgentToolSearchDocuments,
	AgentToolExtractKnowledgePoints,
	AgentToolEditDocumentSection,
	AgentToolCreateDocument,
	AgentToolSplitDocument,
	AgentToolMergeDocuments,
	AgentToolRevertDocument,
	AgentToolDeleteDocument,
	AgentToolRestoreDocument,
	AgentToolOverwriteDocument,
}

// AgentWorkspaceScope is the authoritative KB/document scope of a session.
// Tool arguments never widen it.
type AgentWorkspaceScope struct {
	TenantID         uint64
	UserID           string
	KnowledgeBaseIDs []string
	KnowledgeIDs     []string
}

// AgentWorkspaceTurnContext carries the audit trail of the current turn.
type AgentWorkspaceTurnContext struct {
	SessionID string
	MessageID string
	ActorID   string
	// Model call used by extract_knowledge_points.
	ChatModelID string
	// L1 edits committed so far this turn (each turn allows at most 3).
	EditCount int
}

// AgentWorkspaceSessionView is the session shape exposed to the workspace UI.
type AgentWorkspaceSessionView struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	Surface          string    `json:"surface"`
	ModelID          string    `json:"model_id"`
	KnowledgeBaseIDs []string  `json:"knowledge_base_ids"`
	KnowledgeIDs     []string  `json:"knowledge_ids,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreateAgentWorkspaceSessionRequest opens a workspace session.
type CreateAgentWorkspaceSessionRequest struct {
	Title            string   `json:"title"`
	ModelID          string   `json:"model_id"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	KnowledgeIDs     []string `json:"knowledge_ids,omitempty"`
}

// UpdateAgentWorkspaceSessionRequest patches editable session fields.
type UpdateAgentWorkspaceSessionRequest struct {
	Title            *string  `json:"title,omitempty"`
	ModelID          *string  `json:"model_id,omitempty"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
	KnowledgeIDs     []string `json:"knowledge_ids,omitempty"`
}

// AgentWorkspaceRunRequest is the SSE chat body.
type AgentWorkspaceRunRequest struct {
	Query string `json:"query"`
}

// DocumentContentView is a document rendered as markdown, live or at a version.
type DocumentContentView struct {
	KnowledgeID      string      `json:"knowledge_id"`
	KnowledgeBaseID  string      `json:"knowledge_base_id"`
	Title            string      `json:"title"`
	FileName         string      `json:"file_name"`
	Markdown         string      `json:"markdown"`
	ChunkCount       int         `json:"chunk_count"`
	CharCount        int         `json:"char_count"`
	Truncated        bool        `json:"truncated"`
	EngineSyncStatus string      `json:"engine_sync_status"`
	Version          *DocVersion `json:"version,omitempty"`
}

// DocVersionAudit is the agent audit trail attached to a commit.
type DocVersionAudit struct {
	SessionID     *string
	MessageID     *string
	ToolName      *string
	PendingTaskID *string
}

// AgentDocumentEditRequest applies one heading-scoped section edit.
type AgentDocumentEditRequest struct {
	KnowledgeID       string
	SessionID         string
	MessageID         string
	ToolName          string
	ActorID           string
	Heading           string
	HeadingLevel      int
	Occurrence        int
	Action            string // replace | append | prepend | delete
	Content           string
	ExpectedVersionID string
	Summary           string
}

// AgentDocumentEditResult reports a committed section edit.
type AgentDocumentEditResult struct {
	DocumentID           string `json:"document_id"`
	Title                string `json:"title"`
	SectionHeading       string `json:"section_heading"`
	Action               string `json:"action"`
	VersionID            string `json:"version_id"`
	VersionLabel         string `json:"version_label"`
	PreviousVersionID    string `json:"previous_version_id"`
	PreviousVersionLabel string `json:"previous_version_label"`
	ChangedLinesAdded    int    `json:"changed_lines_added"`
	ChangedLinesRemoved  int    `json:"changed_lines_removed"`
	EngineSyncStatus     string `json:"engine_sync_status"`
}

// AgentDocumentCreateRequest creates a new markdown document.
type AgentDocumentCreateRequest struct {
	KnowledgeBaseID string
	FolderID        string
	Title           string
	Content         string
	SessionID       string
	MessageID       string
	ToolName        string
	ActorID         string
}

// AgentDocumentCreateResult reports a created document and its baseline version.
type AgentDocumentCreateResult struct {
	DocumentID       string `json:"document_id"`
	Title            string `json:"title"`
	KnowledgeBaseID  string `json:"knowledge_base_id"`
	FolderID         string `json:"folder_id"`
	VersionID        string `json:"version_id"`
	VersionLabel     string `json:"version_label"`
	EngineSyncStatus string `json:"engine_sync_status"`
}

// ListDocumentsRequest filters list_documents.
type ListDocumentsRequest struct {
	KnowledgeBaseID string `json:"knowledge_base_id"`
	FolderID        string `json:"folder_id"`
	Keyword         string `json:"keyword"`
	Limit           int    `json:"limit"`
}

// GetDocumentRequest selects get_document output size.
type GetDocumentRequest struct {
	KnowledgeID string `json:"knowledge_id"`
	MaxChars    int    `json:"max_chars"`
}

// SearchDocumentsRequest filters search_documents.
type SearchDocumentsRequest struct {
	Query            string   `json:"query"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	DocumentIDs      []string `json:"document_ids"`
	Limit            int      `json:"limit"`
}

// ExtractKnowledgePointsRequest filters extract_knowledge_points.
type ExtractKnowledgePointsRequest struct {
	KnowledgeID string `json:"knowledge_id"`
	Focus       string `json:"focus"`
	MaxPoints   int    `json:"max_points"`
}

// EditDocumentSectionArgs is the raw edit_document_section tool payload.
type EditDocumentSectionArgs struct {
	KnowledgeID       string `json:"knowledge_id"`
	Heading           string `json:"heading"`
	HeadingLevel      int    `json:"heading_level"`
	Occurrence        int    `json:"occurrence"`
	Action            string `json:"action"`
	Content           string `json:"content"`
	ExpectedVersionID string `json:"expected_version_id"`
	ToolSummary       string `json:"tool_summary"`
}

// CreateDocumentArgs is the raw create_document tool payload.
type CreateDocumentArgs struct {
	KnowledgeBaseID string `json:"knowledge_base_id"`
	FolderID        string `json:"folder_id"`
	Title           string `json:"title"`
	Content         string `json:"content"`
}

// WorkspaceDocumentSummary is one row of list_documents.
type WorkspaceDocumentSummary struct {
	ID              string `json:"document_id"`
	Title           string `json:"title"`
	FileName        string `json:"file_name"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
	FolderID        string `json:"folder_id,omitempty"`
	ParseStatus     string `json:"parse_status"`
	SyncStatus      string `json:"engine_sync_status"`
}

// WorkspaceDocumentList is the list_documents result.
type WorkspaceDocumentList struct {
	Count     int                        `json:"count"`
	Documents []WorkspaceDocumentSummary `json:"documents"`
}

// DocVersionSummary is a compact version entry exposed to the agent.
type DocVersionSummary struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	SourceType string `json:"source_type"`
	CreatedAt  string `json:"created_at"`
}

// WorkspaceDocumentContentResult is the get_document result.
type WorkspaceDocumentContentResult struct {
	DocumentID       string `json:"document_id"`
	Title            string `json:"title"`
	VersionID        string `json:"version_id,omitempty"`
	VersionLabel     string `json:"version_label,omitempty"`
	EngineSyncStatus string `json:"engine_sync_status"`
	ChunkCount       int    `json:"chunk_count"`
	CharCount        int    `json:"char_count"`
	Truncated        bool   `json:"truncated"`
	Markdown         string `json:"markdown"`
	Versions         []DocVersionSummary `json:"versions,omitempty"`
}

// WorkspaceSearchHit is one search_documents result row.
type WorkspaceSearchHit struct {
	DocumentID   string  `json:"document_id"`
	DocumentTitle string `json:"document_title"`
	ChunkIndex   int     `json:"chunk_index"`
	Score        float64 `json:"score"`
	Excerpt      string  `json:"excerpt"`
}

// WorkspaceSearchResult is the search_documents result.
type WorkspaceSearchResult struct {
	Count        int                  `json:"count"`
	SearchSource string               `json:"search_source"`
	Results      []WorkspaceSearchHit `json:"results"`
}

// WorkspaceKnowledgePoints is the extract_knowledge_points result.
type WorkspaceKnowledgePoints struct {
	DocumentID string   `json:"document_id"`
	Title      string   `json:"title"`
	Points     []string `json:"points"`
}

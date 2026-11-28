package types

import (
	"encoding/json"
	"time"
)

// Agent risk levels for document operations.
const (
	AgentRiskL2 = "L2"
	AgentRiskL3 = "L3"
)

// Pending task statuses.
const (
	AgentPendingStatusPending  = "pending"
	AgentPendingStatusApplied  = "applied"
	AgentPendingStatusRejected = "rejected"
	AgentPendingStatusExpired  = "expired"
	AgentPendingStatusFailed   = "failed"
)

// Lineage relations.
const (
	DocLineageSplitFrom  = "split_from"
	DocLineageMergedFrom = "merged_from"
)

const (
	AgentPendingTTL       = 24 * time.Hour
	AgentConfirmTokenTTL  = 10 * time.Minute
	KnowledgeTrashKeepDays = 30
)

// AgentPendingTask is one dry-run preview awaiting owner confirmation.
type AgentPendingTask struct {
	ID                 string          `json:"id"                   gorm:"type:varchar(36);primaryKey"`
	TenantID           uint64          `json:"tenant_id"            gorm:"index"`
	UserID             string          `json:"user_id,omitempty"    gorm:"type:varchar(512)"`
	SessionID          string          `json:"session_id,omitempty" gorm:"type:varchar(36);index"`
	MessageID          string          `json:"message_id,omitempty" gorm:"type:varchar(36)"`
	ToolCallID         string          `json:"tool_call_id,omitempty" gorm:"type:varchar(36)"`
	ToolName           string          `json:"tool_name"            gorm:"type:varchar(64);not null"`
	RiskLevel          string          `json:"risk_level"           gorm:"type:varchar(8);not null"`
	Status             string          `json:"status"               gorm:"type:varchar(16);not null;default:'pending'"`
	TargetKnowledgeIDs JSON            `json:"target_knowledge_ids" gorm:"type:jsonb"`
	DryRun             JSON            `json:"dry_run"              gorm:"type:jsonb"`
	BaseVersionID      string          `json:"base_version_id,omitempty" gorm:"type:varchar(36)"`
	TokenHash          string          `json:"-"                    gorm:"type:varchar(64)"`
	TokenExpiresAt     *time.Time      `json:"token_expires_at,omitempty"`
	TokenConsumedAt    *time.Time      `json:"token_consumed_at,omitempty"`
	ConfirmComment     string          `json:"confirm_comment,omitempty"`
	AppliedVersionIDs  JSON            `json:"applied_version_ids"  gorm:"type:jsonb"`
	Error              string          `json:"error,omitempty"`
	ExpiresAt          time.Time       `json:"expires_at"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// Pending task views expose dry-run diffs without secrets.
type AgentPendingView struct {
	ID            string                     `json:"id"`
	SessionID     string                     `json:"session_id,omitempty"`
	MessageID     string                     `json:"message_id,omitempty"`
	ToolName      string                     `json:"tool_name"`
	RiskLevel     string                     `json:"risk_level"`
	Status        string                     `json:"status"`
	ExpiresAt     time.Time                  `json:"expires_at"`
	DryRun        map[string]interface{}     `json:"dry_run"`
	Error         string                     `json:"error,omitempty"`
	AppliedVersions []string                 `json:"applied_version_ids,omitempty"`
}

// AgentConfirmRequest confirms or rejects a pending task.
type AgentConfirmRequest struct {
	Comment string `json:"comment"`
}

// AgentPendingExecuteResult reports the applied versions of a confirmed task.
type AgentPendingExecuteResult struct {
	PendingID      string   `json:"pending_id"`
	Status         string   `json:"status"`
	AppliedVersions []string `json:"applied_version_ids"`
	Message        string   `json:"message"`
}

// DocLineage records split/merge provenance between documents.
type DocLineage struct {
	ID             string    `json:"id"              gorm:"type:varchar(36);primaryKey"`
	TenantID       uint64    `json:"tenant_id"       gorm:"index"`
	SrcKnowledgeID string    `json:"src_knowledge_id" gorm:"type:varchar(36)"`
	SrcVersionID   string    `json:"src_version_id,omitempty" gorm:"type:varchar(36)"`
	DstKnowledgeID string    `json:"dst_knowledge_id" gorm:"type:varchar(36);index"`
	DstVersionID   string    `json:"dst_version_id,omitempty" gorm:"type:varchar(36)"`
	Relation       string    `json:"relation"        gorm:"type:varchar(24)"`
	PendingTaskID  string    `json:"pending_task_id,omitempty" gorm:"type:varchar(36)"`
	CreatedAt      time.Time `json:"created_at"`
}

// TableName pins lineage rows to the singular table created by migration 12.
func (DocLineage) TableName() string { return "doc_lineage" }

// Trash views.
type KnowledgeTrashItem struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	FileName      string    `json:"file_name"`
	KnowledgeBaseID string  `json:"knowledge_base_id"`
	FileType      string    `json:"file_type"`
	DeletedAt     time.Time `json:"deleted_at"`
	PurgeAfter    time.Time `json:"purge_after"`
	Size          int64     `json:"size"`
}

type KnowledgeTrashList struct {
	Total int64               `json:"total"`
	Items []KnowledgeTrashItem `json:"items"`
}

// ---- tool argument payloads ----------------------------------------------

type SplitDocumentArgs struct {
	KnowledgeID       string `json:"knowledge_id"`
	Heading           string `json:"heading"`
	HeadingLevel      int    `json:"heading_level"`
	NewTitle          string `json:"new_title"`
	ExpectedVersionID string `json:"expected_version_id"`
	ToolSummary       string `json:"tool_summary"`
}

type MergeDocumentsArgs struct {
	KnowledgeIDs      []string `json:"knowledge_ids"`
	Title             string   `json:"title"`
	ExpectedVersionID string   `json:"expected_version_id"`
	ToolSummary       string   `json:"tool_summary"`
}

type DeleteDocumentsArgs struct {
	KnowledgeIDs []string `json:"knowledge_ids"`
	ToolSummary  string   `json:"tool_summary"`
}

type RestoreDocumentsArgs struct {
	KnowledgeIDs []string `json:"knowledge_ids"`
	ToolSummary  string   `json:"tool_summary"`
}

type OverwriteDocumentArgs struct {
	KnowledgeID       string `json:"knowledge_id"`
	Content           string `json:"content"`
	ExpectedVersionID string `json:"expected_version_id"`
	ToolSummary       string `json:"tool_summary"`
}

type RevertDocumentArgs struct {
	KnowledgeID       string `json:"knowledge_id"`
	TargetVersionID   string `json:"target_version_id"`
	ToolSummary       string `json:"tool_summary"`
}

// ---- plan payloads stored in dry_run -------------------------------------

type SplitPlan struct {
	SourceID          string `json:"source_id"`
	SourceTitle       string `json:"source_title"`
	Heading           string `json:"heading"`
	NewTitle          string `json:"new_title"`
	RemainingTitle    string `json:"remaining_title"`
	NewMarkdown       string `json:"new_markdown"`
	RemainingMarkdown string `json:"remaining_markdown"`
	BaseVersionID     string `json:"base_version_id"`
	BaseVersionLabel  string `json:"base_version_label"`
}

type MergePlan struct {
	TargetTitle    string   `json:"target_title"`
	SourceIDs      []string `json:"source_ids"`
	SourceTitles   []string `json:"source_titles"`
	MergedMarkdown string   `json:"merged_markdown"`
	BaseVersions   []string `json:"base_version_ids"`
}

type DeletePlan struct {
	Documents []TrashPlanDocument `json:"documents"`
}

type RestorePlan struct {
	Documents []TrashPlanDocument `json:"documents"`
}

type TrashPlanDocument struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type OverwritePlan struct {
	KnowledgeID      string `json:"knowledge_id"`
	Title            string `json:"title"`
	NewMarkdown      string `json:"new_markdown"`
	BaseVersionID    string `json:"base_version_id"`
	BaseVersionLabel string `json:"base_version_label"`
	AddedLines       int    `json:"added_lines"`
	RemovedLines     int    `json:"removed_lines"`
}

type RevertPlan struct {
	KnowledgeID      string `json:"knowledge_id"`
	Title            string `json:"title"`
	TargetVersionID  string `json:"target_version_id"`
	TargetVersionLabel string `json:"target_version_label"`
	TargetMarkdown   string `json:"target_markdown"`
}

// AgentMarkdownCommitRequest replaces a document's full markdown and records
// an audited version (used by confirmed L2/L3 plans).
type AgentMarkdownCommitRequest struct {
	KnowledgeID       string
	ActorID           string
	SessionID         string
	MessageID         string
	ToolName          string
	PendingTaskID     string
	SourceType        string
	Title             string
	Description       string
	Markdown          string
	ExpectedVersionID string
}

// AgentPendingPlanRequest carries one tool's dry-run input.
type AgentPendingPlanRequest struct {
	ToolName  string
	RiskLevel string
	Scope     AgentWorkspaceScope
	Turn      AgentWorkspaceTurnContext
	Split     *SplitDocumentArgs
	Merge     *MergeDocumentsArgs
	Delete    *DeleteDocumentsArgs
	Restore   *RestoreDocumentsArgs
	Overwrite *OverwriteDocumentArgs
	Revert    *RevertDocumentArgs
}

// EncodePlan stores a plan into the DryRun JSON column.
func EncodePlan(toolName string, plan interface{}, diff string) JSON {
	raw, _ := json.Marshal(map[string]interface{}{
		"tool": toolName,
		"plan": plan,
		"diff": diff,
	})
	return JSON(raw)
}

package interfaces

import (
	"context"

	"gaze/internal/event"
	"gaze/internal/types"
)

// AgentDocumentService renders document content and commits agent L1 edits
// (full-document re-chunk + version snapshot + engine sync trigger), plus the
// L2/L3 building blocks (markdown commits, trash, restore, purge, trash list).
type AgentDocumentService interface {
	GetDocumentContent(ctx context.Context, knowledgeID, versionID string) (*types.DocumentContentView, error)
	CommitAgentEdit(ctx context.Context, req *types.AgentDocumentEditRequest) (*types.AgentDocumentEditResult, error)
	CreateAgentDocument(ctx context.Context, req *types.AgentDocumentCreateRequest) (*types.AgentDocumentCreateResult, error)
	CommitAgentMarkdown(ctx context.Context, req *types.AgentMarkdownCommitRequest) (*types.DocVersion, error)
	TrashDocuments(ctx context.Context, knowledgeIDs []string) ([]string, error)
	RestoreDocuments(ctx context.Context, knowledgeIDs []string) ([]string, error)
	PurgeDocuments(ctx context.Context, knowledgeIDs []string) (int, error)
	ListTrash(ctx context.Context, kbID, keyword string, page *types.Pagination) (*types.KnowledgeTrashList, error)
	PurgeTrashExpired(ctx context.Context, keepDays int) (int, error)
}

// AgentWorkspaceService owns Agent-workspace sessions (surface="agent") and
// drives one workspace turn on the shared agent runtime.
type AgentWorkspaceService interface {
	CreateSession(ctx context.Context, req *types.CreateAgentWorkspaceSessionRequest) (*types.AgentWorkspaceSessionView, error)
	ListSessions(ctx context.Context, page *types.Pagination, keyword string) ([]*types.AgentWorkspaceSessionView, int64, error)
	GetSession(ctx context.Context, id string) (*types.AgentWorkspaceSessionView, error)
	UpdateSession(ctx context.Context, id string, req *types.UpdateAgentWorkspaceSessionRequest) (*types.AgentWorkspaceSessionView, error)
	DeleteSession(ctx context.Context, id string) error
	ListMessages(ctx context.Context, id string, page, pageSize int) ([]*types.Message, error)
	RunTurn(ctx context.Context, session *types.Session, assistantMessageID, query string, bus *event.EventBus) error
}

// KnowledgeChangePublisher triggers engine ingest for one document.
type KnowledgeChangePublisher interface {
	PublishKnowledgeChange(ctx context.Context, tenantID uint64, knowledgeID string)
}

// Package engine defines the retrieval/generation engine behind knowledge QA.
//
// YLSW is the only engine: normal QA, agent retrieval and document ingest all
// go through it. The interface grows as YLSW endpoints land.
package engine

import (
	"context"

	"gaze/internal/types"
)

// DocumentMetadata carries the domain metadata extracted on the app side and
// passed through to the engine at ingest time.
type DocumentMetadata struct {
	DocNumber      string         `json:"doc_number"`
	OrgName        string         `json:"org_name"`
	DocDate        string         `json:"doc_date"`
	DocTitle       string         `json:"doc_title"`
	DocType        string         `json:"doc_type"`
	ProductName    string         `json:"product_name"`
	CustomMetadata map[string]any `json:"custom_metadata"`
}

// IngestChunk mirrors one app-side chunk so the engine can map its own
// internal chunks back to the app chunk that owns the content.
type IngestChunk struct {
	ID         string `json:"id"`
	ChunkIndex int    `json:"chunk_index"`
	StartAt    int    `json:"start_at"`
	EndAt      int    `json:"end_at"`
	Content    string `json:"content"`
}

// RetrievalAuditEvent is a best-effort record emitted after one engine call.
type RetrievalAuditEvent struct {
	TenantID         uint64
	UserID           string
	PrincipalKind    string
	SessionID        string
	Query            string
	KnowledgeBaseIDs []string
	ScopeMode        string
	ScopeVersion     int64
	ScopeDocCount    int
	HitDocumentIDs   []string
	Decision         string
	Error            string
}

// RetrievalAuditSink receives engine audit events. Implementations must be
// non-blocking: retrieval already completed and audit failure must not surface.
type RetrievalAuditSink interface {
	LogRetrieval(ctx context.Context, event RetrievalAuditEvent)
}

// RAGEngine is the retrieval/generation engine behind knowledge QA.
type RAGEngine interface {
	// StreamChat streams an answer for the question scoped to the
	// knowledgeBaseIDs whitelist (empty = full corpus), invoking onDelta for
	// each answer fragment, onReferences when the recall references arrive and
	// onStage for retrieval-pipeline stage notifications (recall start /
	// per-route counts / rerank / generation).
	// It returns the accumulated answer and the recall references.
	StreamChat(
		ctx context.Context,
		question string,
		knowledgeBaseIDs []string,
		aclScope *types.ACLScope,
		skill *types.SkillInjection,
		onDelta func(string),
		onReferences func([]*types.SearchResult),
		onStage func(stage string, counts map[string]int, tMs int64, detail map[string]interface{}),
	) (string, []*types.SearchResult, error)

	// Search runs pure retrieval for the agent knowledge_search tool: each
	// query is retrieved independently and the results are merged, deduplicated
	// by chunk and sorted by rerank score, truncated to topK.
	Search(
		ctx context.Context,
		queries []string,
		knowledgeBaseIDs []string,
		aclScope *types.ACLScope,
		topK int,
	) ([]*types.SearchResult, error)

	// SearchDocuments is Search restricted to an explicit app-side document
	// whitelist. The reading engine uses it to auto-answer questions inside the
	// selected documents with semantic retrieval rather than keyword matching.
	SearchDocuments(
		ctx context.Context,
		queries []string,
		knowledgeBaseIDs []string,
		documentIDs []string,
		aclScope *types.ACLScope,
		topK int,
	) ([]*types.SearchResult, error)

	// IngestDocument pushes parsed markdown to the engine for domain chunking,
	// embedding and indexing. documentID is the app-side document UUID and acts
	// as the idempotency key: re-pushing the same ID replaces the previous
	// version. fileBytes, when non-nil, pushes the raw file instead (xlsx
	// Text2SQL path); otherwise markdown is pushed. The app-side ingest
	// pipeline stays untouched (dual-write) until FAQ/memory migration retires
	// the native vector store.
	IngestDocument(
		ctx context.Context,
		knowledgeBaseID, documentID, fileName, fileType, markdown string,
		fileBytes []byte,
		chunks []IngestChunk,
		meta *DocumentMetadata,
	) error

	// DeleteDocument removes a previously ingested document from the engine.
	DeleteDocument(ctx context.Context, documentID string) error

	// UpdateDocumentMetadata refreshes document-level metadata in place.
	// Unlike IngestDocument it neither re-chunks nor re-embeds, so metadata
	// changes take effect immediately and cost nothing. Engines that cannot
	// update in place may return an error; callers fall back to a full re-push.
	UpdateDocumentMetadata(
		ctx context.Context, knowledgeBaseID, documentID string, meta *DocumentMetadata,
	) error

	// ListDocumentIDs returns every document ID currently stored in the
	// engine (app-side document UUIDs). Backs the daily dual-write
	// reconciliation (§2.2 ③).
	ListDocumentIDs(ctx context.Context) ([]string, error)

	// IndexStatus reports retrieval index health (vector count, chunk counts,
	// pending/failed embeddings, index file freshness) for the ops dashboard.
	IndexStatus(ctx context.Context) (*IndexStatus, error)
}

// IndexStatus mirrors the engine's /index/status payload.
type IndexStatus struct {
	Status        string `json:"status"`
	Vectors       int64  `json:"vectors"`
	Documents     int    `json:"documents"`
	ChildChunks   int    `json:"child_chunks"`
	PendingChunks int    `json:"pending_chunks"`
	FailedChunks  int    `json:"failed_chunks"`
	IndexPath     string `json:"index_path"`
	FileSize      int64  `json:"file_size"`
	FileUpdatedAt string `json:"file_updated_at"`
	PreviousIndex bool   `json:"previous_index"`
	Error         string `json:"error,omitempty"`
}

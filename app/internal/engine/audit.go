package engine

import (
	"context"
	"strings"

	"gaze/internal/types"
)

// NewYLSWEngineWithAudit wraps the YLSW engine with a retrieval audit sink.
func NewYLSWEngineWithAudit(sink RetrievalAuditSink) RAGEngine {
	return &auditedEngine{inner: NewYLSWEngine(), sink: sink}
}

type auditedEngine struct {
	inner RAGEngine
	sink  RetrievalAuditSink
}

func (e *auditedEngine) StreamChat(
	ctx context.Context,
	question string,
	knowledgeBaseIDs []string,
	aclScope *types.ACLScope,
	skill *types.SkillInjection,
	onDelta func(string),
	onReferences func([]*types.SearchResult),
	onStage func(stage string, counts map[string]int, tMs int64, detail map[string]interface{}),
) (string, []*types.SearchResult, error) {
	answer, refs, err := e.inner.StreamChat(
		ctx, question, knowledgeBaseIDs, aclScope, skill,
		onDelta, onReferences, onStage,
	)
	e.emit(ctx, question, knowledgeBaseIDs, aclScope, refs, err)
	return answer, refs, err
}

func (e *auditedEngine) Search(
	ctx context.Context,
	queries []string,
	knowledgeBaseIDs []string,
	aclScope *types.ACLScope,
	topK int,
) ([]*types.SearchResult, error) {
	results, err := e.inner.Search(ctx, queries, knowledgeBaseIDs, aclScope, topK)
	e.emit(ctx, strings.Join(queries, " | "), knowledgeBaseIDs, aclScope, results, err)
	return results, err
}

func (e *auditedEngine) SearchDocuments(
	ctx context.Context,
	queries []string,
	knowledgeBaseIDs []string,
	documentIDs []string,
	aclScope *types.ACLScope,
	topK int,
) ([]*types.SearchResult, error) {
	results, err := e.inner.SearchDocuments(ctx, queries, knowledgeBaseIDs, documentIDs, aclScope, topK)
	e.emit(ctx, strings.Join(queries, " | "), knowledgeBaseIDs, aclScope, results, err)
	return results, err
}

func (e *auditedEngine) IngestDocument(
	ctx context.Context,
	knowledgeBaseID, documentID, fileName, fileType, markdown string,
	fileBytes []byte,
	chunks []IngestChunk,
	meta *DocumentMetadata,
) error {
	return e.inner.IngestDocument(
		ctx, knowledgeBaseID, documentID, fileName, fileType, markdown, fileBytes, chunks, meta,
	)
}

func (e *auditedEngine) DeleteDocument(ctx context.Context, documentID string) error {
	return e.inner.DeleteDocument(ctx, documentID)
}

func (e *auditedEngine) UpdateDocumentMetadata(
	ctx context.Context, knowledgeBaseID, documentID string, meta *DocumentMetadata,
) error {
	return e.inner.UpdateDocumentMetadata(ctx, knowledgeBaseID, documentID, meta)
}

func (e *auditedEngine) ListDocumentIDs(ctx context.Context) ([]string, error) {
	return e.inner.ListDocumentIDs(ctx)
}

func (e *auditedEngine) IndexStatus(ctx context.Context) (*IndexStatus, error) {
	return e.inner.IndexStatus(ctx)
}

func (e *auditedEngine) emit(
	ctx context.Context,
	query string,
	knowledgeBaseIDs []string,
	scope *types.ACLScope,
	results []*types.SearchResult,
	callErr error,
) {
	if e.sink == nil {
		return
	}
	caller := types.CallerFromContext(ctx).Normalize()
	principal := "member"
	if strings.TrimSpace(caller.UserID) == "" {
		principal = "portal"
	}
	hitDocIDs := make([]string, 0, len(results))
	for _, result := range results {
		if result != nil && result.KnowledgeID != "" {
			hitDocIDs = append(hitDocIDs, result.KnowledgeID)
		}
	}
	event := RetrievalAuditEvent{
		TenantID:         caller.TenantID,
		UserID:           caller.UserID,
		PrincipalKind:    principal,
		Query:            query,
		KnowledgeBaseIDs: knowledgeBaseIDs,
		HitDocumentIDs:   hitDocIDs,
	}
	if sessionID, ok := types.SessionIDFromContext(ctx); ok {
		event.SessionID = sessionID
	}
	if scope != nil {
		event.ScopeMode = string(scope.Mode)
		event.ScopeVersion = scope.Version
		event.ScopeDocCount = len(scope.DocIDs)
	}
	switch {
	case callErr != nil:
		event.Decision = "error"
		event.Error = callErr.Error()
	case len(results) == 0:
		event.Decision = "empty"
	default:
		event.Decision = "ok"
	}
	e.sink.LogRetrieval(ctx, event)
}

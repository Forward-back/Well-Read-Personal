package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"gaze/internal/types"
	"github.com/stretchr/testify/require"
)

type stubWorkspaceBackend struct {
	editCalled bool
	editReq    types.AgentDocumentEditRequest
	failSearch bool
}

func (s *stubWorkspaceBackend) ListWorkspaceDocuments(
	context.Context, types.ListDocumentsRequest,
) (*types.WorkspaceDocumentList, error) {
	return &types.WorkspaceDocumentList{Count: 1, Documents: []types.WorkspaceDocumentSummary{{
		ID: "doc-1", Title: "文档", ParseStatus: "completed",
	}}}, nil
}

func (s *stubWorkspaceBackend) GetWorkspaceDocument(
	context.Context, types.GetDocumentRequest,
) (*types.WorkspaceDocumentContentResult, error) {
	return &types.WorkspaceDocumentContentResult{DocumentID: "doc-1", Title: "文档", Markdown: "# 内容"}, nil
}

func (s *stubWorkspaceBackend) SearchWorkspaceDocuments(
	context.Context, types.SearchDocumentsRequest,
) (*types.WorkspaceSearchResult, error) {
	if s.failSearch {
		return nil, errors.New("检索不可用")
	}
	return &types.WorkspaceSearchResult{Count: 1, SearchSource: "knowledge", Results: []types.WorkspaceSearchHit{{
		DocumentID: "doc-1", DocumentTitle: "文档", Excerpt: "片段",
	}}}, nil
}

func (s *stubWorkspaceBackend) ExtractWorkspaceKnowledgePoints(
	context.Context, types.ExtractKnowledgePointsRequest,
) (*types.WorkspaceKnowledgePoints, error) {
	return &types.WorkspaceKnowledgePoints{DocumentID: "doc-1", Title: "文档", Points: []string{"要点一"}}, nil
}

func (s *stubWorkspaceBackend) EditWorkspaceDocumentSection(
	_ context.Context, req types.AgentDocumentEditRequest,
) (*types.AgentDocumentEditResult, error) {
	s.editCalled = true
	s.editReq = req
	return &types.AgentDocumentEditResult{
		DocumentID: "doc-1", Title: "文档", SectionHeading: req.Heading,
		Action: req.Action, VersionID: "ver-2", VersionLabel: "V1.2",
		PreviousVersionID: "ver-1", PreviousVersionLabel: "V1.1",
		EngineSyncStatus: "pending",
	}, nil
}

func (s *stubWorkspaceBackend) CreateWorkspaceDocument(
	context.Context, types.AgentDocumentCreateRequest,
) (*types.AgentDocumentCreateResult, error) {
	return &types.AgentDocumentCreateResult{DocumentID: "doc-9", Title: "新文档", VersionLabel: "V1.0"}, nil
}

func (s *stubWorkspaceBackend) CreateWorkspacePending(
	_ context.Context, req *types.AgentPendingPlanRequest,
) (*types.AgentPendingTask, error) {
	return &types.AgentPendingTask{
		ID: "pending-1", ToolName: req.ToolName, RiskLevel: req.RiskLevel,
		Status: types.AgentPendingStatusPending,
		DryRun: types.EncodePlan(req.ToolName, map[string]string{"title": "计划"}, "预览摘要"),
	}, nil
}

func TestWorkspaceToolSchemasAreValidJSON(t *testing.T) {
	bundle := NewWorkspaceTools(&stubWorkspaceBackend{}, types.AgentWorkspaceScope{}, &types.AgentWorkspaceTurnContext{})
	for _, tool := range []types.Tool{
		bundle.ListDocuments, bundle.GetDocument, bundle.SearchDocuments,
		bundle.ExtractKnowledgePoints, bundle.EditDocumentSection, bundle.CreateDocument,
		bundle.SplitDocument, bundle.MergeDocuments, bundle.DeleteDocument,
		bundle.RestoreDocument, bundle.OverwriteDocument, bundle.RevertDocument,
	} {
		require.NotEmpty(t, tool.Name())
		require.True(t, json.Valid(tool.Parameters()), "%s schema must be valid JSON", tool.Name())
	}
}

func TestEditDocumentSectionToolPassesArgsAndData(t *testing.T) {
	backend := &stubWorkspaceBackend{}
	tool := NewWorkspaceTools(backend, types.AgentWorkspaceScope{}, &types.AgentWorkspaceTurnContext{}).EditDocumentSection
	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"knowledge_id":"doc-1","heading":"背景","action":"replace",
		"content":"新正文","expected_version_id":"ver-1","tool_summary":"改写背景"
	}`))
	require.NoError(t, err)
	require.True(t, result.Success)
	require.True(t, backend.editCalled)
	require.Equal(t, "背景", backend.editReq.Heading)
	require.Equal(t, "ver-1", backend.editReq.ExpectedVersionID)
	require.Equal(t, "ver-2", result.Data["version_id"])
	require.Equal(t, "pending", result.Data["engine_sync_status"])
}

func TestWorkspaceToolsReturnFailureResultInsteadOfError(t *testing.T) {
	backend := &stubWorkspaceBackend{failSearch: true}
	tool := NewWorkspaceTools(backend, types.AgentWorkspaceScope{}, &types.AgentWorkspaceTurnContext{}).SearchDocuments
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"缓存"}`))
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Contains(t, result.Error, "检索不可用")

	invalid := NewWorkspaceTools(backend, types.AgentWorkspaceScope{}, &types.AgentWorkspaceTurnContext{}).GetDocument
	result, err = invalid.Execute(context.Background(), json.RawMessage(`not-json`))
	require.NoError(t, err)
	require.False(t, result.Success)
}

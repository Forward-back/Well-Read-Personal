package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"gaze/internal/engine"
	"gaze/internal/middleware"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

// hybridSearchTestService only needs to resolve the knowledge base; retrieval
// itself now goes through the YLSW engine.
type hybridSearchTestService struct {
	interfaces.KnowledgeBaseService
}

func (s *hybridSearchTestService) GetKnowledgeBaseByID(_ context.Context, id string) (*types.KnowledgeBase, error) {
	return &types.KnowledgeBase{ID: id, TenantID: 1}, nil
}

// stubHybridSearchEngine returns canned retrieval results and records the
// query the handler passed to the engine.
type stubHybridSearchEngine struct {
	engine.RAGEngine
	results     []*types.SearchResult
	err         error
	searchCalls int
	lastQueries []string
	lastKBIDs   []string
	lastTopK    int
}

func (s *stubHybridSearchEngine) Search(
	_ context.Context,
	queries []string,
	knowledgeBaseIDs []string,
	topK int,
) ([]*types.SearchResult, error) {
	s.searchCalls++
	s.lastQueries = queries
	s.lastKBIDs = knowledgeBaseIDs
	s.lastTopK = topK
	if s.err != nil {
		return nil, s.err
	}
	if s.results != nil {
		return s.results, nil
	}
	return []*types.SearchResult{}, nil
}

func newHybridSearchTestRouter(
	svc interfaces.KnowledgeBaseService,
	rag *stubHybridSearchEngine,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.ErrorHandler())
	router.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Set(types.UserIDContextKey.String(), "u-test")
		c.Next()
	})
	handler := &KnowledgeBaseHandler{service: svc, ragEngine: rag}
	router.POST("/knowledge-bases/:id/hybrid-search", handler.HybridSearch)
	return router
}

func TestHybridSearchRejectsMissingQueryText(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing field", body: `{}`},
		{name: "empty field", body: `{"query_text":""}`},
		{name: "whitespace field", body: `{"query_text":"   "}`},
		{name: "wrong field name", body: `{"query":"MiniMax"}`},
		{name: "embedding with keyword matching", body: `{"query_embedding":[0.1]}`},
		{
			name: "embedding with all matching disabled",
			body: `{"query_embedding":[0.1],"disable_keywords_match":true,"disable_vector_match":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &hybridSearchTestService{}
			rag := &stubHybridSearchEngine{}
			response := performHybridSearchRequest(svc, rag, tt.body)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", response.Code, response.Body.String())
			}
			if rag.searchCalls != 0 {
				t.Fatalf("invalid request reached the engine %d time(s)", rag.searchCalls)
			}
			if !strings.Contains(response.Body.String(), `"code":1000`) {
				t.Fatalf("expected bad-request envelope, got %s", response.Body.String())
			}
		})
	}
}

func TestHybridSearchAcceptsQueryText(t *testing.T) {
	svc := &hybridSearchTestService{}
	rag := &stubHybridSearchEngine{}
	response := performHybridSearchRequest(svc, rag, `{"query_text":"MiniMax","match_count":3}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
	if rag.searchCalls != 1 {
		t.Fatalf("expected one engine search call, got %d", rag.searchCalls)
	}
	if len(rag.lastQueries) != 1 || rag.lastQueries[0] != "MiniMax" {
		t.Fatalf("query = %v, want [MiniMax]", rag.lastQueries)
	}
	if rag.lastTopK != 3 {
		t.Fatalf("topK = %d, want 3", rag.lastTopK)
	}
	if len(rag.lastKBIDs) != 1 || rag.lastKBIDs[0] != "kb-1" {
		t.Fatalf("knowledge base IDs = %v, want [kb-1]", rag.lastKBIDs)
	}
}

func TestHybridSearchAcceptsPrecomputedVectorWithoutQueryText(t *testing.T) {
	svc := &hybridSearchTestService{}
	rag := &stubHybridSearchEngine{}
	response := performHybridSearchRequest(
		svc,
		rag,
		`{"query_embedding":[0.1,0.2],"disable_keywords_match":true}`,
	)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
	if rag.searchCalls != 1 {
		t.Fatalf("expected one engine search call, got %d", rag.searchCalls)
	}
	if len(rag.lastQueries) != 1 || rag.lastQueries[0] != "" {
		t.Fatalf("query = %v, want one empty query", rag.lastQueries)
	}
}

func performHybridSearchRequest(
	svc interfaces.KnowledgeBaseService,
	rag *stubHybridSearchEngine,
	body string,
) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/knowledge-bases/kb-1/hybrid-search",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	newHybridSearchTestRouter(svc, rag).ServeHTTP(response, request)
	return response
}

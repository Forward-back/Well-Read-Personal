package session

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gaze/internal/engine"
	"gaze/internal/middleware"
	"gaze/internal/types"
)

// stubSearchRAGEngine returns canned retrieval results, matching the engine
// path SearchKnowledge now uses.
type stubSearchRAGEngine struct {
	engine.RAGEngine
	results     []*types.SearchResult
	searchCalls int
	lastQueries []string
	lastKBIDs   []string
}

func (s *stubSearchRAGEngine) Search(
	_ context.Context,
	queries []string,
	knowledgeBaseIDs []string,
	_ int,
) ([]*types.SearchResult, error) {
	s.searchCalls++
	s.lastQueries = queries
	s.lastKBIDs = knowledgeBaseIDs
	return s.results, nil
}

func newSearchKnowledgeRouter(rag *stubSearchRAGEngine) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	h := &Handler{
		ragEngine:   rag,
		fileService: &stubResourceFileService{},
	}
	r.POST("/knowledge-search", h.SearchKnowledge)
	return r
}

func TestSearchKnowledge_PublicResourceURLs(t *testing.T) {
	rag := &stubSearchRAGEngine{
		results: []*types.SearchResult{{
			Content:   "chunk ![c](" + testResourceHandle + ")",
			ImageInfo: `[{"url":"` + testResourceHandle + `"}]`,
		}},
	}
	r := newSearchKnowledgeRouter(rag)

	body := bytes.NewBufferString(`{"query":"diagram","knowledge_base_ids":["kb-1"]}`)
	req := httptest.NewRequest(http.MethodPost, "/knowledge-search?resource_urls=public", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.NotContains(t, w.Body.String(), testResourceHandle)
	assert.Contains(t, w.Body.String(), "cdn.example.com")
	assert.Equal(t, 1, rag.searchCalls)
}

func TestSearchKnowledge_InvalidResourceURLMode(t *testing.T) {
	rag := &stubSearchRAGEngine{}
	r := newSearchKnowledgeRouter(rag)

	body := bytes.NewBufferString(`{"query":"diagram","knowledge_base_ids":["kb-1"]}`)
	req := httptest.NewRequest(http.MethodPost, "/knowledge-search?resource_urls=signed", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "resource_urls")
	assert.Equal(t, 0, rag.searchCalls, "invalid mode must not reach the engine")
}

func TestSearchKnowledge_DefaultKeepsHandles(t *testing.T) {
	rag := &stubSearchRAGEngine{
		results: []*types.SearchResult{{
			Content: "chunk ![c](" + testResourceHandle + ")",
		}},
	}
	r := newSearchKnowledgeRouter(rag)

	body := bytes.NewBufferString(`{"query":"diagram","knowledge_base_ids":["kb-1"]}`)
	req := httptest.NewRequest(http.MethodPost, "/knowledge-search", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp struct {
		Data []struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	assert.Contains(t, resp.Data[0].Content, testResourceHandle)
}

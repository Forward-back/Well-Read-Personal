package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gaze/internal/middleware"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

func newHybridSearchResourceURLRouter(
	svc interfaces.KnowledgeBaseService,
	fileSvc interfaces.FileService,
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
	h := &KnowledgeBaseHandler{service: svc, fileService: fileSvc, ragEngine: rag}
	router.POST("/knowledge-bases/:id/hybrid-search", h.HybridSearch)
	router.GET("/knowledge-bases/:id/hybrid-search", h.HybridSearch)
	return router
}

func performHybridSearchResourceURLRequest(
	svc interfaces.KnowledgeBaseService,
	fileSvc interfaces.FileService,
	rag *stubHybridSearchEngine,
	method, query, body string,
) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		method,
		"/knowledge-bases/kb-1/hybrid-search"+query,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	newHybridSearchResourceURLRouter(svc, fileSvc, rag).ServeHTTP(response, request)
	return response
}

func TestHybridSearch_PublicResourceURLs(t *testing.T) {
	svc := &hybridSearchTestService{}
	rag := &stubHybridSearchEngine{
		results: []*types.SearchResult{{
			Content:   "chunk ![c](" + testResourceHandle + ")",
			ImageInfo: `[{"url":"` + testResourceHandle + `"}]`,
		}},
	}
	fileSvc := &stubResourceFileService{url: "https://cdn.example.com/signed.png"}

	for _, method := range []string{http.MethodPost, http.MethodGet} {
		t.Run(method, func(t *testing.T) {
			response := performHybridSearchResourceURLRequest(
				svc, fileSvc, rag, method, "?resource_urls=public",
				`{"query_text":"diagram"}`,
			)

			require.Equal(t, http.StatusOK, response.Code, "body=%s", response.Body.String())
			assert.NotContains(t, response.Body.String(), testResourceHandle)
			assert.Contains(t, response.Body.String(), "cdn.example.com")
		})
	}
}

func TestHybridSearch_InvalidResourceURLMode(t *testing.T) {
	svc := &hybridSearchTestService{}
	rag := &stubHybridSearchEngine{}
	response := performHybridSearchResourceURLRequest(
		svc, &stubResourceFileService{url: "https://cdn.example.com/signed.png"}, rag,
		http.MethodPost, "?resource_urls=signed",
		`{"query_text":"diagram"}`,
	)

	require.Equal(t, http.StatusBadRequest, response.Code, "body=%s", response.Body.String())
	assert.Contains(t, response.Body.String(), "resource_urls")
	assert.Equal(t, 0, rag.searchCalls, "invalid mode must not reach the engine")
}

func TestHybridSearch_DefaultKeepsHandles(t *testing.T) {
	svc := &hybridSearchTestService{}
	rag := &stubHybridSearchEngine{
		results: []*types.SearchResult{{
			Content: "chunk ![c](" + testResourceHandle + ")",
		}},
	}
	response := performHybridSearchResourceURLRequest(
		svc, &stubResourceFileService{url: "https://cdn.example.com/signed.png"}, rag,
		http.MethodPost, "",
		`{"query_text":"diagram"}`,
	)

	require.Equal(t, http.StatusOK, response.Code, "body=%s", response.Body.String())

	var resp struct {
		Data []struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	assert.Contains(t, resp.Data[0].Content, testResourceHandle)
}

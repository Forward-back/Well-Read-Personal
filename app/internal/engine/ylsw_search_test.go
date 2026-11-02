package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestYLSWSearchDocumentsSendsDocumentIDs(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"chunk_id":"99","app_chunk_id":"c1","document_id":"y1","source_doc_id":"doc1","file_name":"f.md","content":"hello","rerank_score":0.9}]}`))
	}))
	defer server.Close()
	t.Setenv("YLSW_BASE_URL", server.URL)

	results, err := NewYLSWEngine().SearchDocuments(
		context.Background(), []string{"退款期限"}, []string{"kb1"},
		[]string{"doc1"}, 5,
	)
	if err != nil {
		t.Fatalf("SearchDocuments: %v", err)
	}
	ids, ok := received["document_ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "doc1" {
		t.Fatalf("document_ids not sent correctly: %#v", received["document_ids"])
	}
	if len(results) != 1 || results[0].ID != "c1" || results[0].KnowledgeID != "doc1" {
		t.Fatalf("unexpected mapped result: %#v", results)
	}
}

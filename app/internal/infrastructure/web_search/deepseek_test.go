package web_search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeepSeekProviderSearchMapsResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		var req deepSeekSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Query != "test query" || req.Count != 2 {
			t.Errorf("request = %+v", req)
		}
		_, _ = w.Write([]byte(`{
			"results": [
				{
					"title": "First result",
					"url": "https://example.com/a",
					"snippet": "short",
					"summary": "longer model-readable summary",
					"content": "",
					"date": "2026-11-11T10:00:00Z"
				},
				{
					"title": "Second result",
					"url": "https://example.com/b",
					"snippet": "short b",
					"summary": ""
				},
				{
					"title": "Beyond limit",
					"url": "https://example.com/c"
				}
			]
		}`))
	}))
	defer server.Close()

	provider := &DeepSeekProvider{client: server.Client(), baseURL: server.URL, apiKey: "test-key"}
	results, err := provider.Search(context.Background(), "test query", 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Summary != "longer model-readable summary" || results[0].Source != "deepseek" {
		t.Fatalf("first result = %+v", results[0])
	}
	if results[0].PublishedAt == nil || results[0].PublishedAt.Year() != 2026 {
		t.Fatalf("published at = %v", results[0].PublishedAt)
	}
	if results[1].Summary != "" || results[1].Title != "Second result" {
		t.Fatalf("second result = %+v", results[1])
	}
}

func TestDeepSeekProviderErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key","type":"authentication_error"}}`))
	}))
	defer server.Close()

	provider := &DeepSeekProvider{client: server.Client(), baseURL: server.URL, apiKey: "bad-key"}
	if _, err := provider.Search(context.Background(), "query", 5, false); err == nil {
		t.Fatal("expected error for 401 response")
	}
}

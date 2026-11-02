package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gaze/internal/types"
)

// ylswReference is one recalled chunk reference returned by the YLSW engine.
type ylswReference struct {
	ChunkID         string  `json:"chunk_id"`
	AppChunkID      string  `json:"app_chunk_id"`
	DocumentID      string  `json:"document_id"`
	SourceDocID     string  `json:"source_doc_id"`
	KnowledgeBaseID string  `json:"knowledge_base_id"`
	FileName        string  `json:"file_name"`
	DocNumber       string  `json:"doc_number"`
	OrgName         string  `json:"org_name"`
	DocType         string  `json:"doc_type"`
	ProductName     string  `json:"product_name"`
	DocDate         string  `json:"doc_date"`
	Source          string  `json:"source"`
	RerankScore     float64 `json:"rerank_score"`
	// Content 为 chunk 原文摘要（前端悬浮引用卡直接展示）
	Content  string `json:"content"`
	DocTitle string `json:"doc_title"`
	// StartAt/EndAt 为答案文本内的字符偏移（YLSW 后处理对齐），
	// 供前端插入内联引用标记；0 表示该条未对齐。
	StartAt int `json:"start_at"`
	EndAt   int `json:"end_at"`
}

type ylswStreamEvent struct {
	Type       string                 `json:"type"`
	Content    string                 `json:"content"`
	Message    string                 `json:"message"`
	Stage      string                 `json:"stage"`
	TMs        int64                  `json:"t_ms"`
	Counts     map[string]int         `json:"counts"`
	Detail     map[string]interface{} `json:"detail"`
	References []ylswReference        `json:"references"`
}

// ylswEngine implements RAGEngine over the YLSW HTTP API.
type ylswEngine struct{}

// NewYLSWEngine returns the YLSW-backed RAG engine.
func NewYLSWEngine() RAGEngine { return &ylswEngine{} }

// YLSWBaseURL returns the YLSW engine base URL from the environment.
func YLSWBaseURL() string {
	return strings.TrimRight(os.Getenv("YLSW_BASE_URL"), "/")
}

// applyEngineToken sets the shared-secret header (§2.13 P2). Empty token
// (YLSW side not configured) leaves the header unset.
func applyEngineToken(req *http.Request) {
	if token := strings.TrimSpace(os.Getenv("YLSW_ENGINE_TOKEN")); token != "" {
		req.Header.Set("X-Engine-Token", token)
	}
}

// StreamChat calls the YLSW engine's streaming chat endpoint, invoking
// onDelta for each answer fragment and onReferences when the retrieval
// references arrive (before the answer starts streaming). It returns the
// accumulated answer and the recall references.
func (e *ylswEngine) StreamChat(
	ctx context.Context,
	question string,
	knowledgeBaseIDs []string,
	skill *types.SkillInjection,
	onDelta func(string),
	onReferences func([]*types.SearchResult),
	onStage func(stage string, counts map[string]int, tMs int64, detail map[string]interface{}),
) (string, []*types.SearchResult, error) {
	baseURL := YLSWBaseURL()
	if baseURL == "" {
		return "", nil, fmt.Errorf("YLSW_BASE_URL is not configured")
	}

	payload, err := json.Marshal(map[string]interface{}{
		"question":           question,
		"knowledge_base_ids": knowledgeBaseIDs,
		"knowledge_base_id":  firstOrEmpty(knowledgeBaseIDs),
		"skill":              skill,
	})
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, baseURL+"/chat/stream", bytes.NewReader(payload),
	)
	if err != nil {
		return "", nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	applyEngineToken(req)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("call YLSW engine: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", nil, fmt.Errorf("YLSW engine returned %d: %s", resp.StatusCode, string(body))
	}

	var answer strings.Builder
	var refs []*types.SearchResult
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}

		var evt ylswStreamEvent
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}

		switch evt.Type {
		case "delta":
			if evt.Content != "" {
				answer.WriteString(evt.Content)
				if onDelta != nil {
					onDelta(evt.Content)
				}
			}
		case "references":
			refs = mapReferences(evt.References)
			if onReferences != nil {
				onReferences(refs)
			}
		case "stage":
			if onStage != nil && evt.Stage != "" {
				onStage(evt.Stage, evt.Counts, evt.TMs, evt.Detail)
			}
		case "done":
			refs = mapReferences(evt.References)
		case "error":
			return answer.String(), refs, fmt.Errorf("YLSW engine error: %s", evt.Message)
		}
	}
	if err := scanner.Err(); err != nil {
		return answer.String(), refs, fmt.Errorf("read YLSW stream: %w", err)
	}
	return answer.String(), refs, nil
}

func mapReferences(ylswRefs []ylswReference) []*types.SearchResult {
	refs := make([]*types.SearchResult, 0, len(ylswRefs))
	for _, r := range ylswRefs {
		title := r.FileName
		if title == "" {
			title = r.DocNumber
		}
		// KnowledgeID 用 app 侧文档 UUID（source_doc_id）——前端跳转 app
		// 详情页、按文档聚合分组都用它；YLSW 数字 document_id 仅兜底。
		knowledgeID := r.SourceDocID
		if knowledgeID == "" {
			knowledgeID = r.DocumentID
		}
		// 引用/锚点/统计一律使用 app chunk id；YLSW 内部 id 仅诊断透传。
		chunkID := r.AppChunkID
		if chunkID == "" {
			chunkID = r.ChunkID
		}
		refs = append(refs, &types.SearchResult{
			ID:                chunkID,
			KnowledgeID:       knowledgeID,
			KnowledgeBaseID:   r.KnowledgeBaseID,
			KnowledgeTitle:    title,
			KnowledgeFilename: r.FileName,
			Content:           r.Content,
			Score:             r.RerankScore,
			MatchType:         types.MatchTypeEmbedding,
			StartAt:           r.StartAt,
			EndAt:             r.EndAt,
			Metadata: map[string]string{
				"doc_number":      r.DocNumber,
				"org_name":        r.OrgName,
				"doc_type":        r.DocType,
				"product_name":    r.ProductName,
				"doc_title":       r.DocTitle,
				"doc_date":        r.DocDate,
				"source":          r.Source,
				"engine_chunk_id": r.ChunkID,
			},
		})
	}
	return refs
}

// ylswSearchResult is one retrieved chunk returned by the YLSW /search
// endpoint (agent knowledge_search tool).
type ylswSearchResult struct {
	ChunkID     string  `json:"chunk_id"`
	AppChunkID  string  `json:"app_chunk_id"`
	DocumentID  string  `json:"document_id"`
	SourceDocID string  `json:"source_doc_id"`
	FileName    string  `json:"file_name"`
	Content     string  `json:"content"`
	Score       float64 `json:"score"`
	RerankScore float64 `json:"rerank_score"`
	OrgName     string  `json:"org_name"`
	DocType     string  `json:"doc_type"`
	ProductName string  `json:"product_name"`
	DocNumber   string  `json:"doc_number"`
	DocDate     string  `json:"doc_date"`
	Source      string  `json:"source"`
}

// Search runs pure retrieval through the YLSW /search endpoint.
func (e *ylswEngine) Search(
	ctx context.Context,
	queries []string,
	knowledgeBaseIDs []string,
	topK int,
) ([]*types.SearchResult, error) {
	return e.search(ctx, queries, knowledgeBaseIDs, nil, topK)
}

// SearchDocuments restricts retrieval to the selected app-side documents.
func (e *ylswEngine) SearchDocuments(
	ctx context.Context,
	queries []string,
	knowledgeBaseIDs []string,
	documentIDs []string,
	topK int,
) ([]*types.SearchResult, error) {
	return e.search(ctx, queries, knowledgeBaseIDs, documentIDs, topK)
}

func (e *ylswEngine) search(
	ctx context.Context,
	queries []string,
	knowledgeBaseIDs []string,
	documentIDs []string,
	topK int,
) ([]*types.SearchResult, error) {
	baseURL := YLSWBaseURL()
	if baseURL == "" {
		return nil, fmt.Errorf("YLSW_BASE_URL is not configured")
	}

	payloadMap := map[string]interface{}{
		"queries":            queries,
		"knowledge_base_ids": knowledgeBaseIDs,
		"top_k":              topK,
	}
	if len(documentIDs) > 0 {
		payloadMap["document_ids"] = documentIDs
		// 精读是用户显式选定文档的整篇阅读，废止文档也要能读。
		payloadMap["include_expired"] = true
	}
	payload, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, baseURL+"/search", bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	applyEngineToken(req)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call YLSW search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("YLSW search returned %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		Results []ylswSearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode YLSW search response: %w", err)
	}

	results := make([]*types.SearchResult, 0, len(out.Results))
	for _, r := range out.Results {
		title := r.FileName
		if title == "" {
			title = r.DocNumber
		}
		score := r.RerankScore
		if score == 0 {
			score = r.Score
		}
		chunkID := r.AppChunkID
		if chunkID == "" {
			chunkID = r.ChunkID
		}
		knowledgeID := r.SourceDocID
		if knowledgeID == "" {
			knowledgeID = r.DocumentID
		}
		results = append(results, &types.SearchResult{
			ID:                chunkID,
			Content:           r.Content,
			KnowledgeID:       knowledgeID,
			KnowledgeTitle:    title,
			KnowledgeFilename: r.FileName,
			Score:             score,
			MatchType:         types.MatchTypeEmbedding,
			Metadata: map[string]string{
				"org_name":        r.OrgName,
				"doc_type":        r.DocType,
				"product_name":    r.ProductName,
				"doc_number":      r.DocNumber,
				"doc_date":        r.DocDate,
				"source":          r.Source,
				"engine_chunk_id": r.ChunkID,
			},
		})
	}
	return results, nil
}

// IngestDocument pushes app-parsed markdown and app-owned chunks to the
// YLSW /ingest endpoint.
func (e *ylswEngine) IngestDocument(
	ctx context.Context,
	knowledgeBaseID, documentID, fileName, fileType, markdown string,
	chunks []IngestChunk,
	meta *DocumentMetadata,
) error {
	baseURL := YLSWBaseURL()
	if baseURL == "" {
		return fmt.Errorf("YLSW_BASE_URL is not configured")
	}

	payload := map[string]any{
		"knowledge_base_id": knowledgeBaseID,
		"document_id":       documentID,
		"file_name":         fileName,
		"file_type":         fileType,
		"markdown":          markdown,
	}
	if len(chunks) > 0 {
		payload["chunks"] = chunks
	}
	if meta != nil {
		payload["doc_number"] = meta.DocNumber
		payload["org_name"] = meta.OrgName
		payload["doc_date"] = meta.DocDate
		payload["doc_title"] = meta.DocTitle
		payload["doc_type"] = meta.DocType
		payload["product_name"] = meta.ProductName
		if meta.CustomMetadata != nil {
			payload["custom_metadata"] = meta.CustomMetadata
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal ingest request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, baseURL+"/ingest", bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("build ingest request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	applyEngineToken(req)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call YLSW ingest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("YLSW ingest returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// DeleteDocument removes a document from YLSW via /delete_document.
func (e *ylswEngine) DeleteDocument(ctx context.Context, documentID string) error {
	baseURL := YLSWBaseURL()
	if baseURL == "" {
		return fmt.Errorf("YLSW_BASE_URL is not configured")
	}

	payload, err := json.Marshal(map[string]string{"document_id": documentID})
	if err != nil {
		return fmt.Errorf("marshal delete request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, baseURL+"/delete_document", bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("build delete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	applyEngineToken(req)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call YLSW delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("YLSW delete returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// UpdateDocumentMetadata updates document-level metadata in place. It patches
// the engine's in-memory doc metadata so the next query sees the new values
// without re-chunking or re-embedding.
func (e *ylswEngine) UpdateDocumentMetadata(
	ctx context.Context, knowledgeBaseID, documentID string, meta *DocumentMetadata,
) error {
	baseURL := YLSWBaseURL()
	if baseURL == "" {
		return fmt.Errorf("YLSW_BASE_URL is not configured")
	}
	payloadMap := map[string]any{
		"document_id":       documentID,
		"knowledge_base_id": knowledgeBaseID,
	}
	if meta != nil {
		payloadMap["doc_number"] = meta.DocNumber
		payloadMap["org_name"] = meta.OrgName
		payloadMap["doc_date"] = meta.DocDate
		payloadMap["doc_title"] = meta.DocTitle
		payloadMap["doc_type"] = meta.DocType
		payloadMap["product_name"] = meta.ProductName
		if meta.CustomMetadata != nil {
			payloadMap["custom_metadata"] = meta.CustomMetadata
		}
	}
	payload, err := json.Marshal(payloadMap)
	if err != nil {
		return fmt.Errorf("marshal metadata request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, baseURL+"/update_metadata", bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("build metadata request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	applyEngineToken(req)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call YLSW metadata update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("YLSW metadata update returned %d: %s", resp.StatusCode, string(respBody))
	}
	var out struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err == nil && out.Updated == 0 {
		return fmt.Errorf("YLSW metadata update: document not found")
	}
	return nil
}

// firstOrEmpty returns the first non-empty element (legacy single-value
// parameter kept for the YLSW side's backward compatibility).
func firstOrEmpty(ids []string) string {
	for _, id := range ids {
		if id != "" {
			return id
		}
	}
	return ""
}

// ListDocumentIDs returns all document IDs stored in the engine via
// GET /documents/ids (§2.2 ③ reconciliation).
func (e *ylswEngine) IndexStatus(ctx context.Context) (*IndexStatus, error) {
	baseURL := YLSWBaseURL()
	if baseURL == "" {
		return nil, fmt.Errorf("YLSW_BASE_URL is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/index/status", nil)
	if err != nil {
		return nil, fmt.Errorf("build index status request: %w", err)
	}
	applyEngineToken(req)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call YLSW index status: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("YLSW index status returned %d: %s", resp.StatusCode, string(body))
	}
	var out IndexStatus
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode YLSW index status: %w", err)
	}
	return &out, nil
}

func (e *ylswEngine) ListDocumentIDs(ctx context.Context) ([]string, error) {
	baseURL := YLSWBaseURL()
	if baseURL == "" {
		return nil, fmt.Errorf("YLSW_BASE_URL is not configured")
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, baseURL+"/documents/ids", nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	applyEngineToken(req)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call YLSW document ids: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("YLSW document ids returned %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode YLSW document ids response: %w", err)
	}
	return out.IDs, nil
}

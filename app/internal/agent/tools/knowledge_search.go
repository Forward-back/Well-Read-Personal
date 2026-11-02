package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"gaze/internal/config"
	"gaze/internal/engine"
	"gaze/internal/logger"
	"gaze/internal/searchutil"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

var knowledgeSearchTool = BaseTool{
	name: ToolKnowledgeSearch,
	description: `Semantic/vector search tool for retrieving knowledge by meaning, intent, and conceptual relevance.

This tool uses embeddings to understand the user's query and find semantically similar content across knowledge base chunks.

## Purpose
Designed for high-level understanding tasks, such as:
- conceptual explanations
- topic overviews
- reasoning-based information needs
- contextual or intent-driven retrieval
- queries that cannot be answered with literal keyword matching

The tool searches by MEANING rather than exact text. It identifies chunks that are conceptually relevant even when the wording differs.

## What the Tool Does NOT Do
- Does NOT perform exact keyword matching
- Does NOT search for specific named entities
- Should NOT be used for literal lookup tasks
- Should NOT receive long raw text or user messages as queries
- Should NOT be used to locate specific strings or error codes

For literal/keyword/entity search, another tool should be used.

## Required Input Behavior
"queries" must contain **1–5 short, well-formed semantic questions or conceptual statements** that clearly express the meaning the model is trying to retrieve.

Each query should represent a **concept, idea, topic, explanation, or intent**, such as:
- abstract topics
- definitions
- mechanisms
- best practices
- comparisons
- how/why questions

Avoid:
- keyword lists
- raw text from user messages
- full paragraphs
- unprocessed input

## Examples of valid query shapes (not content):
- "What is the main idea of..."
- "How does X work in general?"
- "Explain the purpose of..."
- "What are the key principles behind..."
- "Overview of ..."

## Parameters
- queries (required): 1–5 semantic questions or conceptual statements.
  These should reflect the meaning or topic you want embeddings to capture.
- knowledge_base_ids (optional): limit the search scope.

## Output
Returns chunks ranked by semantic similarity, reranked when applicable.  
Each chunk has a short cN source ID and belongs to a dN document ID. Results represent conceptual relevance, not literal keyword overlap. Use dN for document-level follow-up tool calls.`,
	schema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "queries": {
      "type": "array",
      "description": "REQUIRED: 1-5 semantic questions/topics (e.g., ['What is RAG?', 'RAG benefits'])",
      "items": {
        "type": "string"
      },
      "minItems": 1,
      "maxItems": 5
    },
    "knowledge_base_ids": {
      "type": "array",
      "description": "Optional: bound knowledge-base IDs (the short bN values shown in runtime context)",
      "items": {
        "type": "string"
      },
      "minItems": 0,
      "maxItems": 10
    }
  },
  "required": ["queries"]
}`),
}

// KnowledgeSearchInput defines the input parameters for knowledge search tool
type KnowledgeSearchInput struct {
	Queries          []string `json:"queries"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
}

// searchResultWithMeta wraps search result with metadata about which query matched it
type searchResultWithMeta struct {
	*types.SearchResult
	SourceQuery       string
	QueryType         string // "ylsw" (engine retrieval)
	KnowledgeBaseID   string // ID of the knowledge base this result came from
	KnowledgeBaseType string // Type of the knowledge base (document, faq, etc.)
}

// KnowledgeSearchTool searches knowledge bases with flexible query modes.
// seenChunks lets repeated calls in the same session surface previously-
// returned chunks in a compact form (mirroring gaze_search's de-duping UX)
// so the LLM doesn't burn tokens re-reading identical content.
type KnowledgeSearchTool struct {
	BaseTool
	knowledgeBaseService interfaces.KnowledgeBaseService
	knowledgeService     interfaces.KnowledgeService
	chunkService         interfaces.ChunkService
	searchTargets        types.SearchTargets // Pre-computed unified search targets
	ragEngine            engine.RAGEngine    // YLSW retrieval engine (only engine)
	config               *config.Config      // Global config for fallback values

	seenMu     sync.Mutex
	seenChunks map[string]bool
}

// NewKnowledgeSearchTool creates a new knowledge search tool
func NewKnowledgeSearchTool(
	knowledgeBaseService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	chunkService interfaces.ChunkService,
	searchTargets types.SearchTargets,
	ragEngine engine.RAGEngine,
	cfg *config.Config,
) *KnowledgeSearchTool {
	return &KnowledgeSearchTool{
		BaseTool:             knowledgeSearchTool,
		knowledgeBaseService: knowledgeBaseService,
		knowledgeService:     knowledgeService,
		chunkService:         chunkService,
		searchTargets:        searchTargets,
		ragEngine:            ragEngine,
		config:               cfg,
		seenChunks:           make(map[string]bool),
	}
}

// Execute executes the knowledge search tool
func (t *KnowledgeSearchTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	logger.Infof(ctx, "[Tool][KnowledgeSearch] Execute started")

	// Parse args from json.RawMessage
	var input KnowledgeSearchInput
	if err := json.Unmarshal(args, &input); err != nil {
		logger.Errorf(ctx, "[Tool][KnowledgeSearch] Failed to parse args: %v", err)
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse args: %v", err),
		}, err
	}

	// Log input arguments
	argsJSON, _ := json.MarshalIndent(input, "", "  ")
	logger.Debugf(ctx, "[Tool][KnowledgeSearch] Input args:\n%s", string(argsJSON))

	// Determine which KBs to search - user can optionally filter to specific KBs
	var userSpecifiedKBs []string
	if len(input.KnowledgeBaseIDs) > 0 {
		userSpecifiedKBs = input.KnowledgeBaseIDs
		if err := validateKnowledgeBaseIDsInSearchTargets(t.searchTargets, userSpecifiedKBs); err != nil {
			return &types.ToolResult{Success: false, Error: err.Error()}, err
		}
		logger.Infof(ctx, "[Tool][KnowledgeSearch] User specified %d knowledge bases: %v", len(userSpecifiedKBs), userSpecifiedKBs)
	}

	// Use pre-computed search targets, optionally filtered by user-specified KBs
	searchTargets := t.searchTargets
	if len(userSpecifiedKBs) > 0 {
		// Filter search targets to only include user-specified KBs
		userKBSet := make(map[string]bool)
		for _, kbID := range userSpecifiedKBs {
			userKBSet[kbID] = true
		}
		var filteredTargets types.SearchTargets
		for _, target := range t.searchTargets {
			if target == nil {
				continue
			}
			if userKBSet[target.KnowledgeBaseID] {
				filteredTargets = append(filteredTargets, target)
			}
		}
		searchTargets = filteredTargets
	}

	// Validate search targets
	if len(searchTargets) == 0 {
		logger.Errorf(ctx, "[Tool][KnowledgeSearch] No search targets available")
		return &types.ToolResult{
			Success: false,
			Error:   "no knowledge bases specified and no search targets configured",
		}, fmt.Errorf("no search targets available")
	}

	kbIDs := searchTargets.GetAllKnowledgeBaseIDs()
	logger.Infof(ctx, "[Tool][KnowledgeSearch] Using %d search targets across %d KBs", len(searchTargets), len(kbIDs))

	// Parse query parameter
	queries := input.Queries

	// Validate: query must be provided
	if len(queries) == 0 {
		logger.Errorf(ctx, "[Tool][KnowledgeSearch] No queries provided")
		return &types.ToolResult{
			Success: false,
			Error:   "queries parameter is required",
		}, fmt.Errorf("no queries provided")
	}

	logger.Infof(ctx, "[Tool][KnowledgeSearch] Queries: %v", queries)

	// Search parameters: fall back to global config, then to hardcoded defaults.
	var topK int
	if topK == 0 && t.config != nil {
		topK = t.config.Conversation.EmbeddingTopK
	}
	if topK == 0 {
		topK = 5
	}
	logger.Infof(ctx, "[Tool][KnowledgeSearch] Search params: top_k=%d", topK)

	// 检索统一走 YLSW 引擎（唯一检索引擎）：多查询合并去重 + RRF + 精排
	// 由引擎完成，工具层不再做二次 rerank/MMR。
	engineResults, err := t.ragEngine.Search(ctx, queries, kbIDs, types.TenantWideScope(), topK)
	if err != nil {
		logger.Errorf(ctx, "[Tool][KnowledgeSearch] Engine search failed: %v", err)
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("知识库检索失败: %v", err),
		}, err
	}

	deduplicatedResults := make([]*searchResultWithMeta, 0, len(engineResults))
	for _, r := range engineResults {
		deduplicatedResults = append(deduplicatedResults, &searchResultWithMeta{
			SearchResult: r,
			QueryType:    "ylsw",
		})
	}
	logger.Infof(ctx, "[Tool][KnowledgeSearch] Engine search completed: %d results", len(deduplicatedResults))

	// Enrich image info for search results (lazy-loaded from child image chunks)
	if t.chunkService != nil && len(deduplicatedResults) > 0 {
		byTenant := make(map[uint64][]*types.SearchResult)
		for _, r := range deduplicatedResults {
			tid := t.searchTargets.GetTenantIDForKB(r.KnowledgeBaseID)
			if tid == 0 {
				continue
			}
			byTenant[tid] = append(byTenant[tid], r.SearchResult)
		}
		for tid, batch := range byTenant {
			searchutil.EnrichSearchResultsImageInfo(ctx, t.chunkService.GetRepository(), tid, batch)
		}
	}

	// Build output
	logger.Infof(ctx, "[Tool][KnowledgeSearch] Formatting output with %d final results", len(deduplicatedResults))
	result, err := t.formatOutput(ctx, deduplicatedResults, kbIDs, queries)
	if err != nil {
		logger.Errorf(ctx, "[Tool][KnowledgeSearch] Failed to format output: %v", err)
		return result, err
	}
	logger.Infof(ctx, "[Tool][KnowledgeSearch] Output: %s", result.Output)
	return result, nil
}

func (t *KnowledgeSearchTool) getFAQMetadata(
	ctx context.Context,
	chunkID string,
	cache map[string]*types.FAQChunkMetadata,
) (*types.FAQChunkMetadata, error) {
	if chunkID == "" || t.chunkService == nil {
		return nil, nil
	}

	if meta, ok := cache[chunkID]; ok {
		return meta, nil
	}

	chunk, err := t.chunkService.GetChunkByID(ctx, chunkID)
	if err != nil {
		cache[chunkID] = nil
		return nil, err
	}
	if chunk == nil {
		cache[chunkID] = nil
		return nil, nil
	}

	meta, err := chunk.FAQMetadata()
	if err != nil {
		cache[chunkID] = nil
		return nil, err
	}
	cache[chunkID] = meta
	return meta, nil
}

// writeKnowledgeMetadataHeader emits document-scoped metadata once per
// knowledge item. Chunk entries keep only chunk-specific content so repeated
// results from the same document do not waste model context.
func writeKnowledgeMetadataHeader(ob *strings.Builder, results []*searchResultWithMeta) {
	seen := make(map[string]struct{}, len(results))
	hasMetadata := false
	var documents strings.Builder
	for _, result := range results {
		if result == nil || result.SearchResult == nil || result.KnowledgeID == "" || result.KnowledgeCustomMetadata == "" {
			continue
		}
		if _, ok := seen[result.KnowledgeID]; ok {
			continue
		}
		seen[result.KnowledgeID] = struct{}{}
		hasMetadata = true
		documents.WriteString(fmt.Sprintf(
			"<document knowledge_id=\"%s\" knowledge_base_id=\"%s\" title=\"%s\">\n",
			xmlEscape(result.KnowledgeID),
			xmlEscape(result.KnowledgeBaseID),
			xmlEscape(result.KnowledgeTitle),
		))
		documents.WriteString(fmt.Sprintf("<metadata>%s</metadata>\n", xmlEscape(result.KnowledgeCustomMetadata)))
		documents.WriteString("</document>\n")
	}
	if !hasMetadata {
		return
	}
	ob.WriteString("<documents>\n")
	ob.WriteString(documents.String())
	ob.WriteString("</documents>\n")
}

// formatOutput formats the search results for display
func (t *KnowledgeSearchTool) formatOutput(
	ctx context.Context,
	results []*searchResultWithMeta,
	kbsToSearch []string,
	queries []string,
) (*types.ToolResult, error) {
	if len(results) == 0 {
		data := map[string]interface{}{
			"knowledge_base_ids": kbsToSearch,
			"results":            []interface{}{},
			"count":              0,
		}
		if len(queries) > 0 {
			data["queries"] = queries
		}
		output := fmt.Sprintf("No relevant content found in %d knowledge base(s).\n\n", len(kbsToSearch))
		output += "=== ⚠️ CRITICAL - Next Steps ===\n"
		output += "- ❌ DO NOT use training data or general knowledge to answer\n"
		output += "- ✅ If web_search is enabled: You MUST use web_search to find information\n"
		output += "- ✅ If web_search is disabled: State 'I couldn't find relevant information in the knowledge base'\n"
		output += "- NEVER fabricate or infer answers - ONLY use retrieved content\n"

		return &types.ToolResult{
			Success: true,
			Output:  output,
			Data:    data,
		}, nil
	}

	// Count results by KB
	kbCounts := make(map[string]int)
	for _, r := range results {
		kbCounts[r.KnowledgeBaseID]++
	}

	// Format individual results as XML. Tag names are kept in sync with
	// gaze_search (`<search_results>`, per-entry element, `<query>`) so that
	// agents and downstream consumers see a single consistent shape across
	// all retrieval tools.
	var ob strings.Builder
	ob.WriteString(fmt.Sprintf("<search_results count=\"%d\">\n", len(results)))
	for _, q := range queries {
		ob.WriteString(fmt.Sprintf("<query>%s</query>\n", xmlEscape(q)))
	}
	writeKnowledgeMetadataHeader(&ob, results)

	formattedResults := make([]map[string]interface{}, 0, len(results))
	enabled := true

	faqMetadataCache := make(map[string]*types.FAQChunkMetadata)

	knowledgeChunkMap := make(map[string]map[int]bool)
	knowledgeTotalMap := make(map[string]int64)
	knowledgeTitleMap := make(map[string]string)

	for i, result := range results {
		var faqMeta *types.FAQChunkMetadata
		if result.KnowledgeBaseType == types.KnowledgeBaseTypeFAQ {
			meta, err := t.getFAQMetadata(ctx, result.ID, faqMetadataCache)
			if err != nil {
				logger.Warnf(ctx, "[Tool][KnowledgeSearch] Failed to load FAQ metadata for chunk %s: %v", result.ID, err)
			} else {
				faqMeta = meta
			}
		}

		if knowledgeChunkMap[result.KnowledgeID] == nil {
			knowledgeChunkMap[result.KnowledgeID] = make(map[int]bool)
		}
		knowledgeChunkMap[result.KnowledgeID][result.ChunkIndex] = true
		knowledgeTitleMap[result.KnowledgeID] = result.KnowledgeTitle

		// Cache total chunk count per knowledge
		if _, exists := knowledgeTotalMap[result.KnowledgeID]; !exists {
			effectiveTenantID := t.searchTargets.GetTenantIDForKB(result.KnowledgeBaseID)
			if effectiveTenantID == 0 {
				logger.Warnf(ctx, "[Tool][KnowledgeSearch] KB %s not found in searchTargets, skipping chunk count", result.KnowledgeBaseID)
				knowledgeTotalMap[result.KnowledgeID] = 0
			} else {
				// Use the same chunk-type filter as list_knowledge_chunks so the
				// total reported here matches what list_knowledge_chunks can page
				// over. Mismatched filters previously let LLMs compute offsets
				// against an inflated/deflated total and page past the end.
				_, total, err := t.chunkService.GetRepository().ListPagedChunksByKnowledgeID(ctx,
					effectiveTenantID, result.KnowledgeID,
					&types.Pagination{Page: 1, PageSize: 1},
					[]types.ChunkType{types.ChunkTypeText, types.ChunkTypeFAQ}, nil, "", "", "", "",
					&enabled,
				)
				if err != nil {
					logger.Warnf(ctx, "[Tool][KnowledgeSearch] Failed to get total chunks for knowledge %s: %v", result.KnowledgeID, err)
					knowledgeTotalMap[result.KnowledgeID] = 0
				} else {
					knowledgeTotalMap[result.KnowledgeID] = total
				}
			}
		}

		t.seenMu.Lock()
		seen := t.seenChunks[result.ID]
		t.seenChunks[result.ID] = true
		t.seenMu.Unlock()

		isFAQ := faqMeta != nil
		if seen {
			// Compact rendering for chunks we already returned in a previous
			// knowledge_search call during this session. The model has the
			// content in context already, so re-emitting it only burns tokens.
			if isFAQ {
				ob.WriteString(fmt.Sprintf(
					"<faq rank=\"%d\" faq_id=\"%s\" index=\"%d\" knowledge_base_id=\"%s\" knowledge_title=\"%s\" score=\"%.3f\" source_query=\"%s\" already_seen=\"true\">\n",
					i+1,
					xmlEscape(result.ID),
					result.ChunkIndex,
					xmlEscape(result.KnowledgeBaseID),
					xmlEscape(result.KnowledgeTitle),
					result.Score,
					xmlEscape(result.SourceQuery),
				))
			} else {
				ob.WriteString(fmt.Sprintf(
					"<chunk rank=\"%d\" chunk_id=\"%s\" chunk_index=\"%d\" knowledge_id=\"%s\" knowledge_base_id=\"%s\" knowledge_title=\"%s\" score=\"%.3f\" source_query=\"%s\" already_seen=\"true\">\n",
					i+1,
					xmlEscape(result.ID),
					result.ChunkIndex,
					xmlEscape(result.KnowledgeID),
					xmlEscape(result.KnowledgeBaseID),
					xmlEscape(result.KnowledgeTitle),
					result.Score,
					xmlEscape(result.SourceQuery),
				))
			}
			ob.WriteString("<note>(content omitted, already returned in a previous knowledge_search call this session)</note>\n")
			if isFAQ {
				ob.WriteString("</faq>\n")
			} else {
				ob.WriteString("</chunk>\n")
			}
		} else {
			if isFAQ {
				ob.WriteString(fmt.Sprintf(
					"<faq rank=\"%d\" faq_id=\"%s\" index=\"%d\" knowledge_base_id=\"%s\" knowledge_title=\"%s\" score=\"%.3f\" source_query=\"%s\">\n",
					i+1,
					xmlEscape(result.ID),
					result.ChunkIndex,
					xmlEscape(result.KnowledgeBaseID),
					xmlEscape(result.KnowledgeTitle),
					result.Score,
					xmlEscape(result.SourceQuery),
				))
			} else {
				ob.WriteString(fmt.Sprintf(
					"<chunk rank=\"%d\" chunk_id=\"%s\" chunk_index=\"%d\" knowledge_id=\"%s\" knowledge_base_id=\"%s\" knowledge_title=\"%s\" score=\"%.3f\" source_query=\"%s\">\n",
					i+1,
					xmlEscape(result.ID),
					result.ChunkIndex,
					xmlEscape(result.KnowledgeID),
					xmlEscape(result.KnowledgeBaseID),
					xmlEscape(result.KnowledgeTitle),
					result.Score,
					xmlEscape(result.SourceQuery),
				))
			}
			snippet := ""
			if faqMeta != nil {
				snippet = faqMatchSnippetFromQueries(faqMeta, queries)
			}
			if snippet == "" {
				snippet = extractSnippetForQueries(result.Content, queries)
			}
			if snippet != "" {
				ob.WriteString(fmt.Sprintf("<match_snippet>%s</match_snippet>\n", xmlEscape(snippet)))
			}
			ob.WriteString(fmt.Sprintf("<content>%s</content>\n", result.Content))

			if result.ImageInfo != "" {
				var imageInfos []types.ImageInfo
				if err := json.Unmarshal([]byte(result.ImageInfo), &imageInfos); err == nil && len(imageInfos) > 0 {
					for _, img := range imageInfos {
						if imageMarkdown := searchutil.BuildImageInfoMarkdownWithURL(img.URL, &img); imageMarkdown != "" {
							ob.WriteString(imageMarkdown)
							ob.WriteString("\n")
						}
					}
				}
			}

			if isFAQ {
				writeFAQFieldsXML(&ob, faqMeta)
				ob.WriteString("</faq>\n")
			} else {
				ob.WriteString("</chunk>\n")
			}
		}

		formattedResults = append(formattedResults, map[string]interface{}{
			"result_index":        i + 1,
			"content":             result.Content,
			"knowledge_id":        result.KnowledgeID,
			"knowledge_base_id":   result.KnowledgeBaseID,
			"knowledge_title":     result.KnowledgeTitle,
			"knowledge_metadata":  result.KnowledgeCustomMetadata,
			"match_type":          result.MatchType,
			"source_query":        result.SourceQuery,
			"query_type":          result.QueryType,
			"knowledge_base_type": result.KnowledgeBaseType,
		})

		last := formattedResults[len(formattedResults)-1]

		if result.ImageInfo != "" {
			var imageInfos []types.ImageInfo
			if err := json.Unmarshal([]byte(result.ImageInfo), &imageInfos); err == nil && len(imageInfos) > 0 {
				imageList := make([]map[string]string, 0, len(imageInfos))
				for _, img := range imageInfos {
					imgData := make(map[string]string)
					if img.URL != "" {
						imgData["url"] = img.URL
					}
					if img.Caption != "" {
						imgData["caption"] = img.Caption
					}
					if img.OCRText != "" {
						imgData["ocr_text"] = img.OCRText
					}
					if len(imgData) > 0 {
						imageList = append(imageList, imgData)
					}
				}
				if len(imageList) > 0 {
					last["images"] = imageList
				}
			}
		}

		if faqMeta != nil {
			last["faq_id"] = result.ID
			last["index"] = result.ChunkIndex
			if faqMeta.StandardQuestion != "" {
				last["faq_standard_question"] = faqMeta.StandardQuestion
			}
			appendSimilarQuestionsToChunkData(last, faqMeta.SimilarQuestions)
			if len(faqMeta.Answers) > 0 {
				last["faq_answers"] = faqMeta.Answers
			}
		} else {
			last["chunk_id"] = result.ID
			last["chunk_index"] = result.ChunkIndex
		}
	}

	// Retrieval statistics
	ob.WriteString("<retrieval_statistics>\n")
	for knowledgeID, retrievedChunks := range knowledgeChunkMap {
		totalChunks := knowledgeTotalMap[knowledgeID]
		retrievedCount := len(retrievedChunks)
		title := knowledgeTitleMap[knowledgeID]
		if totalChunks > 0 {
			remaining := totalChunks - int64(retrievedCount)
			percentage := float64(retrievedCount) / float64(totalChunks) * 100
			ob.WriteString(fmt.Sprintf("<document_stat knowledge_id=\"%s\" title=\"%s\" total_chunks=\"%d\" retrieved=\"%d\" remaining=\"%d\" coverage=\"%.1f%%\" />\n",
				xmlEscape(knowledgeID), xmlEscape(title), totalChunks, retrievedCount, remaining, percentage))
		}
	}
	ob.WriteString("</retrieval_statistics>\n")
	ob.WriteString("</search_results>")

	output := ob.String()

	data := map[string]interface{}{
		"knowledge_base_ids": kbsToSearch,
		"results":            formattedResults,
		"count":              len(formattedResults),
		"kb_counts":          kbCounts,
		"display_type":       "search_results",
	}

	if len(queries) > 0 {
		data["queries"] = queries
	}

	return &types.ToolResult{
		Success: true,
		Output:  output,
		Data:    data,
	}, nil
}

// extractSnippetForQueries tries to produce a short contextual snippet around
// the first occurrence of any token extracted from the provided queries.
// When no token matches (common for fully paraphrased semantic queries) it
// falls back to the leading 160 runes of content so callers always get
// something to scan. The snippet is single-lined and bounded in length to
// keep the rendered XML compact.
func extractSnippetForQueries(content string, queries []string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	tokens := searchQueryTokens(queries)

	lowered := strings.ToLower(content)
	earliest := -1
	earliestEnd := -1
	for _, tok := range tokens {
		idx := strings.Index(lowered, tok)
		if idx < 0 {
			continue
		}
		end := idx + len(tok)
		if earliest < 0 || idx < earliest {
			earliest = idx
			earliestEnd = end
		}
	}

	if earliest < 0 {
		runes := []rune(content)
		if len(runes) > snippetContextRunes*2 {
			return strings.TrimSpace(string(runes[:snippetContextRunes*2])) + " ..."
		}
		return content
	}

	matchStr := content[earliest:earliestEnd]
	before := content[:earliest]
	after := content[earliestEnd:]

	beforeRunes := []rune(before)
	if len(beforeRunes) > snippetContextRunes {
		beforeRunes = beforeRunes[len(beforeRunes)-snippetContextRunes:]
	}
	afterRunes := []rune(after)
	if len(afterRunes) > snippetContextRunes {
		afterRunes = afterRunes[:snippetContextRunes]
	}

	snippet := string(beforeRunes) + matchStr + string(afterRunes)
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	for strings.Contains(snippet, "  ") {
		snippet = strings.ReplaceAll(snippet, "  ", " ")
	}
	return "... " + strings.TrimSpace(snippet) + " ..."
}

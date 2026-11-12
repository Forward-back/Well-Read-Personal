package web_search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

const (
	defaultDeepSeekSearchURL = "https://api.deepseek.com/v1/tools/search"
	defaultDeepSeekTimeout   = 20 * time.Second
)

// DeepSeekProvider uses DeepSeek's standalone Search API. Chat Completions does
// not expose a native web-search tool, so search results are fetched first and
// injected into the direct-mode prompt.
type DeepSeekProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

func NewDeepSeekProvider(params types.WebSearchProviderParameters) (interfaces.WebSearchProvider, error) {
	if strings.TrimSpace(params.APIKey) == "" {
		return nil, fmt.Errorf("API key is required for DeepSeek search provider")
	}
	client, err := NewSearchHTTPClient(defaultDeepSeekTimeout, params.ProxyURL)
	if err != nil {
		return nil, err
	}
	baseURL := defaultDeepSeekSearchURL
	if strings.TrimSpace(params.BaseURL) != "" {
		baseURL = strings.TrimSpace(params.BaseURL)
	}
	return &DeepSeekProvider{client: client, baseURL: baseURL, apiKey: params.APIKey}, nil
}

func (p *DeepSeekProvider) Name() string { return "deepseek" }

type deepSeekSearchRequest struct {
	Query       string `json:"query"`
	Count       int    `json:"count,omitempty"`
	NeedContent bool   `json:"need_content"`
}

type deepSeekSearchResponse struct {
	Results []struct {
		Title       string  `json:"title"`
		URL         string  `json:"url"`
		Snippet     string  `json:"snippet"`
		Summary     string  `json:"summary"`
		Content     string  `json:"content"`
		SiteName    string  `json:"site_name"`
		Date        string  `json:"date"`
		Authority   int     `json:"authority"`
		RerankScore float64 `json:"rerank_score"`
	} `json:"results"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (p *DeepSeekProvider) Search(
	ctx context.Context,
	query string,
	maxResults int,
	_ bool,
) ([]*types.WebSearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return []*types.WebSearchResult{}, nil
	}
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 50 {
		maxResults = 50
	}

	body, err := json.Marshal(deepSeekSearchRequest{
		Query: query,
		Count: maxResults,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal DeepSeek search request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create DeepSeek search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform DeepSeek search: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read DeepSeek search response: %w", err)
	}
	var parsed deepSeekSearchResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, fmt.Errorf("decode DeepSeek search response (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		message := strings.TrimSpace(string(payload))
		if parsed.Error != nil && parsed.Error.Message != "" {
			message = parsed.Error.Message
		}
		return nil, fmt.Errorf("DeepSeek search returned status %d: %s", resp.StatusCode, message)
	}

	results := make([]*types.WebSearchResult, 0, len(parsed.Results))
	for _, item := range parsed.Results {
		if len(results) >= maxResults {
			break
		}
		url := strings.TrimSpace(item.URL)
		if url == "" {
			continue
		}
		result := &types.WebSearchResult{
			Title:   strings.TrimSpace(item.Title),
			URL:     url,
			Snippet: strings.TrimSpace(item.Snippet),
			Summary: strings.TrimSpace(item.Summary),
			Content: strings.TrimSpace(item.Content),
			Source:  "deepseek",
		}
		if item.Date != "" {
			if published, err := time.Parse(time.RFC3339, item.Date); err == nil {
				result.PublishedAt = &published
			} else {
				result.Age = item.Date
			}
		}
		results = append(results, result)
	}
	return results, nil
}

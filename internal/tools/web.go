package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	webFetchTimeout   = 15 * time.Second
	defaultMaxBytes   = 32000
	defaultMaxResults = 5
)

var (
	httpClient  = &http.Client{Timeout: webFetchTimeout}
	htmlTagsRe  = regexp.MustCompile(`<[^>]+>`)
	ddgBaseURL  = "https://api.duckduckgo.com/"
)

// --- web_fetch ---

type webFetchTool struct{}

func NewWebFetchTool() Tool { return &webFetchTool{} }

func (t *webFetchTool) Name() string        { return "web_fetch" }
func (t *webFetchTool) Destructive() bool   { return false }
func (t *webFetchTool) Description() string { return "Fetch the text content of a URL." }
func (t *webFetchTool) Parameters() map[string]any {
	return map[string]any{
		"url":       map[string]any{"type": "string", "description": "URL to fetch"},
		"max_bytes": map[string]any{"type": "integer", "description": "Max bytes to read (default 32000)"},
	}
}

type webFetchParams struct {
	URL      string `json:"url"`
	MaxBytes int    `json:"max_bytes"`
}

func (t *webFetchTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p webFetchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("web fetch: parse params: %w", err)
	}
	if p.MaxBytes <= 0 {
		p.MaxBytes = defaultMaxBytes
	}
	return fetchURL(ctx, p.URL, p.MaxBytes)
}

func fetchURL(ctx context.Context, rawURL string, maxBytes int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("web fetch: build request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("web fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)))
	if err != nil {
		return "", fmt.Errorf("web fetch: read body: %w", err)
	}
	content := string(body)
	if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		content = htmlTagsRe.ReplaceAllString(content, "")
	}
	return strings.TrimSpace(content), nil
}

// --- web_search ---

type webSearchTool struct{}

func NewWebSearchTool() Tool { return &webSearchTool{} }

func (t *webSearchTool) Name() string        { return "web_search" }
func (t *webSearchTool) Destructive() bool   { return false }
func (t *webSearchTool) Description() string { return "Search the web via DuckDuckGo and return top results." }
func (t *webSearchTool) Parameters() map[string]any {
	return map[string]any{
		"query":       map[string]any{"type": "string", "description": "Search query"},
		"max_results": map[string]any{"type": "integer", "description": "Max results to return (default 5)"},
	}
}

type webSearchParams struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

type ddgResponse struct {
	AbstractText  string `json:"AbstractText"`
	RelatedTopics []struct {
		Text string `json:"Text"`
	} `json:"RelatedTopics"`
}

func (t *webSearchTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p webSearchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("web search: parse params: %w", err)
	}
	if p.MaxResults <= 0 {
		p.MaxResults = defaultMaxResults
	}
	return searchDDG(ctx, p.Query, p.MaxResults)
}

func searchDDG(ctx context.Context, query string, maxResults int) (string, error) {
	endpoint := ddgBaseURL + "?q=" + url.QueryEscape(query) + "&format=json&no_redirect=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("web search: build request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("web search: %w", err)
	}
	defer resp.Body.Close()

	var ddg ddgResponse
	if err := json.NewDecoder(resp.Body).Decode(&ddg); err != nil {
		return "", fmt.Errorf("web search: decode response: %w", err)
	}
	return formatDDGResults(ddg, maxResults), nil
}

func formatDDGResults(ddg ddgResponse, max int) string {
	var results []string
	if ddg.AbstractText != "" {
		results = append(results, ddg.AbstractText)
	}
	for _, topic := range ddg.RelatedTopics {
		if len(results) >= max {
			break
		}
		if topic.Text != "" {
			results = append(results, topic.Text)
		}
	}
	return strings.Join(results, "\n\n")
}

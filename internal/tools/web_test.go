package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebFetchTool_PlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello from server"))
	}))
	t.Cleanup(srv.Close)

	tool := NewWebFetchTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"url":       srv.URL,
		"max_bytes": 1000,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "hello from server") {
		t.Errorf("got: %q", out)
	}
}

func TestWebFetchTool_StripHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><p>clean text</p></body></html>"))
	}))
	t.Cleanup(srv.Close)

	tool := NewWebFetchTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"url": srv.URL, "max_bytes": 0}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(out, "<") {
		t.Errorf("HTML tags should be stripped: %q", out)
	}
	if !strings.Contains(out, "clean text") {
		t.Errorf("text content missing: %q", out)
	}
}

func TestWebFetchTool_NotDestructive(t *testing.T) {
	if NewWebFetchTool().Destructive() {
		t.Error("web_fetch must not be destructive")
	}
}

func TestWebSearchTool_ReturnsResults(t *testing.T) {
	response := map[string]any{
		"AbstractText": "Go is a programming language.",
		"RelatedTopics": []map[string]any{
			{"Text": "Golang is fast."},
			{"Text": "Golang is simple."},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(srv.Close)

	// Override the DDG base URL to point to our mock server
	orig := ddgBaseURL
	ddgBaseURL = srv.URL + "/"
	t.Cleanup(func() { ddgBaseURL = orig })

	tool := NewWebSearchTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"query":       "golang",
		"max_results": 3,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "Go is a programming language.") {
		t.Errorf("abstract missing: %q", out)
	}
	if !strings.Contains(out, "Golang is fast.") {
		t.Errorf("related topic missing: %q", out)
	}
}

func TestWebSearchTool_MaxResultsRespected(t *testing.T) {
	response := map[string]any{
		"AbstractText": "",
		"RelatedTopics": []map[string]any{
			{"Text": "result 1"},
			{"Text": "result 2"},
			{"Text": "result 3"},
			{"Text": "result 4"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(srv.Close)

	orig := ddgBaseURL
	ddgBaseURL = srv.URL + "/"
	t.Cleanup(func() { ddgBaseURL = orig })

	tool := NewWebSearchTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"query": "test", "max_results": 2}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(out, "result 3") {
		t.Errorf("max_results not respected: %q", out)
	}
}

func TestWebSearchTool_NotDestructive(t *testing.T) {
	if NewWebSearchTool().Destructive() {
		t.Error("web_search must not be destructive")
	}
}

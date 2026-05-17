package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// --- mock server helpers ---

type mockResponse struct {
	status int
	body   any
}

func chatResponse(content string, toolCalls []mockToolCall) any {
	calls := make([]any, len(toolCalls))
	for i, tc := range toolCalls {
		calls[i] = map[string]any{
			"id":   tc.id,
			"type": "function",
			"function": map[string]any{
				"name":      tc.name,
				"arguments": tc.arguments,
			},
		}
	}
	msg := map[string]any{"role": "assistant", "content": content}
	if len(calls) > 0 {
		msg["tool_calls"] = calls
	}
	return map[string]any{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": 1677652288,
		"model":   "test-model",
		"choices": []any{map[string]any{"index": 0, "message": msg, "finish_reason": "stop"}},
	}
}

func embedResponse(vector []float32) any {
	return map[string]any{
		"object": "list",
		"data": []any{map[string]any{
			"object":    "embedding",
			"embedding": vector,
			"index":     0,
		}},
		"model": "test-embed",
		"usage": map[string]any{"prompt_tokens": 1, "total_tokens": 1},
	}
}

func apiErrorBody(code int, msg string) any {
	return map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    "api_error",
			"code":    code,
		},
	}
}

type mockToolCall struct {
	id        string
	name      string
	arguments string
}

// newMockServer creates an httptest server that serves responses from the queue in order.
// Each path (/v1/chat/completions or /v1/embeddings) is matched and served from its own queue.
func newMockServer(t *testing.T, responses []mockResponse) *httptest.Server {
	t.Helper()
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(callCount.Add(1)) - 1
		if idx >= len(responses) {
			t.Errorf("unexpected request #%d to %s", idx+1, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
			return
		}
		mr := responses[idx]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(mr.status)
		if err := json.NewEncoder(w).Encode(mr.body); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newProvider(t *testing.T, srv *httptest.Server) *LMStudio {
	t.Helper()
	return NewLMStudio(srv.URL+"/v1", "test-model", "test-embed", 0.2, 2, 1)
}

// --- Chat tests ---

func TestChat_SuccessPlainContent(t *testing.T) {
	srv := newMockServer(t, []mockResponse{
		{http.StatusOK, chatResponse("Hello from model", nil)},
	})
	p := newProvider(t, srv)

	resp, err := p.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "Hello from model" {
		t.Errorf("Content: got %q", resp.Content)
	}
	if resp.Done {
		t.Error("Done should be false for non-DONE content")
	}
}

func TestChat_DoneSignalSetsDone(t *testing.T) {
	srv := newMockServer(t, []mockResponse{
		{http.StatusOK, chatResponse("Task complete.\nDONE", nil)},
	})
	p := newProvider(t, srv)

	resp, err := p.Chat(context.Background(), []Message{{Role: RoleUser, Content: "do it"}}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !resp.Done {
		t.Error("Done should be true when last line is DONE")
	}
}

func TestChat_DoneNotSetWhenDoneNotLastLine(t *testing.T) {
	srv := newMockServer(t, []mockResponse{
		{http.StatusOK, chatResponse("DONE\nmore content after", nil)},
	})
	p := newProvider(t, srv)

	resp, err := p.Chat(context.Background(), []Message{{Role: RoleUser, Content: "do it"}}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Done {
		t.Error("Done should be false when DONE is not the last line")
	}
}

func TestChat_ToolCallsInResponse(t *testing.T) {
	srv := newMockServer(t, []mockResponse{
		{http.StatusOK, chatResponse("", []mockToolCall{
			{id: "call_1", name: "read_file", arguments: `{"path":"main.go"}`},
		})},
	})
	p := newProvider(t, srv)

	resp, err := p.Chat(context.Background(), []Message{{Role: RoleUser, Content: "read it"}}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls: got %d, want 1", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_1" || tc.Name != "read_file" || tc.Arguments != `{"path":"main.go"}` {
		t.Errorf("ToolCall fields wrong: %+v", tc)
	}
	if resp.Done {
		t.Error("Done must be false when tool calls are present")
	}
}

func TestChat_Retry_SucceedsOnSecondAttempt(t *testing.T) {
	srv := newMockServer(t, []mockResponse{
		{http.StatusTooManyRequests, apiErrorBody(429, "rate limited")},
		{http.StatusOK, chatResponse("ok after retry", nil)},
	})
	p := newProvider(t, srv)

	resp, err := p.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "ok after retry" {
		t.Errorf("Content: got %q", resp.Content)
	}
}

func TestChat_Retry_ExhaustsRetries(t *testing.T) {
	// maxRetries=2 → 3 total attempts
	srv := newMockServer(t, []mockResponse{
		{http.StatusTooManyRequests, apiErrorBody(429, "rate limited")},
		{http.StatusTooManyRequests, apiErrorBody(429, "rate limited")},
		{http.StatusTooManyRequests, apiErrorBody(429, "rate limited")},
	})
	p := newProvider(t, srv)

	_, err := p.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error after exhausted retries, got nil")
	}
}

func TestChat_ServerError_Retried(t *testing.T) {
	srv := newMockServer(t, []mockResponse{
		{http.StatusInternalServerError, apiErrorBody(500, "server error")},
		{http.StatusOK, chatResponse("recovered", nil)},
	})
	p := newProvider(t, srv)

	resp, err := p.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "recovered" {
		t.Errorf("Content: got %q", resp.Content)
	}
}

// --- Embed tests ---

func TestEmbed_Success(t *testing.T) {
	want := []float32{0.1, 0.2, 0.3}
	srv := newMockServer(t, []mockResponse{
		{http.StatusOK, embedResponse(want)},
	})
	p := newProvider(t, srv)

	got, err := p.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("embedding length: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("embedding[%d]: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestEmbed_ServerError(t *testing.T) {
	srv := newMockServer(t, []mockResponse{
		{http.StatusInternalServerError, apiErrorBody(500, "server error")},
		{http.StatusInternalServerError, apiErrorBody(500, "server error")},
		{http.StatusInternalServerError, apiErrorBody(500, "server error")},
	})
	p := newProvider(t, srv)

	_, err := p.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- unit tests for helpers ---

func TestIsDone(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"DONE", true},
		{"Task complete.\nDONE", true},
		{"Task complete.\nDONE\n", true},
		{"Task complete.\nDONE\n\n  ", true},
		{"DONE\nmore text", false},
		{"notdone", false},
		{"", false},
		{"done", false}, // case-sensitive
	}
	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			if got := isDone(tt.content); got != tt.want {
				t.Errorf("isDone(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	// non-API errors should not be retried
	if shouldRetry(context.DeadlineExceeded) {
		t.Error("deadline exceeded should not be retried")
	}
}

package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/xdreamer/xdreamer/internal/model"
)

// mockProvider is a minimal model.Provider for codegen tests.
type mockProvider struct {
	response model.Response
	err      error
}

func (m *mockProvider) Chat(_ context.Context, messages []model.Message, _ []model.ToolDefinition) (model.Response, error) {
	return m.response, m.err
}

func (m *mockProvider) Embed(_ context.Context, _ string) ([]float32, error) {
	return nil, nil
}

func TestCodeGenTool_ReturnsModelContent(t *testing.T) {
	p := &mockProvider{response: model.Response{Content: "func Hello() {}"}}
	tool := NewCodeGenTool(p)

	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{
		"prompt":   "a hello function",
		"language": "Go",
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "func Hello() {}" {
		t.Errorf("got %q", out)
	}
}

func TestCodeGenTool_PromptIncludesLanguageAndPrompt(t *testing.T) {
	var capturedMessages []model.Message
	p := &mockProvider{}
	p.response = model.Response{Content: "code"}

	capturingProvider := &capturingMockProvider{
		inner:    p,
		captured: &capturedMessages,
	}
	tool := NewCodeGenTool(capturingProvider)

	_, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{
		"prompt":   "sort a slice",
		"language": "Python",
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(capturedMessages) == 0 {
		t.Fatal("no messages sent to provider")
	}
	content := capturedMessages[0].Content
	if !strings.Contains(content, "Python") {
		t.Errorf("language missing from prompt: %q", content)
	}
	if !strings.Contains(content, "sort a slice") {
		t.Errorf("prompt missing from message: %q", content)
	}
}

func TestCodeGenTool_PropagatesProviderError(t *testing.T) {
	p := &mockProvider{err: context.DeadlineExceeded}
	tool := NewCodeGenTool(p)

	_, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{
		"prompt":   "anything",
		"language": "Go",
	}))
	if err == nil {
		t.Fatal("expected error from provider, got nil")
	}
}

func TestCodeGenTool_NotDestructive(t *testing.T) {
	tool := NewCodeGenTool(&mockProvider{})
	if tool.Destructive() {
		t.Error("generate_code must not be destructive")
	}
}

// capturingMockProvider records messages passed to Chat.
type capturingMockProvider struct {
	inner    model.Provider
	captured *[]model.Message
}

func (c *capturingMockProvider) Chat(ctx context.Context, messages []model.Message, tools []model.ToolDefinition) (model.Response, error) {
	*c.captured = append(*c.captured, messages...)
	return c.inner.Chat(ctx, messages, tools)
}

func (c *capturingMockProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return c.inner.Embed(ctx, text)
}

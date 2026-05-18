package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/xdreamer/xdreamer/internal/memory"
	"github.com/xdreamer/xdreamer/internal/model"
	"github.com/xdreamer/xdreamer/internal/rag"
	"github.com/xdreamer/xdreamer/internal/tools"

	"github.com/xdreamer/xdreamer/internal/config"
)

// --- mocks ---

type seqProvider struct {
	responses []model.Response
	calls     int
	messages  [][]model.Message // captured messages per call
}

func (p *seqProvider) Chat(_ context.Context, msgs []model.Message, _ []model.ToolDefinition) (model.Response, error) {
	if p.calls >= len(p.responses) {
		return model.Response{}, errors.New("no more responses")
	}
	p.messages = append(p.messages, msgs)
	r := p.responses[p.calls]
	p.calls++
	return r, nil
}

func (p *seqProvider) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{1, 0, 0}, nil
}

type mockRetriever struct {
	chunks []rag.Chunk
}

func (m *mockRetriever) Retrieve(_ context.Context, _ string) ([]rag.Chunk, error) {
	return m.chunks, nil
}

type mockTool struct {
	name        string
	destructive bool
	output      string
	execErr     error
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return "mock: " + t.name }
func (t *mockTool) Parameters() map[string]any {
	return map[string]any{"x": map[string]any{"type": "string"}}
}
func (t *mockTool) Destructive() bool { return t.destructive }
func (t *mockTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return t.output, t.execErr
}

func disabledMemory(t *testing.T) *memory.Memory {
	t.Helper()
	m, _ := memory.Load(config.MemoryConfig{Enabled: false})
	return m
}

func baseConfig() RunnerConfig {
	return RunnerConfig{MaxSteps: 10, DryRun: false, WorktreePath: "/worktree"}
}

func newTestRunner(provider model.Provider, registry *tools.Registry, retriever Retriever, confirmer Confirmer) *Runner {
	return NewRunner(baseConfig(), provider, registry, retriever, disabledMemory(nil), confirmer)
}

// --- tests ---

func TestRunner_TwoStepRun(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&mockTool{name: "read_file", output: "content"})

	provider := &seqProvider{responses: []model.Response{
		{Content: "Reading file.", ToolCalls: []model.ToolCall{{ID: "c1", Name: "read_file", Arguments: `{"path":"main.go"}`}}},
		{Content: "Done. DONE", Done: true},
	}}

	runner := NewRunner(baseConfig(), provider, reg, nil, disabledMemory(t), AutoConfirmer{})
	transcript, err := runner.Run(context.Background(), "fix bug")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(transcript.Steps) != 1 {
		t.Errorf("steps: got %d, want 1", len(transcript.Steps))
	}
	if transcript.Steps[0].ToolResults[0].ToolName != "read_file" {
		t.Errorf("tool name: got %q", transcript.Steps[0].ToolResults[0].ToolName)
	}
}

func TestRunner_MaxStepsExceeded(t *testing.T) {
	reg := tools.NewRegistry()
	// Provider never signals Done, always returns empty (no tool calls → "continue" loop)
	provider := &seqProvider{
		responses: make([]model.Response, 20),
	}
	for i := range provider.responses {
		provider.responses[i] = model.Response{Content: "thinking..."}
	}

	cfg := RunnerConfig{MaxSteps: 3, DryRun: false, WorktreePath: "/wt"}
	runner := NewRunner(cfg, provider, reg, nil, disabledMemory(t), AutoConfirmer{})
	_, err := runner.Run(context.Background(), "task")
	if !errors.Is(err, ErrMaxStepsExceeded) {
		t.Errorf("got %v, want ErrMaxStepsExceeded", err)
	}
}

func TestRunner_DryRun_ToolSkipped(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&mockTool{name: "write_file", destructive: true, output: "written"})

	provider := &seqProvider{responses: []model.Response{
		{Content: "Writing.", ToolCalls: []model.ToolCall{{ID: "c1", Name: "write_file", Arguments: `{}`}}},
		{Content: "Done.", Done: true},
	}}

	cfg := RunnerConfig{MaxSteps: 10, DryRun: true, WorktreePath: "/wt"}
	runner := NewRunner(cfg, provider, reg, nil, disabledMemory(t), AutoConfirmer{})
	transcript, err := runner.Run(context.Background(), "task")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(transcript.Steps) == 0 {
		t.Fatal("expected at least one step")
	}
	tr := transcript.Steps[0].ToolResults[0]
	if !tr.Skipped {
		t.Error("tool result should be Skipped=true in dry-run mode")
	}
}

func TestRunner_DestructiveTool_Denied(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&mockTool{name: "delete_file", destructive: true, output: "deleted"})

	provider := &seqProvider{responses: []model.Response{
		{Content: "Deleting.", ToolCalls: []model.ToolCall{{ID: "c1", Name: "delete_file", Arguments: `{}`}}},
		{Content: "Done.", Done: true},
	}}

	// Always deny
	denyConfirmer := &TerminalConfirmer{
		In:  strings.NewReader("n\n"),
		Out: &strings.Builder{},
	}
	runner := NewRunner(baseConfig(), provider, reg, nil, disabledMemory(t), denyConfirmer)
	transcript, err := runner.Run(context.Background(), "task")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	tr := transcript.Steps[0].ToolResults[0]
	if !tr.Skipped {
		t.Error("tool result should be Skipped=true when user denies")
	}
}

func TestRunner_UnknownTool_IsError(t *testing.T) {
	reg := tools.NewRegistry() // empty registry

	provider := &seqProvider{responses: []model.Response{
		{Content: "Using ghost.", ToolCalls: []model.ToolCall{{ID: "c1", Name: "ghost_tool", Arguments: `{}`}}},
		{Content: "Done.", Done: true},
	}}

	runner := NewRunner(baseConfig(), provider, reg, nil, disabledMemory(t), AutoConfirmer{})
	transcript, err := runner.Run(context.Background(), "task")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	tr := transcript.Steps[0].ToolResults[0]
	if !tr.IsError {
		t.Error("unknown tool should produce IsError=true")
	}
}

func TestRunner_RAGContext_InjectedIntoPrompt(t *testing.T) {
	reg := tools.NewRegistry()
	retriever := &mockRetriever{chunks: []rag.Chunk{
		{FilePath: "main.go", StartLine: 1, Content: "package main"},
	}}
	provider := &seqProvider{responses: []model.Response{
		{Content: "Done.", Done: true},
	}}

	runner := NewRunner(baseConfig(), provider, reg, retriever, disabledMemory(t), AutoConfirmer{})
	_, err := runner.Run(context.Background(), "task")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The first Chat call's messages should include a RAG context system message.
	if len(provider.messages) == 0 {
		t.Fatal("no messages captured")
	}
	firstCallMsgs := provider.messages[0]
	found := false
	for _, msg := range firstCallMsgs {
		if msg.Role == model.RoleSystem && strings.Contains(msg.Content, "Relevant context") {
			found = true
			break
		}
	}
	if !found {
		t.Error("RAG context message not found in prompt messages")
	}
}

func TestLastUserMessage(t *testing.T) {
	history := []model.Message{
		{Role: model.RoleSystem, Content: "system"},
		{Role: model.RoleUser, Content: "first"},
		{Role: model.RoleAssistant, Content: "response"},
		{Role: model.RoleUser, Content: "second"},
	}
	if got := lastUserMessage(history); got != "second" {
		t.Errorf("got %q, want %q", got, "second")
	}
}

func TestBuildRAGMessage_Empty(t *testing.T) {
	// appendRAGContext with empty chunks should not append a message.
	history := []model.Message{{Role: model.RoleUser, Content: "task"}}
	result := appendRAGContext(history, nil)
	if len(result) != len(history) {
		t.Errorf("empty chunks should not append a message: got %d messages, want %d", len(result), len(history))
	}
}

func TestBuildRAGMessage_NonEmpty(t *testing.T) {
	chunks := []rag.Chunk{{FilePath: "a.go", StartLine: 5, Content: "func X() {}"}}
	msg := buildRAGMessage(chunks)
	if msg.Role != model.RoleSystem {
		t.Errorf("role: got %q, want system", msg.Role)
	}
	if !strings.Contains(msg.Content, "a.go:5") {
		t.Errorf("chunk location missing: %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "func X()") {
		t.Errorf("chunk content missing: %q", msg.Content)
	}
}

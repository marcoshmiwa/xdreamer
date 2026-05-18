package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/xdreamer/xdreamer/internal/model"
)

// mockTool is a minimal Tool implementation for registry tests.
type mockTool struct {
	name        string
	destructive bool
}

func (m *mockTool) Name() string                                                    { return m.name }
func (m *mockTool) Description() string                                             { return "mock: " + m.name }
func (m *mockTool) Parameters() map[string]any                                      { return map[string]any{"x": map[string]any{"type": "string"}} }
func (m *mockTool) Execute(_ context.Context, _ json.RawMessage) (string, error)    { return "ok", nil }
func (m *mockTool) Destructive() bool                                               { return m.destructive }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	tool := &mockTool{name: "my_tool"}
	r.Register(tool)

	got, ok := r.Get("my_tool")
	if !ok {
		t.Fatal("Get: tool not found after Register")
	}
	if got.Name() != "my_tool" {
		t.Errorf("Name: got %q", got.Name())
	}
}

func TestRegistry_GetMissingReturnsNotFound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get: expected false for unregistered tool")
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "a"})
	r.Register(&mockTool{name: "b"})

	all := r.All()
	if len(all) != 2 {
		t.Errorf("All: got %d tools, want 2", len(all))
	}
}

func TestRegistry_Definitions(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "my_tool"})

	defs := r.Definitions()
	if len(defs) != 1 {
		t.Fatalf("Definitions: got %d, want 1", len(defs))
	}
	d := defs[0]
	if d.Name != "my_tool" {
		t.Errorf("Name: got %q", d.Name)
	}
	if d.Description != "mock: my_tool" {
		t.Errorf("Description: got %q", d.Description)
	}
	if _, ok := d.Parameters["x"]; !ok {
		t.Error("Parameters: missing key 'x'")
	}
}

func TestRegistry_RegisterOverwrites(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "t", destructive: false})
	r.Register(&mockTool{name: "t", destructive: true})

	got, _ := r.Get("t")
	if !got.Destructive() {
		t.Error("second Register should overwrite the first")
	}
}

// Verify mockTool satisfies model.ToolDefinition conversion correctly.
func TestDefinitions_MatchesModelToolDefinition(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "alpha"})

	defs := r.Definitions()
	var _ model.ToolDefinition = defs[0] // compile-time interface check
}

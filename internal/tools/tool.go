package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/xdreamer/xdreamer/internal/model"
)

// ErrToolNotFound is returned when a tool name is not in the registry.
var ErrToolNotFound = errors.New("tool not found")

// Tool is the interface every built-in and future tool must satisfy.
// Implementations must be safe for concurrent use.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any // JSON Schema "properties" object
	Execute(ctx context.Context, params json.RawMessage) (string, error)
	Destructive() bool
}

// Registry holds registered tools indexed by name.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds t to the registry under its name, overwriting any prior entry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get looks up a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// All returns all registered tools in no guaranteed order.
func (r *Registry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// Definitions converts all registered tools to model.ToolDefinition for the LLM.
func (r *Registry) Definitions() []model.ToolDefinition {
	defs := make([]model.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, model.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}

package model

import "context"

// Role identifies the author of a message in the conversation history.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single entry in the conversation history sent to the model.
type Message struct {
	Role       Role
	Content    string
	ToolCallID string     // non-empty when Role == RoleTool
	ToolCalls  []ToolCall // non-empty when Role == RoleAssistant and model requested tools
}

// ToolCall represents a single tool invocation requested by the model.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // raw JSON string
}

// ToolDefinition describes a tool the model may invoke.
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema "properties" object
}

// Response is the model's reply to a Chat call.
type Response struct {
	Content   string
	ToolCalls []ToolCall
	Done      bool // true when the model signals task completion with DONE
}

// Provider is the single abstraction the agent uses to communicate with a language model.
// Implementations must be safe for concurrent use.
type Provider interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Response, error)
	Embed(ctx context.Context, text string) ([]float32, error)
}

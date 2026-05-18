package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xdreamer/xdreamer/internal/model"
)

type codeGenTool struct {
	provider model.Provider
}

// NewCodeGenTool creates a generate_code tool that uses the given provider
// for a single non-tool chat call. Dependency is injected, never global.
func NewCodeGenTool(p model.Provider) Tool {
	return &codeGenTool{provider: p}
}

func (t *codeGenTool) Name() string        { return "generate_code" }
func (t *codeGenTool) Destructive() bool   { return false }
func (t *codeGenTool) Description() string { return "Ask the model to generate a code snippet." }
func (t *codeGenTool) Parameters() map[string]any {
	return map[string]any{
		"prompt":   map[string]any{"type": "string", "description": "Description of the code to generate"},
		"language": map[string]any{"type": "string", "description": "Target programming language"},
	}
}

type codeGenParams struct {
	Prompt   string `json:"prompt"`
	Language string `json:"language"`
}

func (t *codeGenTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p codeGenParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("generate code: parse params: %w", err)
	}
	content := fmt.Sprintf(
		"Write %s code for: %s\nRespond with only the code, no explanation.",
		p.Language, p.Prompt,
	)
	msg := model.Message{Role: model.RoleUser, Content: content}
	resp, err := t.provider.Chat(ctx, []model.Message{msg}, nil)
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return resp.Content, nil
}

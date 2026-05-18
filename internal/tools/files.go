package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// --- read_file ---

type readFileTool struct{}

func NewReadFileTool() Tool { return &readFileTool{} }

func (t *readFileTool) Name() string        { return "read_file" }
func (t *readFileTool) Destructive() bool   { return false }
func (t *readFileTool) Description() string { return "Read and return the full contents of a file." }
func (t *readFileTool) Parameters() map[string]any {
	return map[string]any{
		"path": map[string]any{"type": "string", "description": "File path to read"},
	}
}

type readFileParams struct {
	Path string `json:"path"`
}

func (t *readFileTool) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p readFileParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("read file: parse params: %w", err)
	}
	content, err := os.ReadFile(p.Path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(content), nil
}

// --- write_file ---

type writeFileTool struct{}

func NewWriteFileTool() Tool { return &writeFileTool{} }

func (t *writeFileTool) Name() string        { return "write_file" }
func (t *writeFileTool) Destructive() bool   { return true }
func (t *writeFileTool) Description() string { return "Create or overwrite a file with the given content." }
func (t *writeFileTool) Parameters() map[string]any {
	return map[string]any{
		"path":    map[string]any{"type": "string", "description": "Destination file path"},
		"content": map[string]any{"type": "string", "description": "Content to write"},
	}
}

type writeFileParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *writeFileTool) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p writeFileParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("write file: parse params: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil {
		return "", fmt.Errorf("write file: mkdir: %w", err)
	}
	if err := os.WriteFile(p.Path, []byte(p.Content), 0o644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return "written: " + p.Path, nil
}

// --- delete_file ---

type deleteFileTool struct{}

func NewDeleteFileTool() Tool { return &deleteFileTool{} }

func (t *deleteFileTool) Name() string        { return "delete_file" }
func (t *deleteFileTool) Destructive() bool   { return true }
func (t *deleteFileTool) Description() string { return "Delete a file from the filesystem." }
func (t *deleteFileTool) Parameters() map[string]any {
	return map[string]any{
		"path": map[string]any{"type": "string", "description": "File path to delete"},
	}
}

type deleteFileParams struct {
	Path string `json:"path"`
}

func (t *deleteFileTool) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p deleteFileParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("delete file: parse params: %w", err)
	}
	if err := os.Remove(p.Path); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("delete file: %w", os.ErrNotExist)
		}
		return "", fmt.Errorf("delete file: %w", err)
	}
	return "deleted: " + p.Path, nil
}

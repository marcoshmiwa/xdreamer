package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var skippedDirs = map[string]bool{".git": true, ".xdreamer": true}

type listDirTool struct{}

func NewListDirTool() Tool { return &listDirTool{} }

func (t *listDirTool) Name() string        { return "list_dir" }
func (t *listDirTool) Destructive() bool   { return false }
func (t *listDirTool) Description() string { return "List the contents of a directory." }
func (t *listDirTool) Parameters() map[string]any {
	return map[string]any{
		"path":      map[string]any{"type": "string", "description": "Directory path to list"},
		"recursive": map[string]any{"type": "boolean", "description": "List all files recursively"},
	}
}

type listDirParams struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

func (t *listDirTool) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p listDirParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("list dir: parse params: %w", err)
	}
	if p.Recursive {
		return listRecursive(p.Path)
	}
	return listFlat(p.Path)
}

func listFlat(path string) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("list dir: %w", err)
	}
	var lines []string
	for _, e := range entries {
		if skippedDirs[e.Name()] {
			continue
		}
		kind := "FILE"
		if e.IsDir() {
			kind = "DIR"
		}
		lines = append(lines, fmt.Sprintf("%s (%s)", e.Name(), kind))
	}
	return strings.Join(lines, "\n"), nil
}

func listRecursive(root string) (string, error) {
	var lines []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skippedDirs[d.Name()] {
			return filepath.SkipDir
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		lines = append(lines, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("list dir: %w", err)
	}
	return strings.Join(lines, "\n"), nil
}

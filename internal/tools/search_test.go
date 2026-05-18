package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchCodeTool_FindsMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := NewSearchCodeTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{
		"pattern": "func ",
		"glob":    "**/*.go",
		"root":    dir,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "main.go:3: func Hello()") {
		t.Errorf("expected match, got: %q", out)
	}
}

func TestSearchCodeTool_NoMatchReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644)

	tool := NewSearchCodeTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{
		"pattern": "XYZNOTFOUND",
		"glob":    "**/*.go",
		"root":    dir,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output, got: %q", out)
	}
}

func TestSearchCodeTool_InvalidPatternReturnsError(t *testing.T) {
	dir := t.TempDir()
	tool := NewSearchCodeTool()
	_, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{
		"pattern": "[invalid",
		"glob":    "",
		"root":    dir,
	}))
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestSearchCodeTool_GlobFiltersFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("match me\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("match me\n"), 0o644)

	tool := NewSearchCodeTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{
		"pattern": "match",
		"glob":    "**/*.go",
		"root":    dir,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(out, "notes.txt") {
		t.Errorf("notes.txt should be excluded by glob: %q", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("main.go should be included: %q", out)
	}
}

func TestGlobMatches(t *testing.T) {
	tests := []struct {
		glob    string
		relPath string
		want    bool
	}{
		{"", "any/path.go", true},
		{"**/*.go", "main.go", true},
		{"**/*.go", "internal/foo/bar.go", true},
		{"**/*.go", "internal/foo/bar.txt", false},
		{"*.go", "main.go", true},
		{"*.go", "sub/main.go", false},
		{"internal/*.go", "internal/foo.go", true},
		{"internal/*.go", "other/foo.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.glob+"|"+tt.relPath, func(t *testing.T) {
			if got := globMatches(tt.glob, tt.relPath); got != tt.want {
				t.Errorf("globMatches(%q, %q) = %v, want %v", tt.glob, tt.relPath, got, tt.want)
			}
		})
	}
}

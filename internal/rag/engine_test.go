package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xdreamer/xdreamer/internal/config"
)

func makeRAGConfig(t *testing.T) config.RAGConfig {
	t.Helper()
	return config.RAGConfig{
		Enabled:      true,
		ChunkSize:    20,
		ChunkOverlap: 5,
		TopK:         3,
		IndexDir:     filepath.Join(t.TempDir(), "index"),
		Ignore:       []string{"vendor/", ".git/"},
	}
}

func writeGoFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEngine_IndexAndRetrieve(t *testing.T) {
	requireEncoding(t)

	projectDir := t.TempDir()
	writeGoFile(t, projectDir, "main.go", "package main\n\nfunc main() { println(\"hello\") }\n")
	writeGoFile(t, projectDir, "util.go", "package main\n\nfunc add(a, b int) int { return a + b }\n")
	writeGoFile(t, projectDir, "model.go", "package main\n\ntype User struct { Name string }\n")

	cfg := makeRAGConfig(t)
	engine, err := NewEngine(context.Background(), cfg, fixedEmbedFunc)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer engine.Close()

	if err := engine.Index(context.Background(), projectDir); err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := engine.Retrieve(context.Background(), "hello function")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected results after indexing 3 files, got none")
	}
}

func TestEngine_SkipsIgnoredDirectories(t *testing.T) {
	requireEncoding(t)

	projectDir := t.TempDir()
	writeGoFile(t, projectDir, "real.go", "package main\n")
	// Files inside vendor/ should be ignored.
	writeGoFile(t, projectDir, "vendor/pkg/lib.go", "package lib\n")

	cfg := makeRAGConfig(t)
	engine, err := NewEngine(context.Background(), cfg, fixedEmbedFunc)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer engine.Close()

	if err := engine.Index(context.Background(), projectDir); err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := engine.Retrieve(context.Background(), "lib")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	for _, r := range results {
		if filepath.Base(filepath.Dir(r.FilePath)) == "vendor" {
			t.Errorf("vendor file indexed: %s", r.FilePath)
		}
	}
}

func TestEngine_SkipsBinaryFiles(t *testing.T) {
	requireEncoding(t)

	projectDir := t.TempDir()
	writeGoFile(t, projectDir, "code.go", "package main\n")

	// Write a fake binary file (contains null byte).
	binaryPath := filepath.Join(projectDir, "data.bin")
	if err := os.WriteFile(binaryPath, []byte("hello\x00world"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := makeRAGConfig(t)
	engine, err := NewEngine(context.Background(), cfg, fixedEmbedFunc)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer engine.Close()

	// Index should not error on binary files.
	if err := engine.Index(context.Background(), projectDir); err != nil {
		t.Fatalf("Index: %v", err)
	}
}

func TestEngine_IncrementalIndex_SkipsUnchanged(t *testing.T) {
	requireEncoding(t)

	projectDir := t.TempDir()
	writeGoFile(t, projectDir, "a.go", "package main\n")

	cfg := makeRAGConfig(t)
	embedCallCount := 0
	countingEmbed := func(ctx context.Context, text string) ([]float32, error) {
		embedCallCount++
		return fixedEmbedFunc(ctx, text)
	}

	engine, err := NewEngine(context.Background(), cfg, countingEmbed)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer engine.Close()

	if err := engine.Index(context.Background(), projectDir); err != nil {
		t.Fatalf("first Index: %v", err)
	}
	firstCount := embedCallCount

	// Second index of the same unchanged file should not call embed again.
	if err := engine.Index(context.Background(), projectDir); err != nil {
		t.Fatalf("second Index: %v", err)
	}
	if embedCallCount != firstCount {
		t.Errorf("embed called %d extra times on unchanged file", embedCallCount-firstCount)
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		content []byte
		want    bool
	}{
		{[]byte("hello world"), false},
		{[]byte("hello\x00world"), true},
		{[]byte{0x00}, true},
		{[]byte("package main\n"), false},
	}
	for _, tt := range tests {
		if got := isBinary(tt.content); got != tt.want {
			t.Errorf("isBinary(%q) = %v, want %v", tt.content, got, tt.want)
		}
	}
}

func TestShouldIgnoreDir(t *testing.T) {
	patterns := []string{"vendor/", "node_modules/", ".git/"}
	tests := []struct {
		name string
		want bool
	}{
		{"vendor", true},
		{"node_modules", true},
		{".git", true},
		{"internal", false},
		{"cmd", false},
	}
	for _, tt := range tests {
		if got := shouldIgnoreDir(tt.name, patterns); got != tt.want {
			t.Errorf("shouldIgnoreDir(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

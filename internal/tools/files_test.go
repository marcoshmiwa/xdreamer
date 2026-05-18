package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestReadFileTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := NewReadFileTool()

	t.Run("reads existing file", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{"path": path}))
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out != "hello world" {
			t.Errorf("got %q", out)
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{"path": filepath.Join(dir, "nope.txt")}))
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("not destructive", func(t *testing.T) {
		if tool.Destructive() {
			t.Error("read_file must not be destructive")
		}
	})
}

func TestWriteFileTool(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFileTool()

	t.Run("creates new file", func(t *testing.T) {
		path := filepath.Join(dir, "out.txt")
		out, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{"path": path, "content": "data"}))
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.HasPrefix(out, "written:") {
			t.Errorf("output: got %q", out)
		}
		got, _ := os.ReadFile(path)
		if string(got) != "data" {
			t.Errorf("file content: got %q", got)
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		path := filepath.Join(dir, "a", "b", "c.txt")
		_, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{"path": path, "content": "nested"}))
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("file not created: %v", statErr)
		}
	})

	t.Run("is destructive", func(t *testing.T) {
		if !tool.Destructive() {
			t.Error("write_file must be destructive")
		}
	})
}

func TestDeleteFileTool(t *testing.T) {
	dir := t.TempDir()
	tool := NewDeleteFileTool()

	t.Run("deletes existing file", func(t *testing.T) {
		path := filepath.Join(dir, "del.txt")
		_ = os.WriteFile(path, []byte("x"), 0o644)

		out, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{"path": path}))
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.HasPrefix(out, "deleted:") {
			t.Errorf("output: got %q", out)
		}
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Error("file should have been deleted")
		}
	})

	t.Run("returns ErrNotExist for missing file", func(t *testing.T) {
		_, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{"path": filepath.Join(dir, "ghost.txt")}))
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("got %v, want os.ErrNotExist", err)
		}
	})

	t.Run("is destructive", func(t *testing.T) {
		if !tool.Destructive() {
			t.Error("delete_file must be destructive")
		}
	})
}

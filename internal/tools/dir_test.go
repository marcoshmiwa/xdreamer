package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeTree(t *testing.T, root string, paths []string) {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(root, filepath.FromSlash(p))
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestListDirTool_Flat(t *testing.T) {
	dir := t.TempDir()
	makeTree(t, dir, []string{"a.go", "b.go", "sub/"})

	tool := NewListDirTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"path": dir, "recursive": false}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(out, "a.go (FILE)") {
		t.Errorf("missing a.go: %q", out)
	}
	if !strings.Contains(out, "sub (DIR)") {
		t.Errorf("missing sub dir: %q", out)
	}
}

func TestListDirTool_Recursive(t *testing.T) {
	dir := t.TempDir()
	makeTree(t, dir, []string{"a.go", "sub/b.go"})

	tool := NewListDirTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"path": dir, "recursive": true}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(out, "a.go") {
		t.Errorf("missing a.go: %q", out)
	}
	if !strings.Contains(out, "sub/b.go") {
		t.Errorf("missing sub/b.go: %q", out)
	}
}

func TestListDirTool_SkipsGit(t *testing.T) {
	dir := t.TempDir()
	makeTree(t, dir, []string{".git/", "real.go"})

	tool := NewListDirTool()

	t.Run("flat skips .git", func(t *testing.T) {
		out, _ := tool.Execute(context.Background(), mustJSON(t, map[string]any{"path": dir, "recursive": false}))
		if strings.Contains(out, ".git") {
			t.Errorf(".git should be excluded: %q", out)
		}
	})

	t.Run("recursive skips .git", func(t *testing.T) {
		out, _ := tool.Execute(context.Background(), mustJSON(t, map[string]any{"path": dir, "recursive": true}))
		if strings.Contains(out, ".git") {
			t.Errorf(".git should be excluded: %q", out)
		}
	})
}

func TestListDirTool_NotDestructive(t *testing.T) {
	if NewListDirTool().Destructive() {
		t.Error("list_dir must not be destructive")
	}
}

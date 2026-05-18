package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo creates a temp dir with a git repo and one empty commit.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test User")
	run("commit", "--allow-empty", "-m", "init")
	return dir
}

func TestGitStatusTool(t *testing.T) {
	dir := initGitRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644)

	tool := NewGitStatusTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{"workdir": dir}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "new.txt") {
		t.Errorf("expected new.txt in status, got: %q", out)
	}
}

func TestGitDiffTool_Unstaged(t *testing.T) {
	dir := initGitRepo(t)
	path := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(path, []byte("content"), 0o644)

	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = dir
	_ = cmd.Run()

	commitCmd := exec.Command("git", "commit", "-m", "add file")
	commitCmd.Dir = dir
	_ = commitCmd.Run()

	_ = os.WriteFile(path, []byte("changed"), 0o644)

	tool := NewGitDiffTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"workdir": dir, "staged": false}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "changed") {
		t.Errorf("expected diff output, got: %q", out)
	}
}

func TestGitLogTool(t *testing.T) {
	dir := initGitRepo(t)

	tool := NewGitLogTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"workdir": dir, "n": 5}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "init") {
		t.Errorf("expected 'init' commit in log, got: %q", out)
	}
}

func TestGitLogTool_DefaultN(t *testing.T) {
	dir := initGitRepo(t)
	tool := NewGitLogTool()
	// n=0 should default to 10; just verify it runs without error
	_, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"workdir": dir, "n": 0}))
	if err != nil {
		t.Fatalf("Execute with n=0: %v", err)
	}
}

func TestGitCommitTool(t *testing.T) {
	dir := initGitRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "commit_me.txt"), []byte("data"), 0o644)

	tool := NewGitCommitTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{
		"workdir": dir,
		"message": "test commit",
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "test commit") {
		t.Errorf("expected commit message in output, got: %q", out)
	}
}

func TestGitTools_Destructiveness(t *testing.T) {
	if NewGitStatusTool().Destructive() {
		t.Error("git_status must not be destructive")
	}
	if NewGitDiffTool().Destructive() {
		t.Error("git_diff must not be destructive")
	}
	if NewGitLogTool().Destructive() {
		t.Error("git_log must not be destructive")
	}
	if !NewGitCommitTool().Destructive() {
		t.Error("git_commit must be destructive")
	}
}

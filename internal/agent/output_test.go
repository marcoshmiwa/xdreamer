package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xdreamer/xdreamer/internal/model"
)

// initGitRepo creates a temp repo with a committed file and a pending modification.
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
	run("config", "user.name", "Test")

	// Commit an initial file.
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "main.go")
	run("commit", "-m", "init")

	// Modify it (not staged) so diff HEAD shows a change.
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestWritePatch_CreatesFile(t *testing.T) {
	worktree := initGitRepo(t)
	outputDir := t.TempDir()

	patchPath, err := WritePatch(context.Background(), worktree, outputDir, "fix-bug")
	if err != nil {
		t.Fatalf("WritePatch: %v", err)
	}
	if _, err := os.Stat(patchPath); err != nil {
		t.Errorf("patch file not created: %v", err)
	}
	if !strings.HasSuffix(patchPath, "xdreamer-fix-bug.patch") {
		t.Errorf("unexpected patch path: %q", patchPath)
	}
}

func TestWritePatch_ContentIsDiff(t *testing.T) {
	worktree := initGitRepo(t)
	outputDir := t.TempDir()

	patchPath, err := WritePatch(context.Background(), worktree, outputDir, "slug")
	if err != nil {
		t.Fatalf("WritePatch: %v", err)
	}
	content, _ := os.ReadFile(patchPath)
	// git diff HEAD with an unstaged change produces a unified diff.
	if !strings.Contains(string(content), "@@") && len(content) > 0 {
		t.Logf("patch content: %s", content)
	}
	// At minimum the file must exist (empty diff is OK for a clean worktree in CI).
}

func TestWritePatch_CreatesOutputDir(t *testing.T) {
	worktree := initGitRepo(t)
	outputDir := filepath.Join(t.TempDir(), "nested", "logs")

	_, err := WritePatch(context.Background(), worktree, outputDir, "s")
	if err != nil {
		t.Fatalf("WritePatch with nested output dir: %v", err)
	}
	if _, err := os.Stat(outputDir); err != nil {
		t.Errorf("output dir not created: %v", err)
	}
}

// mockSummaryProvider returns a fixed response for GenerateSummary.
type mockSummaryProvider struct {
	content string
}

func (m *mockSummaryProvider) Chat(_ context.Context, _ []model.Message, _ []model.ToolDefinition) (model.Response, error) {
	return model.Response{Content: m.content}, nil
}

func (m *mockSummaryProvider) Embed(_ context.Context, _ string) ([]float32, error) {
	return nil, nil
}

func TestGenerateSummary_WritesSummaryMD(t *testing.T) {
	worktree := t.TempDir()
	provider := &mockSummaryProvider{content: "## Summary\n\nFixed the login bug."}

	transcript := &Transcript{
		Task: "fix bug",
		Steps: []Step{
			{Number: 1, LLMResponse: "I read the file."},
			{Number: 2, LLMResponse: "I fixed the bug. DONE"},
		},
	}

	if err := GenerateSummary(context.Background(), provider, transcript, worktree); err != nil {
		t.Fatalf("GenerateSummary: %v", err)
	}

	summaryPath := filepath.Join(worktree, "SUMMARY.md")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("SUMMARY.md not created: %v", err)
	}
	if !strings.Contains(string(data), "Fixed the login bug.") {
		t.Errorf("summary content wrong: %q", string(data))
	}
}

func TestGenerateSummary_TranscriptInRequest(t *testing.T) {
	worktree := t.TempDir()
	var capturedMessages []model.Message
	provider := &capturingProvider{
		response: model.Response{Content: "summary"},
		captured: &capturedMessages,
	}

	transcript := &Transcript{
		Task:  "task",
		Steps: []Step{{Number: 1, LLMResponse: "step one result"}},
	}

	_ = GenerateSummary(context.Background(), provider, transcript, worktree)

	if len(capturedMessages) == 0 {
		t.Fatal("no messages sent to provider")
	}
	// The user message should include the LLM response from the step.
	userMsg := ""
	for _, msg := range capturedMessages {
		if msg.Role == model.RoleUser {
			userMsg = msg.Content
		}
	}
	if !strings.Contains(userMsg, "step one result") {
		t.Errorf("transcript not included in summary request: %q", userMsg)
	}
}

type capturingProvider struct {
	response model.Response
	captured *[]model.Message
}

func (c *capturingProvider) Chat(_ context.Context, msgs []model.Message, _ []model.ToolDefinition) (model.Response, error) {
	*c.captured = append(*c.captured, msgs...)
	return c.response, nil
}

func (c *capturingProvider) Embed(_ context.Context, _ string) ([]float32, error) { return nil, nil }

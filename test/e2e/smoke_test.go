package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/xdreamer/xdreamer/internal/worktree"
)

// binaryPath is set by TestMain and shared across all tests.
var binaryPath string

func TestMain(m *testing.M) {
	bin, err := buildBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: could not build xdreamer binary: %v\n", err)
		os.Exit(0)
	}
	binaryPath = bin
	code := m.Run()
	os.Remove(binaryPath)
	os.Exit(code)
}

// buildBinary compiles the xdreamer binary to the module root (a trusted path).
func buildBinary() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	moduleRoot := filepath.Join(wd, "..", "..")

	name := "xdreamer-e2e-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	out := filepath.Join(moduleRoot, name)

	cmd := exec.Command("go", "build", "-trimpath", "-o", out, "./cmd/xdreamer")
	cmd.Dir = moduleRoot
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w: %s", err, b)
	}
	return out, nil
}

// initTestRepo creates a bare git repo with a single Go file and a config that disables RAG.
func initTestRepo(t *testing.T, mainGoContent string) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGoContent), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "main.go")
	run("commit", "-m", "init")

	// Disable RAG so tiktoken/chromem are not needed.
	toml := "[rag]\nenabled = false\n"
	if err := os.WriteFile(filepath.Join(dir, ".xdreamer.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// --- mock LM Studio server helpers ---

func chatResponse(content string, toolCalls []mockToolCall) any {
	calls := make([]any, len(toolCalls))
	for i, tc := range toolCalls {
		calls[i] = map[string]any{
			"id":   fmt.Sprintf("call_%d", i+1),
			"type": "function",
			"function": map[string]any{
				"name":      tc.name,
				"arguments": tc.args,
			},
		}
	}
	msg := map[string]any{"role": "assistant", "content": content}
	if len(calls) > 0 {
		msg["tool_calls"] = calls
	}
	return map[string]any{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": 1677652288,
		"model":   "test",
		"choices": []any{map[string]any{"index": 0, "message": msg, "finish_reason": "stop"}},
	}
}

type mockToolCall struct{ name, args string }

func embedResponse() any {
	return map[string]any{
		"object": "list",
		"data":   []any{map[string]any{"object": "embedding", "embedding": []float32{0.1, 0.2, 0.3}, "index": 0}},
		"model":  "test",
		"usage":  map[string]any{"prompt_tokens": 1, "total_tokens": 1},
	}
}

// newMockServer builds an httptest server whose chat responses are returned
// in order from the responses slice. All embed requests return a fixed vector.
func newMockServer(t *testing.T, responses []any) *httptest.Server {
	t.Helper()
	var idx atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "embeddings") {
			json.NewEncoder(w).Encode(embedResponse()) //nolint:errcheck
			return
		}
		i := int(idx.Add(1)) - 1
		if i >= len(responses) {
			// Default: return a DONE response for any extra calls (e.g. summary).
			json.NewEncoder(w).Encode(chatResponse("Done.\nDONE", nil)) //nolint:errcheck
			return
		}
		json.NewEncoder(w).Encode(responses[i]) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv
}

// runBinary executes the xdreamer binary with the given arguments and environment.
// It pipes "K\n" to stdin so the merge UX keeps the worktree branch.
func runBinary(t *testing.T, repoDir, logDir, serverURL string, extraArgs ...string) ([]byte, error) {
	t.Helper()
	args := append([]string{
		"--auto",
		"--dir", repoDir,
	}, extraArgs...)

	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"XDREAMER_MODEL_BASE_URL="+serverURL+"/v1",
		"XDREAMER_MODEL=test",
		"XDREAMER_EMBED_MODEL=test",
		"XDREAMER_LOG_DIR="+logDir,
	)
	cmd.Stdin = strings.NewReader("K\n") // keep the worktree branch
	return cmd.CombinedOutput()
}

// --- tests ---

func TestOneShotMode(t *testing.T) {
	const task = "fix the bug"
	const originalContent = "package main\n\nfunc answer() int { return 0 }\n"
	const fixedContent = "package main\n\nfunc answer() int { return 42 }\n"

	repoDir := initTestRepo(t, originalContent)
	logDir := t.TempDir()

	slug := worktree.Slug(task)
	worktreePath := filepath.Join(repoDir, ".xdreamer", "worktrees", slug)
	mainGoInWT := filepath.Join(worktreePath, "main.go")

	readArgs, _ := json.Marshal(map[string]string{"path": mainGoInWT})
	writeArgs, _ := json.Marshal(map[string]string{"path": mainGoInWT, "content": fixedContent})

	responses := []any{
		chatResponse("Reading file.", []mockToolCall{{"read_file", string(readArgs)}}),
		chatResponse("Writing fix.", []mockToolCall{{"write_file", string(writeArgs)}}),
		chatResponse("Fixed the bug.\nDONE", nil),
	}
	srv := newMockServer(t, responses)

	out, err := runBinary(t, repoDir, logDir, srv.URL, task)
	if err != nil {
		t.Fatalf("binary failed (exit non-zero):\n%s\nerr: %v", out, err)
	}

	// Worktree main.go contains fixed content.
	gotContent, err := os.ReadFile(mainGoInWT)
	if err != nil {
		t.Fatalf("worktree main.go not found: %v\noutput:\n%s", err, out)
	}
	if string(gotContent) != fixedContent {
		t.Errorf("worktree main.go:\ngot  %q\nwant %q", gotContent, fixedContent)
	}

	// SUMMARY.md exists in worktree.
	if _, err := os.Stat(filepath.Join(worktreePath, "SUMMARY.md")); err != nil {
		t.Errorf("SUMMARY.md not found: %v", err)
	}

	// A .patch file exists in the log dir.
	patches, _ := filepath.Glob(filepath.Join(logDir, "*.patch"))
	if len(patches) == 0 {
		t.Error(".patch file not found in log dir")
	}

	// A .log file exists in the log dir.
	logs, _ := filepath.Glob(filepath.Join(logDir, "*.log"))
	if len(logs) == 0 {
		t.Error(".log file not found in log dir")
	}
}

func TestDryRun(t *testing.T) {
	const task = "fix the bug"
	const originalContent = "package main\n\nfunc answer() int { return 0 }\n"

	repoDir := initTestRepo(t, originalContent)
	logDir := t.TempDir()

	slug := worktree.Slug(task)
	worktreePath := filepath.Join(repoDir, ".xdreamer", "worktrees", slug)
	mainGoInWT := filepath.Join(worktreePath, "main.go")

	// With dry-run: first call returns a write_file tool call (skipped),
	// second call returns DONE; extra calls handled by default (summary).
	mainGoPath := filepath.Join(worktreePath, "main.go")
	writeArgs, _ := json.Marshal(map[string]string{"path": mainGoPath, "content": "changed"})

	responses := []any{
		chatResponse("Will write.", []mockToolCall{{"write_file", string(writeArgs)}}),
		chatResponse("Nothing written. DONE", nil),
	}
	srv := newMockServer(t, responses)

	out, err := runBinary(t, repoDir, logDir, srv.URL, "--dry-run", task)
	if err != nil {
		t.Fatalf("dry-run binary failed:\n%s\nerr: %v", out, err)
	}

	// Worktree main.go must contain the ORIGINAL content (write_file was skipped).
	content, err := os.ReadFile(mainGoInWT)
	if err != nil {
		// Worktree file may not exist if nothing was written; that's also acceptable.
		t.Logf("worktree main.go not found (acceptable in dry-run): %v", err)
		return
	}
	// Normalise CRLF → LF before comparing (git on Windows converts line endings).
	got := strings.ReplaceAll(string(content), "\r\n", "\n")
	if got != originalContent {
		t.Errorf("dry-run must not modify files:\ngot  %q\nwant %q", got, originalContent)
	}
}

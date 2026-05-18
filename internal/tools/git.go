package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const defaultLogN = 10

// runGit executes a git command in workdir and returns trimmed combined output.
func runGit(ctx context.Context, workdir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workdir
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, output)
	}
	return output, nil
}

// --- git_status ---

type gitStatusTool struct{}

func NewGitStatusTool() Tool { return &gitStatusTool{} }

func (t *gitStatusTool) Name() string        { return "git_status" }
func (t *gitStatusTool) Destructive() bool   { return false }
func (t *gitStatusTool) Description() string { return "Show the working tree status (git status --short)." }
func (t *gitStatusTool) Parameters() map[string]any {
	return map[string]any{
		"workdir": map[string]any{"type": "string", "description": "Git repository path"},
	}
}

type gitWorkdirParams struct {
	Workdir string `json:"workdir"`
}

func (t *gitStatusTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p gitWorkdirParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("git status: parse params: %w", err)
	}
	return runGit(ctx, p.Workdir, "status", "--short")
}

// --- git_diff ---

type gitDiffTool struct{}

func NewGitDiffTool() Tool { return &gitDiffTool{} }

func (t *gitDiffTool) Name() string        { return "git_diff" }
func (t *gitDiffTool) Destructive() bool   { return false }
func (t *gitDiffTool) Description() string { return "Show changes between commits or the working tree." }
func (t *gitDiffTool) Parameters() map[string]any {
	return map[string]any{
		"workdir": map[string]any{"type": "string", "description": "Git repository path"},
		"staged":  map[string]any{"type": "boolean", "description": "Show staged changes"},
	}
}

type gitDiffParams struct {
	Workdir string `json:"workdir"`
	Staged  bool   `json:"staged"`
}

func (t *gitDiffTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p gitDiffParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("git diff: parse params: %w", err)
	}
	args := []string{"diff"}
	if p.Staged {
		args = append(args, "--staged")
	}
	return runGit(ctx, p.Workdir, args...)
}

// --- git_log ---

type gitLogTool struct{}

func NewGitLogTool() Tool { return &gitLogTool{} }

func (t *gitLogTool) Name() string        { return "git_log" }
func (t *gitLogTool) Destructive() bool   { return false }
func (t *gitLogTool) Description() string { return "Show recent commit history (git log --oneline)." }
func (t *gitLogTool) Parameters() map[string]any {
	return map[string]any{
		"workdir": map[string]any{"type": "string", "description": "Git repository path"},
		"n":       map[string]any{"type": "integer", "description": "Number of commits to show (default 10)"},
	}
}

type gitLogParams struct {
	Workdir string `json:"workdir"`
	N       int    `json:"n"`
}

func (t *gitLogTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p gitLogParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("git log: parse params: %w", err)
	}
	if p.N <= 0 {
		p.N = defaultLogN
	}
	return runGit(ctx, p.Workdir, "log", "--oneline", fmt.Sprintf("-%d", p.N))
}

// --- git_commit ---

type gitCommitTool struct{}

func NewGitCommitTool() Tool { return &gitCommitTool{} }

func (t *gitCommitTool) Name() string        { return "git_commit" }
func (t *gitCommitTool) Destructive() bool   { return true }
func (t *gitCommitTool) Description() string { return "Stage all changes and create a commit." }
func (t *gitCommitTool) Parameters() map[string]any {
	return map[string]any{
		"workdir": map[string]any{"type": "string", "description": "Git repository path"},
		"message": map[string]any{"type": "string", "description": "Commit message"},
	}
}

type gitCommitParams struct {
	Workdir string `json:"workdir"`
	Message string `json:"message"`
}

func (t *gitCommitTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p gitCommitParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("git commit: parse params: %w", err)
	}
	if _, err := runGit(ctx, p.Workdir, "add", "-A"); err != nil {
		return "", err
	}
	return runGit(ctx, p.Workdir, "commit", "-m", p.Message)
}

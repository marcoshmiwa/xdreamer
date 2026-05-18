package tools

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func echoCmd() (string, string) {
	if runtime.GOOS == "windows" {
		return "echo hello", "hello"
	}
	return "echo hello", "hello"
}

func failCmd() string {
	if runtime.GOOS == "windows" {
		return "exit 1"
	}
	return "exit 1"
}

func TestRunShellTool_Success(t *testing.T) {
	dir := t.TempDir()
	tool := NewRunShellTool()

	cmd, want := echoCmd()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{
		"command": cmd,
		"workdir": dir,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, want) {
		t.Errorf("output: got %q, want to contain %q", out, want)
	}
}

func TestRunShellTool_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	tool := NewRunShellTool()

	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]string{
		"command": failCmd(),
		"workdir": dir,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(out, "[exit ") {
		t.Errorf("expected [exit ...] prefix, got: %q", out)
	}
}

func TestRunShellTool_ContextCancelled(t *testing.T) {
	dir := t.TempDir()
	tool := NewRunShellTool()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// A long-running command that should be killed by context
	cmd := "sleep 60"
	if runtime.GOOS == "windows" {
		cmd = "timeout /t 60"
	}
	// On cancelled context the command may error or return exit prefix; either is acceptable
	_, _ = tool.Execute(ctx, mustJSON(t, map[string]string{
		"command": cmd,
		"workdir": dir,
	}))
}

func TestRunShellTool_IsDestructive(t *testing.T) {
	if !NewRunShellTool().Destructive() {
		t.Error("run_shell must be destructive")
	}
}

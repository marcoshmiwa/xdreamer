package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

const (
	unixShell = "sh"
	unixFlag  = "-c"
	winShell  = "cmd"
	winFlag   = "/C"
)

type runShellTool struct{}

func NewRunShellTool() Tool { return &runShellTool{} }

func (t *runShellTool) Name() string        { return "run_shell" }
func (t *runShellTool) Destructive() bool   { return true }
func (t *runShellTool) Description() string { return "Execute a shell command in a given working directory." }
func (t *runShellTool) Parameters() map[string]any {
	return map[string]any{
		"command": map[string]any{"type": "string", "description": "Shell command to execute"},
		"workdir": map[string]any{"type": "string", "description": "Working directory for the command"},
	}
}

type shellParams struct {
	Command string `json:"command"`
	Workdir string `json:"workdir"`
}

func (t *runShellTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p shellParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("run shell: parse params: %w", err)
	}
	return runCommand(ctx, p.Command, p.Workdir)
}

func runCommand(ctx context.Context, command, workdir string) (string, error) {
	shell, flag := interpreter()
	cmd := exec.CommandContext(ctx, shell, flag, command)
	cmd.Dir = workdir
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		exitCode := exitCodeFrom(err)
		if exitCode != 0 {
			return fmt.Sprintf("[exit %d]\n%s", exitCode, output), nil
		}
		return "", fmt.Errorf("run shell: %w", err)
	}
	return output, nil
}

func interpreter() (shell, flag string) {
	if runtime.GOOS == "windows" {
		return winShell, winFlag
	}
	return unixShell, unixFlag
}

func exitCodeFrom(err error) int {
	var exitErr *exec.ExitError
	if ok := isExitError(err, &exitErr); ok {
		return exitErr.ExitCode()
	}
	return 0
}

func isExitError(err error, out **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*out = e
		return true
	}
	return false
}

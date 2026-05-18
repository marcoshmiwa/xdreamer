package agent

import (
	"strings"
	"testing"
)

func TestRenderSystemPrompt_InjectsWorktreePath(t *testing.T) {
	prompt, err := renderSystemPrompt("/workspace/task-slug", "no memory")
	if err != nil {
		t.Fatalf("renderSystemPrompt: %v", err)
	}
	if !strings.Contains(prompt, "/workspace/task-slug") {
		t.Errorf("worktree path missing from prompt:\n%s", prompt)
	}
}

func TestRenderSystemPrompt_InjectsMemoryContent(t *testing.T) {
	memory := "## Architecture\nEntry point is cmd/xdreamer/main.go"
	prompt, err := renderSystemPrompt("/path", memory)
	if err != nil {
		t.Fatalf("renderSystemPrompt: %v", err)
	}
	if !strings.Contains(prompt, memory) {
		t.Errorf("memory content missing from prompt:\n%s", prompt)
	}
}

func TestRenderSystemPrompt_ContainsDONEInstruction(t *testing.T) {
	prompt, err := renderSystemPrompt("/p", "")
	if err != nil {
		t.Fatalf("renderSystemPrompt: %v", err)
	}
	if !strings.Contains(prompt, "DONE") {
		t.Errorf("DONE instruction missing from prompt:\n%s", prompt)
	}
}

func TestRenderSystemPrompt_ContainsRules(t *testing.T) {
	prompt, err := renderSystemPrompt("/p", "")
	if err != nil {
		t.Fatalf("renderSystemPrompt: %v", err)
	}
	for _, rule := range []string{"read_file", "write_file", "one tool per turn"} {
		if !strings.Contains(prompt, rule) {
			t.Errorf("rule %q missing from prompt", rule)
		}
	}
}

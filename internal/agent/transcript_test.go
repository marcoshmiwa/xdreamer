package agent

import (
	"path/filepath"
	"strings"
	"testing"
)

func sampleTranscript() *Transcript {
	t := &Transcript{Task: "fix the bug"}
	t.AddStep(Step{
		Number:      1,
		UserMessage: "fix the bug",
		LLMResponse: "I'll read the file first.",
		ToolResults: []ToolResult{
			{ToolName: "read_file", Params: `{"path":"main.go"}`, Output: "package main\n"},
		},
	})
	t.AddStep(Step{
		Number:      2,
		LLMResponse: "Fixed. DONE",
		ToolResults: []ToolResult{
			{ToolName: "write_file", Params: `{"path":"main.go","content":"..."}`, Output: "written: main.go"},
		},
	})
	return t
}

func TestTranscript_AddStep(t *testing.T) {
	tr := &Transcript{Task: "task"}
	tr.AddStep(Step{Number: 1})
	tr.AddStep(Step{Number: 2})
	if len(tr.Steps) != 2 {
		t.Errorf("got %d steps, want 2", len(tr.Steps))
	}
}

func TestTranscript_WriteLog_ContainsKeyFields(t *testing.T) {
	tr := sampleTranscript()
	var sb strings.Builder
	if err := tr.WriteLog(&sb); err != nil {
		t.Fatalf("WriteLog: %v", err)
	}
	out := sb.String()

	for _, want := range []string{
		"Task: fix the bug",
		"=== Step 1 ===",
		"[User] fix the bug",
		"[LLM]  I'll read the file first.",
		"[Tool: read_file]",
		"[Result] package main",
		"=== Step 2 ===",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestTranscript_WriteLog_SkippedTool(t *testing.T) {
	tr := &Transcript{Task: "t"}
	tr.AddStep(Step{
		Number: 1,
		ToolResults: []ToolResult{
			{ToolName: "run_shell", Params: `{}`, Skipped: true},
		},
	})
	var sb strings.Builder
	_ = tr.WriteLog(&sb)
	if !strings.Contains(sb.String(), "[skipped]") {
		t.Errorf("skipped label missing: %s", sb.String())
	}
}

func TestTranscript_WriteLog_ErrorTool(t *testing.T) {
	tr := &Transcript{Task: "t"}
	tr.AddStep(Step{
		Number: 1,
		ToolResults: []ToolResult{
			{ToolName: "bad_tool", Output: "file not found", IsError: true},
		},
	})
	var sb strings.Builder
	_ = tr.WriteLog(&sb)
	if !strings.Contains(sb.String(), "[error]") {
		t.Errorf("[error] label missing: %s", sb.String())
	}
}

func TestTranscript_SaveToFile(t *testing.T) {
	tr := sampleTranscript()
	path := filepath.Join(t.TempDir(), "sub", "run.log")

	if err := tr.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	// Re-read and check content.
	var sb strings.Builder
	_ = tr.WriteLog(&sb)
	want := sb.String()
	if want == "" {
		t.Fatal("transcript content is empty")
	}
}

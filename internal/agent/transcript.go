package agent

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ToolResult records the outcome of a single tool call within a step.
type ToolResult struct {
	ToolName string
	Params   string
	Output   string
	IsError  bool
	Skipped  bool // true when dry-run or user denied
}

// Step is one turn of the agent loop: one LLM response and its tool calls.
type Step struct {
	Number      int
	UserMessage string
	LLMResponse string
	ToolResults []ToolResult
}

// Transcript is the full record of a Run() execution.
type Transcript struct {
	Task  string
	Steps []Step
}

// AddStep appends a completed step to the transcript.
func (t *Transcript) AddStep(s Step) {
	t.Steps = append(t.Steps, s)
}

// WriteLog writes a human-readable log of the transcript to w.
func (t *Transcript) WriteLog(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "Task: %s\n\n", t.Task); err != nil {
		return err
	}
	for _, step := range t.Steps {
		if err := writeStep(w, step); err != nil {
			return err
		}
	}
	return nil
}

func writeStep(w io.Writer, step Step) error {
	if _, err := fmt.Fprintf(w, "=== Step %d ===\n", step.Number); err != nil {
		return err
	}
	if step.UserMessage != "" {
		if _, err := fmt.Fprintf(w, "[User] %s\n", step.UserMessage); err != nil {
			return err
		}
	}
	if step.LLMResponse != "" {
		if _, err := fmt.Fprintf(w, "[LLM]  %s\n", step.LLMResponse); err != nil {
			return err
		}
	}
	for _, tr := range step.ToolResults {
		if err := writeToolResult(w, tr); err != nil {
			return err
		}
	}
	return nil
}

func writeToolResult(w io.Writer, tr ToolResult) error {
	label := fmt.Sprintf("[Tool: %s] params: %s", tr.ToolName, tr.Params)
	if tr.Skipped {
		label += " [skipped]"
	}
	if _, err := fmt.Fprintln(w, label); err != nil {
		return err
	}
	result := tr.Output
	if tr.IsError {
		result = "[error] " + result
	}
	if result != "" {
		_, err := fmt.Fprintf(w, "[Result] %s\n", strings.TrimSpace(result))
		return err
	}
	return nil
}

// SaveToFile writes the transcript to a file, creating parent directories as needed.
func (t *Transcript) SaveToFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("save transcript: mkdir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("save transcript: create: %w", err)
	}
	defer f.Close()
	if err := t.WriteLog(f); err != nil {
		return fmt.Errorf("save transcript: write: %w", err)
	}
	return nil
}

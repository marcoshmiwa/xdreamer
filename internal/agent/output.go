package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xdreamer/xdreamer/internal/model"
)

// WritePatch runs git diff HEAD in worktreePath and writes the result to
// <outputDir>/xdreamer-<slug>.patch. Returns the patch file path.
func WritePatch(ctx context.Context, worktreePath, outputDir, slug string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "diff", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("write patch: git diff: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("write patch: mkdir: %w", err)
	}
	patchPath := filepath.Join(outputDir, "xdreamer-"+slug+".patch")
	if err := os.WriteFile(patchPath, out, 0o644); err != nil {
		return "", fmt.Errorf("write patch: write: %w", err)
	}
	return patchPath, nil
}

// GenerateSummary asks the model to summarise the run, then writes the result
// to <worktreePath>/SUMMARY.md.
func GenerateSummary(ctx context.Context, provider model.Provider, transcript *Transcript, worktreePath string) error {
	messages := buildSummaryMessages(transcript)
	resp, err := provider.Chat(ctx, messages, nil)
	if err != nil {
		return fmt.Errorf("generate summary: chat: %w", err)
	}
	summaryPath := filepath.Join(worktreePath, "SUMMARY.md")
	if err := os.WriteFile(summaryPath, []byte(resp.Content), 0o644); err != nil {
		return fmt.Errorf("generate summary: write: %w", err)
	}
	return nil
}

func buildSummaryMessages(transcript *Transcript) []model.Message {
	var log strings.Builder
	for _, step := range transcript.Steps {
		if step.LLMResponse != "" {
			log.WriteString(step.LLMResponse)
			log.WriteString("\n\n")
		}
	}
	return []model.Message{
		{
			Role:    model.RoleSystem,
			Content: "You are summarising a coding session. Be concise and use markdown.",
		},
		{
			Role: model.RoleUser,
			Content: "Summarize what you did during this coding session in markdown format. Be concise.\n\nSession log:\n" +
				log.String(),
		},
	}
}

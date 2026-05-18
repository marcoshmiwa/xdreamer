package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// Confirmer decides whether a destructive tool action may proceed.
type Confirmer interface {
	Confirm(ctx context.Context, toolName, description string) (bool, error)
}

// TerminalConfirmer prompts the user on stderr and reads their answer from stdin.
type TerminalConfirmer struct {
	In  io.Reader
	Out io.Writer
}

// AutoConfirmer approves every action without prompting. Used with --auto flag.
type AutoConfirmer struct{}

// Confirm asks the user whether the destructive action may proceed.
func (c *TerminalConfirmer) Confirm(_ context.Context, toolName, description string) (bool, error) {
	if _, err := fmt.Fprintf(c.Out, "[destructive] %s: %s\nProceed? (y/N): ", toolName, description); err != nil {
		return false, fmt.Errorf("confirm: write prompt: %w", err)
	}
	scanner := bufio.NewScanner(c.In)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}

// Confirm always returns true, allowing every action.
func (AutoConfirmer) Confirm(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

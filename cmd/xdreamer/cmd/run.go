package cmd

import (
	"context"
	"fmt"

	"github.com/xdreamer/xdreamer/internal/config"
)

// runOneShotMode executes a single task given as a positional argument and exits.
func runOneShotMode(ctx context.Context, cfg *config.Config, task string) error {
	s, err := newSession(ctx, cfg)
	if err != nil {
		return fmt.Errorf("one-shot: %w", err)
	}
	defer s.close() //nolint:errcheck

	_, err = runTask(ctx, s, task, flagWorktree, flagDryRun, flagAuto)
	return err
}

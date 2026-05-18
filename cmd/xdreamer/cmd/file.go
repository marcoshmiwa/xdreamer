package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/xdreamer/xdreamer/internal/config"
)

// runFileMode reads a task from the file named by --file and executes it.
func runFileMode(ctx context.Context, cfg *config.Config) error {
	raw, err := os.ReadFile(flagFile)
	if err != nil {
		return fmt.Errorf("read task file: %w", err)
	}
	task := string(raw)

	s, err := newSession(ctx, cfg)
	if err != nil {
		return fmt.Errorf("file mode: %w", err)
	}
	defer s.close() //nolint:errcheck

	_, err = runTask(ctx, s, task, flagWorktree, flagDryRun, flagAuto)
	return err
}

package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/xdreamer/xdreamer/internal/config"
)

var errExit = errors.New("exit")

const replBanner = "xdreamer — type your task, or /exit to quit"

// runREPL starts an interactive session where each input line is a task.
func runREPL(ctx context.Context, cfg *config.Config) error {
	fmt.Println(replBanner)

	s, err := newSession(ctx, cfg)
	if err != nil {
		return fmt.Errorf("repl: %w", err)
	}
	defer s.close() //nolint:errcheck

	var lastWorktreePath string
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			if err := handleReplCommand(ctx, line, s, &lastWorktreePath); errors.Is(err, errExit) {
				break
			}
			continue
		}

		wtPath, err := runTask(ctx, s, line, "", flagDryRun, flagAuto)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		} else {
			lastWorktreePath = wtPath
		}
		fmt.Println("\n--- ready ---")
	}

	_ = s.close()
	fmt.Println("bye")
	return scanner.Err()
}

// handleReplCommand processes a slash command and updates state as needed.
func handleReplCommand(ctx context.Context, line string, s *session, lastWtPath *string) error {
	cmd := strings.ToLower(strings.TrimSpace(line))
	switch cmd {
	case "/exit":
		return errExit
	case "/memory":
		printMemory(s)
	case "/status":
		printWorktreeStatus(ctx, s.cfg.Dir)
	case "/undo":
		undoLastCommit(ctx, *lastWtPath)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", line)
	}
	return nil
}

func printMemory(s *session) {
	content := s.memory.Content()
	if content == "" {
		fmt.Println("memory disabled")
		return
	}
	fmt.Println(content)
}

func printWorktreeStatus(ctx context.Context, repoRoot string) {
	out, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "list").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git worktree list: %v\n", err)
		return
	}
	fmt.Print(string(out))
}

func undoLastCommit(ctx context.Context, wtPath string) {
	if wtPath == "" {
		fmt.Fprintln(os.Stderr, "no worktree from a previous task")
		return
	}
	out, err := exec.CommandContext(ctx, "git", "-C", wtPath, "reset", "--soft", "HEAD~1").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "undo: %v: %s\n", err, out)
		return
	}
	fmt.Println("undone last commit in", wtPath)
}

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xdreamer/xdreamer/internal/agent"
	"github.com/xdreamer/xdreamer/internal/config"
	"github.com/xdreamer/xdreamer/internal/memory"
	"github.com/xdreamer/xdreamer/internal/model"
	"github.com/xdreamer/xdreamer/internal/rag"
	"github.com/xdreamer/xdreamer/internal/tools"
	"github.com/xdreamer/xdreamer/internal/worktree"
)

// session holds the long-lived dependencies created once per CLI invocation.
type session struct {
	cfg       *config.Config
	provider  model.Provider
	registry  *tools.Registry
	ragEngine *rag.Engine
	memory    *memory.Memory
}

// newSession wires all dependencies from the loaded config.
// RAG indexing is performed if cfg.RAG.Enabled is true.
func newSession(ctx context.Context, cfg *config.Config) (*session, error) {
	provider := model.NewLMStudio(
		cfg.Model.BaseURL,
		cfg.Model.Model,
		cfg.Model.EmbedModel,
		cfg.Model.Temperature,
		cfg.Model.MaxRetries,
		cfg.Model.RetryDelayMS,
	)

	var ragEngine *rag.Engine
	if cfg.RAG.Enabled {
		eng, err := rag.NewEngine(ctx, cfg.RAG, provider.Embed)
		if err != nil {
			return nil, fmt.Errorf("new session: rag: %w", err)
		}
		if err := eng.Index(ctx, cfg.Dir); err != nil {
			return nil, fmt.Errorf("new session: rag index: %w", err)
		}
		ragEngine = eng
	}

	mem, err := memory.Load(cfg.Memory)
	if err != nil {
		return nil, fmt.Errorf("new session: memory: %w", err)
	}

	registry := buildRegistry(provider)

	return &session{
		cfg:       cfg,
		provider:  provider,
		registry:  registry,
		ragEngine: ragEngine,
		memory:    mem,
	}, nil
}

// buildRegistry creates the tool registry with all built-in tools registered.
func buildRegistry(provider model.Provider) *tools.Registry {
	r := tools.NewRegistry()
	r.Register(tools.NewReadFileTool())
	r.Register(tools.NewWriteFileTool())
	r.Register(tools.NewDeleteFileTool())
	r.Register(tools.NewListDirTool())
	r.Register(tools.NewSearchCodeTool())
	r.Register(tools.NewRunShellTool())
	r.Register(tools.NewGitStatusTool())
	r.Register(tools.NewGitDiffTool())
	r.Register(tools.NewGitLogTool())
	r.Register(tools.NewGitCommitTool())
	r.Register(tools.NewWebFetchTool())
	r.Register(tools.NewWebSearchTool())
	r.Register(tools.NewCodeGenTool(provider))
	return r
}

// close releases resources held by the session.
func (s *session) close() error {
	if s.ragEngine != nil {
		return s.ragEngine.Close()
	}
	return nil
}

// runTask executes the full agent loop for a single task, then handles the
// merge/keep/discard prompt. Returns the worktree path (if kept for /undo).
func runTask(ctx context.Context, s *session, task, worktreeName string, dryRun, auto bool) (wtPath string, err error) {
	cfg := s.cfg
	slug := worktree.Slug(task)
	if worktreeName != "" {
		slug = worktreeName
	}

	wt, err := worktree.Create(cfg.Dir, cfg.Agent.WorktreeBase, slug)
	if err != nil {
		return "", fmt.Errorf("create worktree: %w", err)
	}

	var confirmer agent.Confirmer
	if auto || cfg.Agent.Auto {
		confirmer = agent.AutoConfirmer{}
	} else {
		confirmer = &agent.TerminalConfirmer{In: os.Stdin, Out: os.Stderr}
	}

	// Resolve the Retriever interface: nil when RAG is disabled.
	var retriever agent.Retriever
	if s.ragEngine != nil {
		retriever = s.ragEngine
	}

	runner := agent.NewRunner(
		agent.RunnerConfig{
			MaxSteps:     cfg.Agent.MaxSteps,
			DryRun:       dryRun,
			WorktreePath: wt.Path,
		},
		s.provider,
		s.registry,
		retriever,
		s.memory,
		confirmer,
	)

	transcript, err := runner.Run(ctx, task)
	if err != nil {
		return "", fmt.Errorf("run: %w", err)
	}

	if err := agent.GenerateSummary(ctx, s.provider, transcript, wt.Path); err != nil {
		fmt.Fprintf(os.Stderr, "warning: generate summary: %v\n", err)
	}

	commitWorktree(ctx, wt.Path, task)

	patchPath, err := agent.WritePatch(ctx, wt.Path, cfg.Output.LogDir, slug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: write patch: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Patch: %s\n", patchPath)
	}

	logPath := filepath.Join(cfg.Output.LogDir, "xdreamer-"+slug+".log")
	if err := transcript.SaveToFile(logPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: save log: %v\n", err)
	}

	printDiff(ctx, wt.Path)
	fmt.Fprintf(os.Stdout, "Branch: %s\n", wt.Branch)

	return wt.Path, promptMergeUX(ctx, cfg.Dir, wt)
}

// commitWorktree stages and commits all changes made by the agent.
func commitWorktree(ctx context.Context, wtPath, task string) {
	msg := "xdreamer: " + task
	_ = exec.CommandContext(ctx, "git", "-C", wtPath, "add", "-A").Run()
	_ = exec.CommandContext(ctx, "git", "-C", wtPath, "commit", "-m", msg).Run()
}

// printDiff prints git diff HEAD of the worktree to stdout.
func printDiff(ctx context.Context, wtPath string) {
	out, err := exec.CommandContext(ctx, "git", "-C", wtPath, "diff", "HEAD").Output()
	if err == nil && len(out) > 0 {
		fmt.Println(string(out))
	}
}

// promptMergeUX asks the user what to do with the finished worktree branch.
func promptMergeUX(ctx context.Context, repoRoot string, wt *worktree.Worktree) error {
	fmt.Fprint(os.Stdout, "[M]erge / [K]eep branch / [D]iscard? ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return scanner.Err()
	}
	answer := strings.ToUpper(strings.TrimSpace(scanner.Text()))
	switch answer {
	case "M":
		out, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "merge", wt.Branch).CombinedOutput()
		if err != nil {
			return fmt.Errorf("merge: %w: %s", err, out)
		}
		fmt.Printf("Merged: %s\n", wt.Branch)
	case "D":
		if err := wt.Remove(); err != nil {
			return fmt.Errorf("discard: %w", err)
		}
		fmt.Printf("Discarded: %s\n", wt.Branch)
	default: // "K" or anything else → keep
		fmt.Printf("Branch kept: %s\n", wt.Branch)
	}
	return nil
}

package worktree

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrNotGitRepo is returned when the given directory is not a git repository.
var ErrNotGitRepo = errors.New("directory is not a git repository")

const (
	branchPrefix  = "xdreamer/"
	maxSlugLength = 40
)

var (
	slugIllegal   = regexp.MustCompile(`[^a-z0-9-]+`)
	slugMultiDash = regexp.MustCompile(`-{2,}`)
)

// Worktree represents a git worktree created for a single agent task.
type Worktree struct {
	Branch string
	Path   string
	Root   string
}

// Slug converts a task string into a safe git branch name segment.
// Result is lowercase, contains only [a-z0-9-], with no leading/trailing dashes,
// and is at most 40 characters.
func Slug(task string) string {
	s := slugIllegal.ReplaceAllString(strings.ToLower(task), "-")
	s = slugMultiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > maxSlugLength {
		s = strings.TrimRight(s[:maxSlugLength], "-")
	}
	return s
}

// Create makes a new git worktree for the given slug under <root>/<base>/<slug>.
// Returns ErrNotGitRepo if root is not inside a git repository.
func Create(root, base, slug string) (*Worktree, error) {
	if err := verifyGitRepo(root); err != nil {
		return nil, ErrNotGitRepo
	}

	branch := branchPrefix + slug
	path := filepath.Join(root, base, slug)

	if err := addWorktree(root, branch, path); err != nil {
		return nil, fmt.Errorf("create worktree: %w", err)
	}

	return &Worktree{Branch: branch, Path: path, Root: root}, nil
}

// Remove deletes the worktree directory and its associated branch.
func (w *Worktree) Remove() error {
	if err := gitRun(w.Root, "worktree", "remove", "--force", w.Path); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	if err := gitRun(w.Root, "branch", "-D", w.Branch); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}
	return nil
}

// verifyGitRepo checks that dir is inside a git repository.
func verifyGitRepo(dir string) error {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	return cmd.Run()
}

// addWorktree tries to create a new worktree with a new branch.
// If the branch already exists, it attaches the worktree to the existing branch.
func addWorktree(root, branch, path string) error {
	if err := gitRun(root, "worktree", "add", "-b", branch, path); err == nil {
		return nil
	}
	// Branch may already exist; try attaching without creating a new one.
	return gitRun(root, "worktree", "add", path, branch)
}

// gitRun executes a git command in the given directory and returns any error.
func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

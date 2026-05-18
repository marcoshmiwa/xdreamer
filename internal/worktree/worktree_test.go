package worktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo creates a temp git repository with a single empty commit.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test User")
	run("commit", "--allow-empty", "-m", "init")
	return dir
}

// --- Slug tests ---

func TestSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"fix login bug", "fix-login-bug"},
		{"Fix Login Bug!", "fix-login-bug"},
		{"  leading and trailing  ", "leading-and-trailing"},
		{"hello--world", "hello-world"},
		{"hello---world", "hello-world"},
		{"123 numeric", "123-numeric"},
		{"already-valid", "already-valid"},
		{"UPPERCASE", "uppercase"},
		{"special!@#$chars", "special-chars"},
		// long string: result must be ≤40 chars with no trailing dash
		{strings.Repeat("a", 50), strings.Repeat("a", 40)},
		{"implement the new feature for user authentication", "implement-the-new-feature-for-user-authe"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slug(tt.input)
			if got != tt.want {
				t.Errorf("Slug(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if len(got) > maxSlugLength {
				t.Errorf("Slug too long: %d > %d", len(got), maxSlugLength)
			}
			if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
				t.Errorf("Slug has leading/trailing dash: %q", got)
			}
		})
	}
}

func TestSlug_NoTrailingDashAfterTruncation(t *testing.T) {
	// Build a string that ends with a dash right at the 40-char boundary.
	// e.g. 39 'a's + '-' + more chars → after truncation to 40, trailing dash trimmed.
	input := strings.Repeat("a", 39) + "-extra-stuff"
	got := Slug(input)
	if strings.HasSuffix(got, "-") {
		t.Errorf("trailing dash after truncation: %q", got)
	}
	if len(got) > maxSlugLength {
		t.Errorf("slug too long after truncation: %d", len(got))
	}
}

// --- Create / Remove tests ---

func TestCreate_RoundTrip(t *testing.T) {
	root := initRepo(t)
	base := ".xdreamer/worktrees"
	slug := "test-task"

	wt, err := Create(root, base, slug)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify the struct fields.
	if wt.Branch != "xdreamer/test-task" {
		t.Errorf("Branch: got %q, want %q", wt.Branch, "xdreamer/test-task")
	}
	wantPath := filepath.Join(root, base, slug)
	if wt.Path != wantPath {
		t.Errorf("Path: got %q, want %q", wt.Path, wantPath)
	}
	if wt.Root != root {
		t.Errorf("Root: got %q, want %q", wt.Root, root)
	}

	// Verify the worktree directory was created on disk.
	if _, err := os.Stat(wt.Path); err != nil {
		t.Errorf("worktree path not on disk: %v", err)
	}

	// Remove and verify cleanup.
	if err := wt.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Error("worktree path still exists after Remove")
	}
}

func TestCreate_ExistingBranchAttaches(t *testing.T) {
	root := initRepo(t)

	// Create the worktree the first time.
	wt, err := Create(root, ".xdreamer/worktrees", "existing")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	// Remove worktree but keep branch — simulates a detached worktree state.
	if err := gitRun(root, "worktree", "remove", "--force", wt.Path); err != nil {
		t.Fatalf("remove worktree dir: %v", err)
	}

	// Second Create with same slug should attach to the existing branch.
	wt2, err := Create(root, ".xdreamer/worktrees", "existing")
	if err != nil {
		t.Fatalf("second Create (attach): %v", err)
	}
	if _, statErr := os.Stat(wt2.Path); statErr != nil {
		t.Errorf("worktree path not created on second Create: %v", statErr)
	}

	// Cleanup.
	_ = wt2.Remove()
}

func TestCreate_NonGitDirReturnsErrNotGitRepo(t *testing.T) {
	dir := t.TempDir() // plain directory, not a git repo

	_, err := Create(dir, "worktrees", "task")
	if !errors.Is(err, ErrNotGitRepo) {
		t.Errorf("got %v, want ErrNotGitRepo", err)
	}
}

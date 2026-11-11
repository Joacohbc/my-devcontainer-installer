package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsAvailable(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git is not available in PATH")
	}
}

func TestInitAndIsRepo(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git is not available")
	}

	dir := t.TempDir()

	if IsRepo(dir) {
		t.Fatalf("expected empty temp dir not to be a git repo")
	}

	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !IsRepo(dir) {
		t.Fatalf("expected initialized dir to be a git repo")
	}
}

func TestInitInvalidDir(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")
	if err := Init(nonExistent); err == nil {
		t.Fatalf("expected error initializing non-existent dir, got nil")
	}
}

func TestCloneInvalidURL(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git is not available")
	}

	target := filepath.Join(t.TempDir(), "target")
	err := Clone("https://invalid.example.com/non-existent-repo.git", target)
	if err == nil {
		t.Fatalf("expected error cloning invalid repo, got nil")
	}
}

func TestCloneLocalRepo(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git is not available")
	}

	src := t.TempDir()
	if err := Init(src); err != nil {
		t.Fatalf("Init src failed: %v", err)
	}

	dummyFile := filepath.Join(src, "README.md")
	if err := os.WriteFile(dummyFile, []byte("# Test"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	target := filepath.Join(t.TempDir(), "cloned")
	if err := Clone(src, target); err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	if !IsRepo(target) {
		t.Fatalf("expected cloned directory to be a git repo")
	}
}

// isolateGitConfig keeps the developer's global git config (identity, hooks,
// signing) out of the test so commit behaviour is deterministic.
func isolateGitConfig(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestIsRepoRoot(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git is not available")
	}
	isolateGitConfig(t)

	parent := t.TempDir()
	if err := Init(parent); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(parent, "profiles")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	if !IsRepoRoot(parent) {
		t.Error("expected the initialised directory to be a repository root")
	}
	if IsRepoRoot(child) {
		t.Error("a subdirectory of a repository must not count as its own root")
	}
	if IsRepoRoot(t.TempDir()) {
		t.Error("a plain directory must not count as a repository root")
	}
}

func TestCommitPathsWithoutIdentityUsesFallback(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git is not available")
	}
	isolateGitConfig(t)

	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	committed, err := CommitPaths(dir, "feat: add a", "a.txt")
	if err != nil {
		t.Fatalf("CommitPaths failed: %v", err)
	}
	if !committed {
		t.Fatal("expected a commit to be created")
	}
	if got := gitOutput(t, dir, "log", "-1", "--format=%s|%ae"); got != "feat: add a|"+fallbackEmail {
		t.Errorf("unexpected last commit %q", got)
	}
}

func TestCommitPathsKeepsConfiguredIdentity(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git is not available")
	}
	isolateGitConfig(t)

	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, dir, "config", "user.name", "Someone")
	gitOutput(t, dir, "config", "user.email", "someone@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := CommitPaths(dir, "feat: add a", "a.txt"); err != nil {
		t.Fatal(err)
	}
	if got := gitOutput(t, dir, "log", "-1", "--format=%ae"); got != "someone@example.com" {
		t.Errorf("expected the configured identity to be kept, got %q", got)
	}
}

// Only the named paths are committed: unrelated edits in the same repository
// stay out of the operation's commit.
func TestCommitPathsLeavesOtherChangesUncommitted(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git is not available")
	}
	isolateGitConfig(t)

	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "profile.yml"), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := CommitPaths(dir, "feat: add a", "a"); err != nil {
		t.Fatal(err)
	}
	if got := gitOutput(t, dir, "show", "--name-only", "--format=", "HEAD"); got != "a/profile.yml" {
		t.Errorf("expected only a/ in the commit, got %q", got)
	}
	if got := gitOutput(t, dir, "status", "--porcelain"); got != "?? b/" {
		t.Errorf("expected b/ to stay untracked, got %q", got)
	}
}

func TestCommitPathsRecordsDeletion(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git is not available")
	}
	isolateGitConfig(t)

	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "a")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "profile.yml"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitPaths(dir, "feat: add a", "a"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}

	committed, err := CommitPaths(dir, "chore: remove a", "a")
	if err != nil {
		t.Fatalf("CommitPaths failed: %v", err)
	}
	if !committed {
		t.Fatal("expected the deletion to be committed")
	}
	if got := gitOutput(t, dir, "status", "--porcelain"); got != "" {
		t.Errorf("expected a clean tree, got %q", got)
	}
}

func TestCommitPathsWithNothingToCommit(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git is not available")
	}
	isolateGitConfig(t)

	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}

	committed, err := CommitPaths(dir, "chore: nothing", ".")
	if err != nil {
		t.Fatalf("expected no error on an empty change, got %v", err)
	}
	if committed {
		t.Error("expected no commit when nothing changed")
	}
}

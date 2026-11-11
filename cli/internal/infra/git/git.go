// Package git runs the git binary. It is the only place in the tree that does,
// mirroring how infra/docker owns every shell-out to docker.
package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsAvailable reports whether a git binary is on PATH.
func IsAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// IsRepo reports whether dir is inside a git working tree. It asks git rather
// than looking for a .git entry, so a worktree or a subdirectory of a repo is
// recognised as one instead of being offered a second `git init`.
func IsRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// Init creates a repository in dir.
func Init(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("cannot initialise a repository in %s: %w", dir, err)
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init in %s failed: %w: %s", filepath.Clean(dir), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// IsRepoRoot reports whether dir is the top level of its own working tree. A
// directory nested inside some other repository (a versioned ~/.config, say) is
// not, so a caller that wants a repository of its own can tell the two apart.
func IsRepoRoot(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return samePath(strings.TrimSpace(string(out)), dir)
}

func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// The identity used for a commit only when the user has none configured, so
// that an automatic commit never fails on a fresh machine and never overrides
// an identity the user did set.
const (
	fallbackName  = "devcontainer-cli"
	fallbackEmail = "devcontainer-cli@localhost"
)

// CommitPaths stages and commits only the given paths (relative to dir),
// leaving any other change in the repository untouched. It reports whether a
// commit was created: nothing to commit is not an error.
func CommitPaths(dir, message string, paths ...string) (bool, error) {
	if err := run(dir, append([]string{"add", "-A", "--"}, paths...)...); err != nil {
		return false, err
	}
	if !hasStagedChanges(dir, paths) {
		return false, nil
	}
	args := identityArgs(dir)
	args = append(args, "commit", "-q", "-m", message, "--")
	args = append(args, paths...)
	if err := run(dir, args...); err != nil {
		return false, err
	}
	return true, nil
}

func hasStagedChanges(dir string, paths []string) bool {
	cmd := exec.Command("git", append([]string{"diff", "--cached", "--quiet", "--"}, paths...)...)
	cmd.Dir = dir
	return cmd.Run() != nil
}

func identityArgs(dir string) []string {
	cmd := exec.Command("git", "config", "user.email")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		return nil
	}
	return []string{"-c", "user.name=" + fallbackName, "-c", "user.email=" + fallbackEmail}
}

func run(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s in %s failed: %w: %s", strings.Join(args, " "), filepath.Clean(dir), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Clone clones a git repository from repoURL into targetDir.
func Clone(repoURL, targetDir string) error {
	cmd := exec.Command("git", "clone", repoURL, targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone %s into %s failed: %w: %s", repoURL, filepath.Clean(targetDir), err, strings.TrimSpace(string(out)))
	}
	return nil
}

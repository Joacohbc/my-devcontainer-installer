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

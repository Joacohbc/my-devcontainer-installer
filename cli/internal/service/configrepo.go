package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/git"
)

const initConfigRepoMessage = "chore(config): initialize configuration repository"

// ConfigRepoGitignore is an allowlist: only the shareable definitions are
// versioned. Everything else in the config dir (config.json, images.json,
// router_state.json and above all ssh/, which holds private keys) stays out
// even if a user runs `git add -A` by hand.
const ConfigRepoGitignore = `# Managed by devcontainer-cli: only shareable definitions are versioned.
/*
!/.gitignore
!/profiles/
!/images/
!/skills/
`

// ConfigVCS is the version-control side of the configuration repository. It is
// an interface so callers can be exercised without a git binary.
type ConfigVCS interface {
	Available() bool
	IsRepoRoot(dir string) bool
	Init(dir string) error
	CommitPaths(dir, message string, paths ...string) (bool, error)
}

type gitConfigVCS struct{}

func (gitConfigVCS) Available() bool            { return git.IsAvailable() }
func (gitConfigVCS) IsRepoRoot(dir string) bool { return git.IsRepoRoot(dir) }
func (gitConfigVCS) Init(dir string) error      { return git.Init(dir) }
func (gitConfigVCS) CommitPaths(dir, message string, paths ...string) (bool, error) {
	return git.CommitPaths(dir, message, paths...)
}

// ConfigRepo is the single git repository rooted at the global config dir that
// versions profiles/, images/ and skills/ together, so the whole kit can be
// cloned onto another machine. Every CLI operation that changes a definition
// records one path-scoped commit. Git trouble is only ever a warning: the
// definition files were written either way.
type ConfigRepo struct {
	Report Reporter
	VCS    ConfigVCS
	Dir    string
}

// NewConfigRepo returns the repository for the real global config dir.
func NewConfigRepo(report Reporter) ConfigRepo {
	return ConfigRepo{Report: report, VCS: gitConfigVCS{}, Dir: domain.GlobalConfigDir()}
}

// Prepare makes Dir the root of its own repository (initialising it with the
// managed .gitignore and committing what is already there) and reports whether
// the caller can commit afterwards. Call it before writing, so the operation's
// own commit holds only its change.
func (c ConfigRepo) Prepare() bool {
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		c.Report.Warn("could not create %s: %v", c.Dir, err)
		return false
	}
	if !c.VCS.Available() {
		c.Report.Warn("git is not installed; changes in %s will not be versioned.", c.Dir)
		return false
	}
	if c.VCS.IsRepoRoot(c.Dir) {
		return true
	}
	if err := c.VCS.Init(c.Dir); err != nil {
		c.Report.Warn("could not initialise a git repository in %s: %v", c.Dir, err)
		return false
	}
	if err := c.writeGitignore(); err != nil {
		c.Report.Warn("could not write %s: %v", filepath.Join(c.Dir, ".gitignore"), err)
		return false
	}
	if _, err := c.VCS.CommitPaths(c.Dir, initConfigRepoMessage, "."); err != nil {
		c.Report.Warn("could not record the initial configuration commit: %v", err)
	}
	return true
}

func (c ConfigRepo) writeGitignore() error {
	path := filepath.Join(c.Dir, ".gitignore")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, []byte(ConfigRepoGitignore), 0o644)
}

// Commit records one commit holding only the given paths (absolute, or
// relative to Dir). Failures are reported as warnings.
func (c ConfigRepo) Commit(message string, paths ...string) {
	rel := make([]string, 0, len(paths))
	for _, p := range paths {
		r, err := c.relative(p)
		if err != nil {
			c.Report.Warn("not committing %s: %v", p, err)
			return
		}
		rel = append(rel, r)
	}
	if _, err := c.VCS.CommitPaths(c.Dir, message, rel...); err != nil {
		c.Report.Warn("files were written but the git commit failed: %v", err)
	}
}

func (c ConfigRepo) relative(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	rel, err := filepath.Rel(c.Dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the configuration repository %s", path, c.Dir)
	}
	return rel, nil
}

// silentReporter drops every message. It stands in for a missing Reporter so a
// service built without one can still record commits.
type silentReporter struct{}

func (silentReporter) Info(string, ...any)    {}
func (silentReporter) Warn(string, ...any)    {}
func (silentReporter) Success(string, ...any) {}
func (silentReporter) Error(string, ...any)   {}
func (silentReporter) Fatal(string, ...any)   {}
func (silentReporter) Debug(string, ...any)   {}

func reporterOrSilent(r Reporter) Reporter {
	if r == nil {
		return silentReporter{}
	}
	return r
}

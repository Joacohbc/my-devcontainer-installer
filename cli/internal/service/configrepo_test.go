package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fakeVCS struct {
	unavailable bool
	root        bool
	commitErr   error
	inits       int
	commits     []string
}

func (f *fakeVCS) Available() bool        { return !f.unavailable }
func (f *fakeVCS) IsRepoRoot(string) bool { return f.root }
func (f *fakeVCS) Init(string) error      { f.inits++; f.root = true; return nil }
func (f *fakeVCS) CommitPaths(_, msg string, paths ...string) (bool, error) {
	if f.commitErr != nil {
		return false, f.commitErr
	}
	f.commits = append(f.commits, msg+" | "+strings.Join(paths, ","))
	return true, nil
}

type warnRecorder struct {
	nopReporter
	warns []string
}

func (w *warnRecorder) Warn(format string, args ...any) {
	w.warns = append(w.warns, fmt.Sprintf(format, args...))
}

func newTestRepo(t *testing.T, vcs ConfigVCS) (ConfigRepo, *warnRecorder) {
	t.Helper()
	rec := &warnRecorder{}
	return ConfigRepo{Report: rec, VCS: vcs, Dir: filepath.Join(t.TempDir(), "devcontainer-cli")}, rec
}

func isolateGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func TestConfigRepoPrepareInitialisesWithAllowlist(t *testing.T) {
	vcs := &fakeVCS{}
	repo, rec := newTestRepo(t, vcs)

	if !repo.Prepare() {
		t.Fatal("expected Prepare to succeed")
	}
	if vcs.inits != 1 {
		t.Errorf("expected one init, got %d", vcs.inits)
	}
	data, err := os.ReadFile(filepath.Join(repo.Dir, ".gitignore"))
	if err != nil || string(data) != ConfigRepoGitignore {
		t.Errorf("expected the managed .gitignore, got %q (%v)", data, err)
	}
	if len(vcs.commits) != 1 || vcs.commits[0] != initConfigRepoMessage+" | ." {
		t.Errorf("unexpected commits: %v", vcs.commits)
	}
	if len(rec.warns) != 0 {
		t.Errorf("unexpected warnings: %v", rec.warns)
	}
}

func TestConfigRepoPrepareKeepsUserGitignore(t *testing.T) {
	repo, _ := newTestRepo(t, &fakeVCS{})
	if err := os.MkdirAll(repo.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "/*\n!/profiles/\n"
	if err := os.WriteFile(filepath.Join(repo.Dir, ".gitignore"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Prepare()
	if data, _ := os.ReadFile(filepath.Join(repo.Dir, ".gitignore")); string(data) != custom {
		t.Errorf("an existing .gitignore must be left alone, got %q", data)
	}
}

func TestConfigRepoPrepareSkipsInitWhenAlreadyRoot(t *testing.T) {
	vcs := &fakeVCS{root: true}
	repo, _ := newTestRepo(t, vcs)
	if !repo.Prepare() {
		t.Fatal("expected Prepare to succeed")
	}
	if vcs.inits != 0 || len(vcs.commits) != 0 {
		t.Errorf("expected no init and no commit, got inits=%d commits=%v", vcs.inits, vcs.commits)
	}
}

func TestConfigRepoWithoutGitOnlyWarns(t *testing.T) {
	vcs := &fakeVCS{unavailable: true}
	repo, rec := newTestRepo(t, vcs)
	if repo.Prepare() {
		t.Fatal("expected Prepare to report git as unusable")
	}
	if len(rec.warns) != 1 || vcs.inits != 0 {
		t.Errorf("expected one warning and no init, warns=%v inits=%d", rec.warns, vcs.inits)
	}
}

func TestConfigRepoCommitUsesPathsRelativeToRepo(t *testing.T) {
	vcs := &fakeVCS{root: true}
	repo, _ := newTestRepo(t, vcs)
	repo.Commit("feat(profile): add a", filepath.Join(repo.Dir, "profiles", "a"), "images/b")
	if len(vcs.commits) != 1 || vcs.commits[0] != "feat(profile): add a | profiles/a,images/b" {
		t.Errorf("unexpected commits: %v", vcs.commits)
	}
}

func TestConfigRepoCommitRefusesPathsOutsideRepo(t *testing.T) {
	vcs := &fakeVCS{root: true}
	repo, rec := newTestRepo(t, vcs)
	repo.Commit("m", filepath.Join(filepath.Dir(repo.Dir), "elsewhere"))
	if len(vcs.commits) != 0 || len(rec.warns) != 1 {
		t.Errorf("expected a warning and no commit, commits=%v warns=%v", vcs.commits, rec.warns)
	}
}

func TestConfigRepoCommitErrorIsOnlyAWarning(t *testing.T) {
	repo, rec := newTestRepo(t, &fakeVCS{root: true, commitErr: fmt.Errorf("boom")})
	repo.Commit("m", "profiles/a")
	if len(rec.warns) != 1 || !strings.Contains(rec.warns[0], "boom") {
		t.Errorf("expected a warning with the cause, got %v", rec.warns)
	}
}

// Secrets and machine state live next to the definitions; the allowlist must
// keep every one of them out of the repository.
func TestConfigRepoNeverVersionsSecretsOrState(t *testing.T) {
	isolateGit(t)
	repo, rec := newTestRepo(t, gitConfigVCS{})
	for path, content := range map[string]string{
		"config.json":            "{}",
		"images.json":            "{}",
		"router_state.json":      "{}",
		"ssh/id_ed25519":         "PRIVATE",
		"profiles/a/profile.yml": "id: a\n",
		"skills/x.yml":           "id: x\n",
	} {
		full := filepath.Join(repo.Dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if !repo.Prepare() {
		t.Fatalf("Prepare failed: %v", rec.warns)
	}
	out, err := exec.Command("git", "-C", repo.Dir, "ls-files").Output()
	if err != nil {
		t.Fatal(err)
	}
	tracked := strings.Fields(string(out))
	want := []string{".gitignore", "profiles/a/profile.yml", "skills/x.yml"}
	if strings.Join(tracked, " ") != strings.Join(want, " ") {
		t.Errorf("tracked files = %v, want %v", tracked, want)
	}
}

// A versioned ~/.config (dotfiles) must not receive the CLI's commits: the
// config dir gets a repository of its own.
func TestConfigRepoNestedInsideAnotherRepoGetsItsOwn(t *testing.T) {
	isolateGit(t)
	outer := t.TempDir()
	if out, err := exec.Command("git", "-C", outer, "init").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	rec := &warnRecorder{}
	repo := ConfigRepo{Report: rec, VCS: gitConfigVCS{}, Dir: filepath.Join(outer, "devcontainer-cli")}
	if !repo.Prepare() {
		t.Fatalf("Prepare failed: %v", rec.warns)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".git")); err != nil {
		t.Errorf("expected a nested repository at %s: %v", repo.Dir, err)
	}
	out, _ := exec.Command("git", "-C", outer, "log", "--oneline").CombinedOutput()
	if strings.Contains(string(out), "initialize configuration repository") {
		t.Errorf("the outer repository must not receive the commit: %s", out)
	}
}

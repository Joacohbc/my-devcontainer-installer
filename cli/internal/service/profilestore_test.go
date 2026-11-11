package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func newTestStore(t *testing.T, vcs ConfigVCS) (ProfileStore, *warnRecorder) {
	t.Helper()
	repo, rec := newTestRepo(t, vcs)
	return ProfileStore{Report: rec, Repo: repo, Dir: filepath.Join(repo.Dir, "profiles")}, rec
}

func TestProfileStoreSaveInitialisesRepoAndCommits(t *testing.T) {
	vcs := &fakeVCS{}
	store, rec := newTestStore(t, vcs)

	path, err := store.Save(catalog.Profile{ID: "mine", Modules: []string{"go"}}, "feat(profile): add mine")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(store.Dir, "mine", "profile.yml"); path != want {
		t.Errorf("expected %s, got %s", want, path)
	}
	want := []string{initConfigRepoMessage + " | .", "feat(profile): add mine | profiles/mine"}
	if strings.Join(vcs.commits, "\n") != strings.Join(want, "\n") {
		t.Errorf("unexpected commits: %v", vcs.commits)
	}
	if len(rec.warns) != 0 {
		t.Errorf("unexpected warnings: %v", rec.warns)
	}
}

func TestProfileStoreSaveCopiesScripts(t *testing.T) {
	store, _ := newTestStore(t, &fakeVCS{root: true})
	src := filepath.Join(t.TempDir(), "setup.sh")
	if err := os.WriteFile(src, []byte("echo hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := catalog.Profile{ID: "withscript", Scripts: []types.CustomScript{{File: "setup.sh", Source: src}}}
	if _, err := store.Save(p, "m"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(store.Dir, "withscript", "setup.sh"))
	if err != nil || string(data) != "echo hi\n" {
		t.Errorf("script not copied: %q, %v", data, err)
	}
	manifest, err := os.ReadFile(filepath.Join(store.Dir, "withscript", "profile.yml"))
	if err != nil || !strings.Contains(string(manifest), "setup.sh") {
		t.Errorf("manifest missing script: %q, %v", manifest, err)
	}
}

func TestProfileStoreSaveWithoutGitStillWrites(t *testing.T) {
	vcs := &fakeVCS{unavailable: true}
	store, rec := newTestStore(t, vcs)
	path, err := store.Save(catalog.Profile{ID: "mine"}, "m")
	if err != nil {
		t.Fatal(err)
	}
	if _, serr := os.Stat(path); serr != nil {
		t.Errorf("profile not written: %v", serr)
	}
	if len(rec.warns) != 1 || len(vcs.commits) != 0 {
		t.Errorf("expected one warning and no commit, warns=%v commits=%v", rec.warns, vcs.commits)
	}
}

func TestProfileStoreCommitErrorIsOnlyAWarning(t *testing.T) {
	store, rec := newTestStore(t, &fakeVCS{root: true, commitErr: fmt.Errorf("boom")})
	path, err := store.Save(catalog.Profile{ID: "mine"}, "m")
	if err != nil {
		t.Fatalf("commit failure must not fail Save: %v", err)
	}
	if _, serr := os.Stat(path); serr != nil {
		t.Errorf("profile not written: %v", serr)
	}
	if len(rec.warns) != 1 || !strings.Contains(rec.warns[0], "boom") {
		t.Errorf("expected a warning with the cause, got %v", rec.warns)
	}
}

func TestProfileStoreRemoveCommitsOnce(t *testing.T) {
	vcs := &fakeVCS{root: true}
	store, _ := newTestStore(t, vcs)
	for _, id := range []string{"a", "b"} {
		if _, err := store.Save(catalog.Profile{ID: id}, "add"); err != nil {
			t.Fatal(err)
		}
	}
	vcs.commits = nil
	if err := store.Remove([]string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if _, err := os.Stat(filepath.Join(store.Dir, id)); !os.IsNotExist(err) {
			t.Errorf("profile %s still exists", id)
		}
	}
	if len(vcs.commits) != 1 || vcs.commits[0] != "chore(profile): remove a, b | profiles/a,profiles/b" {
		t.Errorf("unexpected commits: %v", vcs.commits)
	}
}

func TestProfileStoreRemoveRejectsBuiltinAndUnknownBeforeDeleting(t *testing.T) {
	vcs := &fakeVCS{root: true}
	store, _ := newTestStore(t, vcs)
	if _, err := store.Save(catalog.Profile{ID: "keep"}, "add"); err != nil {
		t.Fatal(err)
	}
	vcs.commits = nil

	err := store.Remove([]string{"keep", "nodejs"})
	if err == nil || !strings.Contains(err.Error(), "built-in") {
		t.Errorf("expected built-in error, got %v", err)
	}
	err = store.Remove([]string{"keep", "ghost"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
	if _, serr := os.Stat(filepath.Join(store.Dir, "keep")); serr != nil {
		t.Errorf("valid profile must survive a failed validation: %v", serr)
	}
	if len(vcs.commits) != 0 {
		t.Errorf("no commit expected, got %v", vcs.commits)
	}
	if err := store.Locate([]string{"keep"}); err != nil {
		t.Errorf("Locate of a user profile: %v", err)
	}
}

func TestProfileStoreLegacyWarnings(t *testing.T) {
	store, rec := newTestStore(t, &fakeVCS{})
	if err := os.MkdirAll(store.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir, "old.yml"), []byte("id: old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(filepath.Dir(store.Dir), "presets"), 0o755); err != nil {
		t.Fatal(err)
	}
	store.LegacyWarnings()
	if len(rec.warns) != 2 {
		t.Fatalf("expected 2 warnings, got %v", rec.warns)
	}
	if !strings.Contains(rec.warns[0], "old.yml") || !strings.Contains(rec.warns[1], "presets") {
		t.Errorf("unexpected warnings: %v", rec.warns)
	}
}

func TestProfileStoreWithRealGit(t *testing.T) {
	isolateGit(t)
	store, rec := newTestStore(t, gitConfigVCS{})

	if _, err := store.Save(catalog.Profile{ID: "one"}, "feat(profile): add one"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(catalog.Profile{ID: "two"}, "feat(profile): add two"); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove([]string{"one"}); err != nil {
		t.Fatal(err)
	}
	if len(rec.warns) != 0 {
		t.Fatalf("unexpected warnings: %v", rec.warns)
	}
	out, err := exec.Command("git", "-C", store.Repo.Dir, "log", "--format=%s").Output()
	if err != nil {
		t.Fatal(err)
	}
	want := "chore(profile): remove one\nfeat(profile): add two\nfeat(profile): add one\n" + initConfigRepoMessage
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("unexpected git log:\n%s\nwant:\n%s", got, want)
	}
}

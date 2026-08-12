package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

// setGitInit points the global config at a temp dir and sets the opt-in.
func setGitInit(t *testing.T, enabled bool) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := domain.LoadGlobalConfig()
	cfg.GitInit = enabled
	if err := domain.SaveGlobalConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

// The directory is the user's own, so nothing happens until they opt in.
func TestEnsureRepoDoesNothingWhenDisabled(t *testing.T) {
	setGitInit(t, false)
	dir := t.TempDir()

	created, err := GitRepoService{Report: nopReporter{}}.EnsureRepo(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected no repository without the opt-in")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Errorf("expected no .git, stat err = %v", err)
	}
}

// With the opt-in given and no terminal to ask at, the answer is already known.
func TestEnsureRepoInitialisesWhenEnabled(t *testing.T) {
	setGitInit(t, true)
	dir := t.TempDir()

	created, err := GitRepoService{Report: nopReporter{}}.EnsureRepo(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Fatal("expected a repository to be created")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Errorf("expected .git to exist: %v", err)
	}
}

// An existing repository is left alone, and a second run is a no-op.
func TestEnsureRepoLeavesAnExistingRepoAlone(t *testing.T) {
	setGitInit(t, true)
	dir := t.TempDir()

	svc := GitRepoService{Report: nopReporter{}}
	if _, err := svc.EnsureRepo(dir, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	created, err := svc.EnsureRepo(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected the second run to be a no-op")
	}
}

// Declining the prompt must leave the directory untouched.
func TestEnsureRepoRespectsADeclinedPrompt(t *testing.T) {
	setGitInit(t, true)
	dir := t.TempDir()

	created, err := GitRepoService{
		Report: nopReporter{},
		Prompt: &confirmingPrompter{answer: false},
	}.EnsureRepo(dir, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected no repository when the user declines")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Errorf("expected no .git, stat err = %v", err)
	}
}

func TestEnsureRepoInitialisesWhenTheUserAccepts(t *testing.T) {
	setGitInit(t, true)
	dir := t.TempDir()

	created, err := GitRepoService{Report: nopReporter{}, Prompt: &confirmingPrompter{answer: true}}.EnsureRepo(dir, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Fatal("expected a repository to be created")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Errorf("expected .git to exist: %v", err)
	}
}

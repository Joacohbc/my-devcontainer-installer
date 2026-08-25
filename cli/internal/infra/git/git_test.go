package git

import (
	"os"
	"path/filepath"
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

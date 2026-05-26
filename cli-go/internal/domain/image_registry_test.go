package domain_test

import (
	"os"
	"testing"

	"github.com/adrg/xdg"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/domain"
)

func withTempRegistryDir(t *testing.T, fn func()) {
	t.Helper()
	tmp, err := os.MkdirTemp("", "dc-cli-registry-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })

	prev, hasPrev := os.LookupEnv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmp)
	xdg.Reload()
	t.Cleanup(func() {
		if hasPrev {
			os.Setenv("XDG_CONFIG_HOME", prev)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		xdg.Reload()
	})

	fn()
}

func TestComputeFingerprint_sameContentProducesSameResult(t *testing.T) {
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04\nRUN apt-get update", nil, []string{"base"})
	fp2 := domain.ComputeFingerprint("FROM ubuntu:24.04\nRUN apt-get update", nil, []string{"base"})
	if fp1 != fp2 {
		t.Errorf("expected identical fingerprints, got %q and %q", fp1, fp2)
	}
}

func TestComputeFingerprint_labelChangeDoesNotAffectResult(t *testing.T) {
	withoutLabel := domain.ComputeFingerprint("FROM ubuntu:24.04\nRUN apt-get update", nil, nil)
	withLabel := domain.ComputeFingerprint("FROM ubuntu:24.04\nRUN apt-get update\nLABEL foo=\"bar\"", nil, nil)
	if withoutLabel != withLabel {
		t.Errorf("LABEL change should not affect fingerprint: %q vs %q", withoutLabel, withLabel)
	}
}

func TestComputeFingerprint_multilineLabelStripped(t *testing.T) {
	withoutLabel := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, nil)
	withMultilineLabel := domain.ComputeFingerprint("FROM ubuntu:24.04\nLABEL foo=\"bar\" \\\n      baz=\"qux\"", nil, nil)
	if withoutLabel != withMultilineLabel {
		t.Errorf("multiline LABEL should be stripped: %q vs %q", withoutLabel, withMultilineLabel)
	}
}

func TestComputeFingerprint_differentDockerfileProducesDifferentResult(t *testing.T) {
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, nil)
	fp2 := domain.ComputeFingerprint("FROM ubuntu:22.04", nil, nil)
	if fp1 == fp2 {
		t.Error("different Dockerfiles should produce different fingerprints")
	}
}

func TestComputeFingerprint_differentCopyFilesProducesDifferentResult(t *testing.T) {
	files1 := map[string]string{"script.sh": "echo hello"}
	files2 := map[string]string{"script.sh": "echo world"}
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04", files1, nil)
	fp2 := domain.ComputeFingerprint("FROM ubuntu:24.04", files2, nil)
	if fp1 == fp2 {
		t.Error("different copy file contents should produce different fingerprints")
	}
}

func TestComputeFingerprint_differentModuleIDsProducesDifferentResult(t *testing.T) {
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, []string{"base", "nodejs"})
	fp2 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, []string{"base", "python"})
	if fp1 == fp2 {
		t.Error("different module IDs should produce different fingerprints")
	}
}

func TestComputeFingerprint_moduleIDOrderDoesNotMatter(t *testing.T) {
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, []string{"nodejs", "base"})
	fp2 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, []string{"base", "nodejs"})
	if fp1 != fp2 {
		t.Errorf("module ID order should not affect fingerprint: %q vs %q", fp1, fp2)
	}
}

func TestFingerprintTag_usesFirst12Chars(t *testing.T) {
	fp := "abcdef123456789012"
	tag := domain.FingerprintTag(fp)
	expected := "devcontainer-cli/abcdef123456:latest"
	if tag != expected {
		t.Errorf("expected %q, got %q", expected, tag)
	}
}

func TestFingerprintTag_shortFingerprintUsedWhole(t *testing.T) {
	fp := "abc"
	tag := domain.FingerprintTag(fp)
	expected := "devcontainer-cli/abc:latest"
	if tag != expected {
		t.Errorf("expected %q, got %q", expected, tag)
	}
}

func TestRecordEntry_addsNewEntry(t *testing.T) {
	withTempRegistryDir(t, func() {
		entry := domain.ImageEntry{
			ProjectDir:  "/home/user/myproject",
			Workspace:   "myproject",
			Mode:        "local-cached",
			Image:       "devcontainer-cli/abc123:latest",
			Fingerprint: "abc123",
			CreatedAt:   "2024-01-01T00:00:00Z",
			LastUpdated: "2024-01-01T00:00:00Z",
		}
		if err := domain.RecordEntry(entry); err != nil {
			t.Fatalf("RecordEntry failed: %v", err)
		}

		entries := domain.ListEntries()
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].ProjectDir != entry.ProjectDir {
			t.Errorf("expected project dir %q, got %q", entry.ProjectDir, entries[0].ProjectDir)
		}
	})
}

func TestRecordEntry_updatesExistingEntryByProjectDir(t *testing.T) {
	withTempRegistryDir(t, func() {
		first := domain.ImageEntry{
			ProjectDir:  "/home/user/myproject",
			Workspace:   "myproject",
			Mode:        "local-cached",
			Image:       "devcontainer-cli/aaa:latest",
			Fingerprint: "aaa",
			CreatedAt:   "2024-01-01T00:00:00Z",
			LastUpdated: "2024-01-01T00:00:00Z",
		}
		if err := domain.RecordEntry(first); err != nil {
			t.Fatalf("first RecordEntry failed: %v", err)
		}

		updated := first
		updated.Image = "devcontainer-cli/bbb:latest"
		updated.Fingerprint = "bbb"
		updated.LastUpdated = "2024-02-01T00:00:00Z"
		if err := domain.RecordEntry(updated); err != nil {
			t.Fatalf("second RecordEntry failed: %v", err)
		}

		entries := domain.ListEntries()
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry after update, got %d", len(entries))
		}
		if entries[0].Image != updated.Image {
			t.Errorf("expected updated image %q, got %q", updated.Image, entries[0].Image)
		}
	})
}

func TestFindByFingerprint_returnsMatchingEntry(t *testing.T) {
	withTempRegistryDir(t, func() {
		entry := domain.ImageEntry{
			ProjectDir:  "/home/user/proj",
			Workspace:   "proj",
			Mode:        "local-cached",
			Image:       "devcontainer-cli/fp12345:latest",
			Fingerprint: "fp12345",
			CreatedAt:   "2024-01-01T00:00:00Z",
			LastUpdated: "2024-01-01T00:00:00Z",
		}
		_ = domain.RecordEntry(entry)

		found := domain.FindByFingerprint("fp12345")
		if found == nil {
			t.Fatal("expected to find entry by fingerprint, got nil")
		}
		if found.ProjectDir != entry.ProjectDir {
			t.Errorf("expected project dir %q, got %q", entry.ProjectDir, found.ProjectDir)
		}
	})
}

func TestFindByFingerprint_returnsNilWhenMissing(t *testing.T) {
	withTempRegistryDir(t, func() {
		found := domain.FindByFingerprint("doesnotexist")
		if found != nil {
			t.Errorf("expected nil, got entry %+v", found)
		}
	})
}

func TestRemoveEntry_removesExistingEntry(t *testing.T) {
	withTempRegistryDir(t, func() {
		entry := domain.ImageEntry{
			ProjectDir:  "/home/user/proj",
			Workspace:   "proj",
			Mode:        "local-cached",
			Image:       "img:latest",
			CreatedAt:   "2024-01-01T00:00:00Z",
			LastUpdated: "2024-01-01T00:00:00Z",
		}
		_ = domain.RecordEntry(entry)

		removed := domain.RemoveEntry("/home/user/proj")
		if !removed {
			t.Error("expected RemoveEntry to return true")
		}

		entries := domain.ListEntries()
		if len(entries) != 0 {
			t.Errorf("expected 0 entries after removal, got %d", len(entries))
		}
	})
}

func TestRemoveEntry_returnsFalseWhenNotFound(t *testing.T) {
	withTempRegistryDir(t, func() {
		removed := domain.RemoveEntry("/nonexistent/path")
		if removed {
			t.Error("expected RemoveEntry to return false for nonexistent entry")
		}
	})
}

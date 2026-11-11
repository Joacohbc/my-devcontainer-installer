package domain_test

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestComputeFingerprint_sameContentProducesSameResult(t *testing.T) {
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04\nRUN apt-get update", nil, []string{"base"}, nil)
	fp2 := domain.ComputeFingerprint("FROM ubuntu:24.04\nRUN apt-get update", nil, []string{"base"}, nil)
	if fp1 != fp2 {
		t.Errorf("expected identical fingerprints, got %q and %q", fp1, fp2)
	}
}

func TestComputeFingerprint_labelChangeDoesNotAffectResult(t *testing.T) {
	withoutLabel := domain.ComputeFingerprint("FROM ubuntu:24.04\nRUN apt-get update", nil, nil, nil)
	withLabel := domain.ComputeFingerprint("FROM ubuntu:24.04\nRUN apt-get update\nLABEL foo=\"bar\"", nil, nil, nil)
	if withoutLabel != withLabel {
		t.Errorf("LABEL change should not affect fingerprint: %q vs %q", withoutLabel, withLabel)
	}
}

func TestComputeFingerprint_multilineLabelStripped(t *testing.T) {
	withoutLabel := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, nil, nil)
	withMultilineLabel := domain.ComputeFingerprint("FROM ubuntu:24.04\nLABEL foo=\"bar\" \\\n      baz=\"qux\"", nil, nil, nil)
	if withoutLabel != withMultilineLabel {
		t.Errorf("multiline LABEL should be stripped: %q vs %q", withoutLabel, withMultilineLabel)
	}
}

func TestComputeFingerprint_differentDockerfileProducesDifferentResult(t *testing.T) {
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, nil, nil)
	fp2 := domain.ComputeFingerprint("FROM ubuntu:22.04", nil, nil, nil)
	if fp1 == fp2 {
		t.Error("different Dockerfiles should produce different fingerprints")
	}
}

func TestComputeFingerprint_differentCopyFilesProducesDifferentResult(t *testing.T) {
	files1 := map[string]string{"script.sh": "echo hello"}
	files2 := map[string]string{"script.sh": "echo world"}
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04", files1, nil, nil)
	fp2 := domain.ComputeFingerprint("FROM ubuntu:24.04", files2, nil, nil)
	if fp1 == fp2 {
		t.Error("different copy file contents should produce different fingerprints")
	}
}

func TestComputeFingerprint_differentModuleIDsProducesDifferentResult(t *testing.T) {
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, []string{"base", "nodejs"}, nil)
	fp2 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, []string{"base", "python"}, nil)
	if fp1 == fp2 {
		t.Error("different module IDs should produce different fingerprints")
	}
}

func TestComputeFingerprint_moduleIDOrderDoesNotMatter(t *testing.T) {
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, []string{"nodejs", "base"}, nil)
	fp2 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, []string{"base", "nodejs"}, nil)
	if fp1 != fp2 {
		t.Errorf("module ID order should not affect fingerprint: %q vs %q", fp1, fp2)
	}
}

func TestComputeFingerprint_differentBuildArgsProduceDifferentResult(t *testing.T) {
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, nil, map[string]string{"USER_UID": "1000", "USER_GID": "1000"})
	fp2 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, nil, map[string]string{"USER_UID": "1001", "USER_GID": "1001"})
	if fp1 == fp2 {
		t.Error("different build args (host UID/GID) should produce different fingerprints")
	}
}

func TestComputeFingerprint_sameBuildArgsProduceSameResult(t *testing.T) {
	args := map[string]string{"USER_UID": "1000", "USER_GID": "1000"}
	fp1 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, nil, args)
	fp2 := domain.ComputeFingerprint("FROM ubuntu:24.04", nil, nil, map[string]string{"USER_UID": "1000", "USER_GID": "1000"})
	if fp1 != fp2 {
		t.Errorf("identical build args should produce identical fingerprints: %q vs %q", fp1, fp2)
	}
}

func TestContentImageTag(t *testing.T) {
	fp := "3f2a9c1b7d4e5f60aa"
	cases := []struct {
		name, origin, fp, want string
	}{
		{"profile origin", "ai-browser", fp, "devcontainer-cli/ai-browser-3f2a9c1b:latest"},
		{"composed profiles", "nodejs-my-tools", fp, "devcontainer-cli/nodejs-my-tools-3f2a9c1b:latest"},
		{"no origin falls back to custom", "", fp, "devcontainer-cli/custom-3f2a9c1b:latest"},
		{"unsafe characters are slugged", "My Tools/v2", fp, "devcontainer-cli/my-tools-v2-3f2a9c1b:latest"},
		{"short fingerprint used whole", "go", "abc", "devcontainer-cli/go-abc:latest"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := domain.ContentImageTag(c.origin, c.fp); got != c.want {
				t.Errorf("ContentImageTag(%q, %q) = %q, want %q", c.origin, c.fp, got, c.want)
			}
		})
	}
}

// The tag must depend on content and origin only: the workspace never appears,
// so two projects with the same content share one image.
func TestContentImageTagIgnoresWorkspace(t *testing.T) {
	a := &types.DevcontainerConfig{Mode: types.BuildModeCustom, Workspace: "alpha", Origin: "go", Fingerprint: "deadbeefcafe"}
	b := &types.DevcontainerConfig{Mode: types.BuildModeCustom, Workspace: "beta", Origin: "go", Fingerprint: "deadbeefcafe"}
	if domain.ResolveDevcontainerImageName(a) != domain.ResolveDevcontainerImageName(b) {
		t.Errorf("same content resolved to different images: %q vs %q", domain.ResolveDevcontainerImageName(a), domain.ResolveDevcontainerImageName(b))
	}
}

func TestContentImageTagTruncatesLongOrigins(t *testing.T) {
	tag := domain.ContentImageTag(strings.Repeat("a", 100), "0123456789")
	if !strings.HasSuffix(tag, "-01234567:latest") || len(tag) > len("devcontainer-cli/")+48+len("-01234567:latest") {
		t.Errorf("unexpected tag %q", tag)
	}
}

func TestIsLocalImage(t *testing.T) {
	cases := []struct {
		name  string
		image string
		want  bool
	}{
		{"content-tagged build", domain.ContentImageTag("go", "abcdef123456789012"), true},
		{"namespace with any tag", "devcontainer-cli/abc:dev", true},
		{"pulled variant", "ghcr.io/joacohbc/devcontainer-nodejs:latest", false},
		{"plain upstream image", "postgres:16", false},
		// A repository merely ending in the namespace is somebody else's image.
		{"namespace as a suffix", "ghcr.io/other/devcontainer-cli/abc:latest", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := domain.IsLocalImage(c.image); got != c.want {
				t.Errorf("IsLocalImage(%q) = %v, want %v", c.image, got, c.want)
			}
		})
	}
}

func TestRecordEntry_addsNewEntry(t *testing.T) {
	withTempXDGDir(t, func() {
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
	withTempXDGDir(t, func() {
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
	withTempXDGDir(t, func() {
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
	withTempXDGDir(t, func() {
		found := domain.FindByFingerprint("doesnotexist")
		if found != nil {
			t.Errorf("expected nil, got entry %+v", found)
		}
	})
}

func TestRemoveEntry_removesExistingEntry(t *testing.T) {
	withTempXDGDir(t, func() {
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
	withTempXDGDir(t, func() {
		removed := domain.RemoveEntry("/nonexistent/path")
		if removed {
			t.Error("expected RemoveEntry to return false for nonexistent entry")
		}
	})
}

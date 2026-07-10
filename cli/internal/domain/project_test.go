package domain_test

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestResolveWorkspace_returnsConfigWorkspaceWhenSet(t *testing.T) {
	cfg := &types.DevcontainerConfig{Workspace: "custom-workspace"}
	result := domain.ResolveWorkspace("/any/path", cfg)
	if result != "custom-workspace" {
		t.Errorf("expected %q, got %q", "custom-workspace", result)
	}
}

func TestResolveWorkspace_usesBasenameWhenConfigWorkspaceEmpty(t *testing.T) {
	cfg := &types.DevcontainerConfig{Workspace: ""}
	result := domain.ResolveWorkspace("/home/user/my-project", cfg)
	if result == "" {
		t.Error("expected a non-empty workspace derived from directory name")
	}
}

func TestResolveWorkspace_usesBasenameWhenConfigNil(t *testing.T) {
	result := domain.ResolveWorkspace("/home/user/myapp", nil)
	if result == "" {
		t.Error("expected a non-empty workspace derived from directory name")
	}
}

func TestResolveWorkspace_sanitizesDirectoryName(t *testing.T) {
	cfg := &types.DevcontainerConfig{Workspace: ""}
	result := domain.ResolveWorkspace("/home/user/My App", cfg)
	if result == "" {
		t.Error("expected non-empty sanitized workspace")
	}
	for _, ch := range result {
		isAllowed := (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.'
		if !isAllowed {
			t.Errorf("sanitized workspace %q contains invalid character %q", result, ch)
			break
		}
	}
}

func TestUniqueWorkspaceName_noCollisionReturnsBase(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := domain.UniqueWorkspaceName("/home/a/api", "api"); got != "api" {
		t.Errorf("expected base name when registry is empty, got %q", got)
	}
}

func TestUniqueWorkspaceName_sameDirIsStable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := domain.SaveRegistry([]domain.ImageEntry{{ProjectDir: "/home/a/api", Workspace: "api"}}); err != nil {
		t.Fatal(err)
	}
	// Re-resolving the same project's own name must not disambiguate it.
	if got := domain.UniqueWorkspaceName("/home/a/api", "api"); got != "api" {
		t.Errorf("same-dir name must stay stable, got %q", got)
	}
}

func TestUniqueWorkspaceName_differentDirCollisionSuffixes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := domain.SaveRegistry([]domain.ImageEntry{{ProjectDir: "/home/work/api", Workspace: "api"}}); err != nil {
		t.Fatal(err)
	}
	got := domain.UniqueWorkspaceName("/home/personal/api", "api")
	if got == "api" {
		t.Fatal("expected a disambiguated name for a colliding different directory")
	}
	if len(got) <= len("api-") || got[:4] != "api-" {
		t.Errorf("expected an 'api-<hash>' suffix, got %q", got)
	}
	// Deterministic: same path yields the same suffix.
	if again := domain.UniqueWorkspaceName("/home/personal/api", "api"); again != got {
		t.Errorf("UniqueWorkspaceName not deterministic: %q vs %q", got, again)
	}
}

func TestUniqueWorkspaceName_respectsDockerLengthLimit(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	base := strings.Repeat("a", 63)
	if err := domain.SaveRegistry([]domain.ImageEntry{{ProjectDir: "/home/work/x", Workspace: base}}); err != nil {
		t.Fatal(err)
	}
	got := domain.UniqueWorkspaceName("/home/other/x", base)
	if len(got) > 63 {
		t.Errorf("disambiguated name %q exceeds 63 chars (%d)", got, len(got))
	}
}

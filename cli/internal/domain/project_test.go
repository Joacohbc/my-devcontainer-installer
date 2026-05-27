package domain_test

import (
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

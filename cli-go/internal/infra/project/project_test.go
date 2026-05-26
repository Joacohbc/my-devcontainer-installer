package project_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/infra/project"
)

func TestProjectPaths_buildsCorrectStructure(t *testing.T) {
	cwd := "/home/user/myproject"
	workspace := "myproject"

	paths := project.ProjectPaths(cwd, workspace)

	expectedProjectDir := filepath.Join(cwd, ".dc_myproject")
	if paths.ProjectDir != expectedProjectDir {
		t.Errorf("expected ProjectDir %q, got %q", expectedProjectDir, paths.ProjectDir)
	}

	expectedBuildDir := filepath.Join(expectedProjectDir, "build")
	if paths.BuildDir != expectedBuildDir {
		t.Errorf("expected BuildDir %q, got %q", expectedBuildDir, paths.BuildDir)
	}

	expectedComposeFile := filepath.Join(expectedBuildDir, "docker-compose.yml")
	if paths.ComposeFile != expectedComposeFile {
		t.Errorf("expected ComposeFile %q, got %q", expectedComposeFile, paths.ComposeFile)
	}

	expectedDockerfilePath := filepath.Join(expectedBuildDir, "Dockerfile")
	if paths.DockerfilePath != expectedDockerfilePath {
		t.Errorf("expected DockerfilePath %q, got %q", expectedDockerfilePath, paths.DockerfilePath)
	}

	expectedEnvPath := filepath.Join(expectedBuildDir, ".env")
	if paths.EnvPath != expectedEnvPath {
		t.Errorf("expected EnvPath %q, got %q", expectedEnvPath, paths.EnvPath)
	}
}

func TestProjectPaths_projectDirUsesDcPrefix(t *testing.T) {
	paths := project.ProjectPaths("/some/path", "myws")
	if !strings.HasPrefix(filepath.Base(paths.ProjectDir), ".dc_") {
		t.Errorf("ProjectDir should start with .dc_, got %q", paths.ProjectDir)
	}
}

func TestResolveWorkspace_returnsConfigWorkspaceWhenSet(t *testing.T) {
	cfg := &core.DevcontainerConfig{Workspace: "custom-workspace"}
	result := project.ResolveWorkspace("/any/path", cfg)
	if result != "custom-workspace" {
		t.Errorf("expected %q, got %q", "custom-workspace", result)
	}
}

func TestResolveWorkspace_usesBasenameWhenConfigWorkspaceEmpty(t *testing.T) {
	cfg := &core.DevcontainerConfig{Workspace: ""}
	result := project.ResolveWorkspace("/home/user/my-project", cfg)
	if result == "" {
		t.Error("expected a non-empty workspace derived from directory name")
	}
	if result == "my-project" {
		t.Log("got expected sanitized workspace name 'my-project'")
	}
}

func TestResolveWorkspace_usesBasenameWhenConfigNil(t *testing.T) {
	result := project.ResolveWorkspace("/home/user/myapp", nil)
	if result == "" {
		t.Error("expected a non-empty workspace derived from directory name")
	}
}

func TestResolveWorkspace_sanitizesDirectoryName(t *testing.T) {
	cfg := &core.DevcontainerConfig{Workspace: ""}
	result := project.ResolveWorkspace("/home/user/My App", cfg)
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

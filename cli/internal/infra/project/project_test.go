package project_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
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

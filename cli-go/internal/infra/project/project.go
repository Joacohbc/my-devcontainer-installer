package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/domain"
)

type Paths struct {
	ProjectDir     string
	BuildDir       string
	ComposeFile    string
	DockerfilePath string
	EnvPath        string
}

func ResolveWorkspace(cwd string, config *core.DevcontainerConfig) string {
	if config != nil && config.Workspace != "" {
		return config.Workspace
	}
	return domain.SanitizeDockerName(filepath.Base(cwd), "devcontainer")
}

func ProjectPaths(cwd, workspace string) Paths {
	projectDir := filepath.Join(cwd, ".dc_"+workspace)
	buildDir := filepath.Join(projectDir, "build")
	return Paths{
		ProjectDir:     projectDir,
		BuildDir:       buildDir,
		ComposeFile:    filepath.Join(buildDir, "docker-compose.yml"),
		DockerfilePath: filepath.Join(buildDir, "Dockerfile"),
		EnvPath:        filepath.Join(buildDir, ".env"),
	}
}

func ResolveProjectComposeFile(cwd string) (string, error) {
	cfg, err := domain.LoadConfig(cwd)
	if err != nil {
		return "", err
	}
	workspace := ResolveWorkspace(cwd, cfg)
	paths := ProjectPaths(cwd, workspace)
	if _, statErr := os.Stat(paths.ComposeFile); os.IsNotExist(statErr) {
		return "", fmt.Errorf("no compose file found at %s. Run 'devcontainer-cli' to generate one first", paths.ComposeFile)
	}
	return paths.ComposeFile, nil
}

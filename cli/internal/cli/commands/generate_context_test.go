package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
)

func contextTestConfig() *types.DevcontainerConfig {
	return &types.DevcontainerConfig{
		Mode:       types.BuildModeLocalCached,
		Image:      "devcontainer-cli/test:latest",
		Workspace:  "ctxws",
		Dockerfile: types.DockerfileConfig{Modules: []types.SelectedModule{{ID: types.ModulePython}}},
	}
}

// prepareBuildDir must materialize the generated CONTEXT.md next to the
// Dockerfile: the aliases module COPYs it, so a missing file fails the build.
func TestPrepareBuildDirWritesContextFile(t *testing.T) {
	cwd := t.TempDir()
	config := contextTestConfig()
	paths := project.ProjectPaths(cwd, config.Workspace)

	copyContents, _, err := prepareBuildDir(cwd, config, paths)
	if err != nil {
		t.Fatalf("prepareBuildDir: %v", err)
	}

	onDisk, err := os.ReadFile(paths.ContextPath)
	if err != nil {
		t.Fatalf("CONTEXT.md must be written to the build dir: %v", err)
	}
	if !strings.Contains(string(onDisk), "use `uv`") {
		t.Errorf("the written document must reflect the selected modules:\n%s", onDisk)
	}

	// It must also be in copyContents, or the fingerprint would ignore it.
	inFingerprint, ok := copyContents[types.ContextFileName]
	if !ok {
		t.Fatalf("CONTEXT.md must be folded into copyContents, got keys %v", keysOf(copyContents))
	}
	if inFingerprint != string(onDisk) {
		t.Error("the fingerprinted content must be exactly what was written to disk")
	}
}

// Adding a database service changes CONTEXT.md but not the Dockerfile. Unless
// the document feeds the fingerprint, two such projects would share one image
// and one of them would ship the wrong document.
func TestPrepareBuildDirContextVariesWithServices(t *testing.T) {
	bare := contextTestConfig()
	bareCwd := t.TempDir()
	bareContents, _, err := prepareBuildDir(bareCwd, bare, project.ProjectPaths(bareCwd, bare.Workspace))
	if err != nil {
		t.Fatalf("prepareBuildDir (bare): %v", err)
	}

	withDB := contextTestConfig()
	withDB.Compose.Services = []types.SelectedService{{ID: types.ServicePostgres}}
	dbCwd := t.TempDir()
	dbContents, _, err := prepareBuildDir(dbCwd, withDB, project.ProjectPaths(dbCwd, withDB.Workspace))
	if err != nil {
		t.Fatalf("prepareBuildDir (with postgres): %v", err)
	}

	if bareContents[types.ContextFileName] == dbContents[types.ContextFileName] {
		t.Error("adding a compose service must change the fingerprinted CONTEXT.md")
	}
	if !strings.Contains(dbContents[types.ContextFileName], "PostgreSQL (sibling container)") {
		t.Errorf("the postgres section is missing:\n%s", dbContents[types.ContextFileName])
	}
}

// Remote builds skip every build artifact, CONTEXT.md included: the prebuilt
// image ships the document it was built with.
func TestPrepareBuildDirSkipsContextForRemote(t *testing.T) {
	cwd := t.TempDir()
	config := contextTestConfig()
	config.Mode = types.BuildModeRemote
	paths := project.ProjectPaths(cwd, config.Workspace)

	copyContents, _, err := prepareBuildDir(cwd, config, paths)
	if err != nil {
		t.Fatalf("prepareBuildDir: %v", err)
	}
	if _, ok := copyContents[types.ContextFileName]; ok {
		t.Error("remote mode must not fingerprint a CONTEXT.md")
	}
	if _, err := os.Stat(filepath.Join(paths.BuildDir, types.ContextFileName)); !os.IsNotExist(err) {
		t.Errorf("remote mode must not write CONTEXT.md, stat err = %v", err)
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

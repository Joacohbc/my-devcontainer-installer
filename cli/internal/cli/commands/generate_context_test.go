package commands

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
)

func contextTestConfig() *types.DevcontainerConfig {
	return &types.DevcontainerConfig{
		Mode:       types.BuildModeCustom,
		Image:      "devcontainer-cli/test:latest",
		Workspace:  "ctxws",
		Dockerfile: types.DockerfileConfig{Modules: []types.SelectedModule{{ID: types.ModulePython}}},
	}
}

// prepareBuildDir must materialize the generated CONTEXT.md next to the compose
// file, which bind-mounts it: a missing file would make Docker create a
// directory in its place.
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

	// It is mounted, not baked: it must stay out of the fingerprint, or every
	// workspace (whose path it names) would get its own image.
	if _, ok := copyContents[types.ContextFileName]; ok {
		t.Errorf("CONTEXT.md must not feed the fingerprint, got keys %v", keysOf(copyContents))
	}
}

// Adding a database service changes CONTEXT.md, which reaches the container
// through the mount, without forking the image.
func TestPrepareBuildDirContextVariesWithServices(t *testing.T) {
	bare := contextTestConfig()
	bareCwd := t.TempDir()
	barePaths := project.ProjectPaths(bareCwd, bare.Workspace)
	bareContents, _, err := prepareBuildDir(bareCwd, bare, barePaths)
	if err != nil {
		t.Fatalf("prepareBuildDir (bare): %v", err)
	}

	withDB := contextTestConfig()
	withDB.Compose.Services = []types.SelectedService{{ID: types.ServicePostgres}}
	dbCwd := t.TempDir()
	dbPaths := project.ProjectPaths(dbCwd, withDB.Workspace)
	dbContents, _, err := prepareBuildDir(dbCwd, withDB, dbPaths)
	if err != nil {
		t.Fatalf("prepareBuildDir (with postgres): %v", err)
	}

	dbDoc, _ := os.ReadFile(dbPaths.ContextPath)
	if !strings.Contains(string(dbDoc), "PostgreSQL (sibling container)") {
		t.Errorf("the postgres section is missing:\n%s", dbDoc)
	}
	if strings.Join(keysOf(bareContents), ",") != strings.Join(keysOf(dbContents), ",") {
		t.Error("a compose-only change must not alter what feeds the fingerprint")
	}
}

// Prebuilt images (profiles/image modes) get an accurate CONTEXT.md too: it is
// mounted, so it no longer depends on what the image was built with.
func TestPrepareBuildDirWritesContextForRemote(t *testing.T) {
	cwd := t.TempDir()
	config := contextTestConfig()
	config.Mode = types.BuildModeProfiles
	paths := project.ProjectPaths(cwd, config.Workspace)

	if _, _, err := prepareBuildDir(cwd, config, paths); err != nil {
		t.Fatalf("prepareBuildDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.BuildDir, types.ContextFileName)); err != nil {
		t.Errorf("remote mode must write CONTEXT.md for the mount: %v", err)
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

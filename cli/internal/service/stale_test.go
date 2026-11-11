package service

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
)

func generatedProject(t *testing.T) (*types.DevcontainerConfig, project.Paths) {
	t.Helper()
	defer useFakeDocker(&fakeRunner{status: 0})()
	cwd := t.TempDir()
	config := &types.DevcontainerConfig{
		Mode:       types.BuildModeCustom,
		Workspace:  "ws",
		Dockerfile: types.DockerfileConfig{Modules: []types.SelectedModule{{ID: "nodejs"}}},
	}
	paths := project.ProjectPaths(cwd, config.Workspace)
	svc := GenerateService{Report: nopReporter{}}
	contents, _, err := svc.PrepareBuildDir(config, paths)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := svc.Plan(config, contents)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.DockerfilePath, []byte(plan.Dockerfile), 0o644); err != nil {
		t.Fatal(err)
	}
	return config, paths
}

func TestStaleBuildReasonsFreshProjectIsNotStale(t *testing.T) {
	config, paths := generatedProject(t)
	if got := StaleBuildReasons(config, paths); len(got) != 0 {
		t.Errorf("freshly generated files reported stale: %v", got)
	}
}

// A CLI upgrade only bumps the version label: that alone is not staleness.
func TestStaleBuildReasonsIgnoresTheCLIVersionLabel(t *testing.T) {
	config, paths := generatedProject(t)
	types.CLIVersion = "9.9.9"
	t.Cleanup(func() { types.CLIVersion = "dev" })
	if got := StaleBuildReasons(config, paths); len(got) != 0 {
		t.Errorf("a version-only change reported stale: %v", got)
	}
}

func TestStaleBuildReasonsDetectsChangedFiles(t *testing.T) {
	config, paths := generatedProject(t)
	df, _ := os.ReadFile(paths.DockerfilePath)
	if err := os.WriteFile(paths.DockerfilePath, []byte(strings.Replace(string(df), "FROM ", "FROM old-", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.BuildDir, "entrypoint.sh"), []byte("#!/bin/sh\n# old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := StaleBuildReasons(config, paths)
	for _, want := range []string{"the Dockerfile", "entrypoint.sh"} {
		if !slices.Contains(got, want) {
			t.Errorf("expected %q among %v", want, got)
		}
	}
}

func TestStaleBuildReasonsSkipsPrebuiltModes(t *testing.T) {
	config, paths := generatedProject(t)
	config.Mode = types.BuildModeProfiles
	if got := StaleBuildReasons(config, paths); got != nil {
		t.Errorf("prebuilt modes have no local build files to compare: %v", got)
	}
}

// The fix the warning names must actually clear it: regenerating rewrites an
// outdated embedded helper instead of keeping the old copy forever.
func TestStaleBuildReasonsClearAfterRegenerating(t *testing.T) {
	config, paths := generatedProject(t)
	if err := os.WriteFile(filepath.Join(paths.BuildDir, "entrypoint.sh"), []byte("#!/bin/sh\n# old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (GenerateService{Report: nopReporter{}}).PrepareBuildDir(config, paths); err != nil {
		t.Fatal(err)
	}
	if got := StaleBuildReasons(config, paths); len(got) != 0 {
		t.Errorf("still stale after regenerating: %v", got)
	}
}

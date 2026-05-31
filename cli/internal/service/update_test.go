package service

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
)

func TestUpdateOneLocalComputesComposeArgs(t *testing.T) {
	cases := []struct {
		name     string
		pull     bool
		rebuild  bool
		wantArgs []string
	}{
		{"default rebuilds with pull", false, false, []string{"build", "--pull"}},
		{"pure rebuild skips pull", false, true, []string{"build"}},
		{"forced pull", true, true, []string{"build", "--pull"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runner := &fakeRunner{status: 0}
			restore := useFakeDocker(runner)
			defer restore()

			projectDir := t.TempDir()
			config := &types.DevcontainerConfig{Mode: types.BuildModeLocalCached, Workspace: "ws", Image: "img:latest"}
			composeFile := project.ProjectPaths(projectDir, config.Workspace).ComposeFile
			if err := os.MkdirAll(filepath.Dir(composeFile), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(composeFile, []byte("services: {}"), 0o644); err != nil {
				t.Fatal(err)
			}

			svc := UpdateService{Report: nopReporter{}}
			_, ok := svc.UpdateOne(projectDir, config, c.pull, c.rebuild)
			if !ok {
				t.Fatal("expected UpdateOne to succeed")
			}
			call := runner.callContaining("build")
			if call == nil {
				t.Fatalf("no compose build call recorded; calls=%v", runner.calls)
			}
			for _, want := range c.wantArgs {
				if !slices.Contains(call, want) {
					t.Errorf("compose call %v missing %q", call, want)
				}
			}
			if slices.Contains(c.wantArgs, "--pull") != slices.Contains(call, "--pull") {
				t.Errorf("--pull mismatch: call=%v wantArgs=%v", call, c.wantArgs)
			}
		})
	}
}

func TestUpdateOneRemotePulls(t *testing.T) {
	runner := &fakeRunner{status: 0}
	restore := useFakeDocker(runner)
	defer restore()

	config := &types.DevcontainerConfig{
		Mode:      types.BuildModeRemote,
		Workspace: "ws",
		Remote:    &types.RemoteConfig{Variant: "nodejs"},
	}
	svc := UpdateService{Report: nopReporter{}}
	_, ok := svc.UpdateOne(t.TempDir(), config, false, false)
	if !ok {
		t.Fatal("expected remote UpdateOne to succeed")
	}
	if call := runner.callContaining("pull"); call == nil {
		t.Errorf("expected a docker pull call; calls=%v", runner.calls)
	}
}

func TestUpdateOneRemoteMissingConfigFails(t *testing.T) {
	restore := useFakeDocker(&fakeRunner{status: 0})
	defer restore()

	config := &types.DevcontainerConfig{Mode: types.BuildModeRemote, Workspace: "ws"}
	svc := UpdateService{Report: nopReporter{}}
	if _, ok := svc.UpdateOne(t.TempDir(), config, false, false); ok {
		t.Error("expected failure when remote config is missing")
	}
}

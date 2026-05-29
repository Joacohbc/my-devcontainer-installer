package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
)

func newUpdateService(compose func(projectDir, composeFile string, args []string) error) UpdateService {
	return UpdateService{
		Report:  NopReporter{},
		Pull:    func(string) error { return nil },
		Compose: compose,
	}
}

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
			projectDir := t.TempDir()
			config := &types.DevcontainerConfig{Mode: types.BuildModeLocalCached, Workspace: "ws", Image: "img:latest"}
			composeFile := project.ProjectPaths(projectDir, config.Workspace).ComposeFile
			if err := os.MkdirAll(filepath.Dir(composeFile), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(composeFile, []byte("services: {}"), 0o644); err != nil {
				t.Fatal(err)
			}

			var gotArgs []string
			svc := newUpdateService(func(_, _ string, args []string) error { gotArgs = args; return nil })
			_, ok := svc.UpdateOne(projectDir, config, c.pull, c.rebuild)
			if !ok {
				t.Fatal("expected UpdateOne to succeed")
			}
			if len(gotArgs) != len(c.wantArgs) {
				t.Fatalf("args = %v, want %v", gotArgs, c.wantArgs)
			}
			for i := range c.wantArgs {
				if gotArgs[i] != c.wantArgs[i] {
					t.Errorf("args = %v, want %v", gotArgs, c.wantArgs)
				}
			}
		})
	}
}

func TestUpdateOneRemoteMissingConfigFails(t *testing.T) {
	config := &types.DevcontainerConfig{Mode: types.BuildModeRemote, Workspace: "ws", Remote: nil}
	svc := newUpdateService(func(_, _ string, _ []string) error { return nil })
	if _, ok := svc.UpdateOne(t.TempDir(), config, false, false); ok {
		t.Error("expected UpdateOne to fail when remote config is missing")
	}
}

func TestUpdateAllNoEntries(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("APPDATA", tmp)

	svc := newUpdateService(func(_, _ string, _ []string) error { return nil })
	updated, skipped, failed := svc.UpdateAll(false, false)
	if updated != 0 || skipped != 0 || failed != 0 {
		t.Errorf("got (%d,%d,%d), want all zero", updated, skipped, failed)
	}
}

func TestUpdateAllCountsFailures(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("APPDATA", tmp)

	projectDir := t.TempDir()
	config := domain.DefaultConfig(projectDir)
	config.Mode = types.BuildModeRemote
	config.Remote = nil
	if err := domain.SaveConfig(config, projectDir); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if err := domain.RecordEntry(domain.ImageEntry{
		ProjectDir: projectDir,
		Workspace:  config.Workspace,
		Mode:       string(config.Mode),
		Image:      config.Image,
	}); err != nil {
		t.Fatalf("RecordEntry: %v", err)
	}

	svc := newUpdateService(func(_, _ string, _ []string) error { return nil })
	if _, _, failed := svc.UpdateAll(false, false); failed == 0 {
		t.Error("expected at least one failed project")
	}
}

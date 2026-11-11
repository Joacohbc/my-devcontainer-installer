package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

// buildStatusRunner reports docker as available and answers every other call
// with buildStatus, so the build step can be made to fail without a daemon.
type buildStatusRunner struct {
	buildStatus int
	calls       [][]string
}

func (r *buildStatusRunner) Run(_ context.Context, args []string, _ string, _ string, _ map[string]string) (int, string, string) {
	if len(args) >= 2 && args[1] == "version" {
		return 0, "27.0.0", ""
	}
	r.calls = append(r.calls, args)
	return r.buildStatus, "", ""
}

func runSaveAndPostProcess(t *testing.T, status int) error {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cwd := t.TempDir()

	runner := &buildStatusRunner{buildStatus: status}
	docker.SetRunner(runner)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	sharedConfigOff := false
	config := &types.DevcontainerConfig{
		Mode:       types.BuildModeCustom,
		Image:      "devcontainer-cli/test:latest",
		Workspace:  "buildws",
		Dockerfile: types.DockerfileConfig{Modules: []types.SelectedModule{}},
		Compose:    types.ComposeConfig{SharedConfig: &sharedConfigOff},
	}
	build := true
	flags := &genFlags{build: &build, interactive: false}

	return saveAndPostProcess(cwd, config, &service.GeneratePlan{}, project.ProjectPaths(cwd, config.Workspace), flags, generateService(), nil)
}

// A build that failed is not a generate that succeeded. Reporting it as a
// warning with a zero exit reads as success to a script or to 'agent create',
// which then goes on to use an image that was never produced.
func TestSaveAndPostProcessFailsWhenTheBuildFails(t *testing.T) {
	err := runSaveAndPostProcess(t, 1)
	if err == nil {
		t.Fatal("a failed build must fail the command, not warn and return nil")
	}
	if !strings.Contains(err.Error(), "build") {
		t.Errorf("error = %q, want it to name the failed build", err)
	}
}

func TestSaveAndPostProcessSucceedsWhenTheBuildSucceeds(t *testing.T) {
	if err := runSaveAndPostProcess(t, 0); err != nil {
		t.Fatalf("a successful build must not fail the command: %v", err)
	}
}

func TestParseGenFlags_BuildFalse(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	flags, err := parseGenFlagsFor(t, "--build=false")
	if err != nil {
		t.Fatalf("parseGenFlags: %v", err)
	}
	if flags.build == nil {
		t.Fatal("expected flags.build to be non-nil when --build=false is set")
	}
	if *flags.build != false {
		t.Fatalf("flags.build = %v, want false", *flags.build)
	}
}

func TestBuildFalseDoesNotTriggerComposeBuild(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cwd := t.TempDir()

	runner := &buildStatusRunner{buildStatus: 1} // Fails if build is invoked
	docker.SetRunner(runner)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	flags, err := parseGenFlagsFor(t, "--build=false", "--no-interactive")
	if err != nil {
		t.Fatalf("parseGenFlags: %v", err)
	}

	sharedConfigOff := false
	config := &types.DevcontainerConfig{
		Mode:       types.BuildModeCustom,
		Image:      "devcontainer-cli/test:latest",
		Workspace:  "buildws",
		Dockerfile: types.DockerfileConfig{Modules: []types.SelectedModule{}},
		Compose:    types.ComposeConfig{SharedConfig: &sharedConfigOff},
	}

	if err := saveAndPostProcess(cwd, config, &service.GeneratePlan{}, project.ProjectPaths(cwd, config.Workspace), flags, generateService(), nil); err != nil {
		t.Fatalf("saveAndPostProcess failed: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Errorf("expected no docker runner calls when --build=false, got %v", runner.calls)
	}
}

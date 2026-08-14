package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestAgentInfoCoversTheCatalogue(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	info := AgentService{Report: nopReporter{}}.Info("test")

	if info.Version != "test" {
		t.Errorf("Version = %q, want the injected version", info.Version)
	}

	for _, m := range catalog.DockerfileModules {
		idx := slices.IndexFunc(info.Modules, func(e AgentModuleInfo) bool { return e.ID == string(m.ID) })
		if idx < 0 {
			t.Fatalf("module %q missing from the catalogue dump", m.ID)
		}
		got := info.Modules[idx]
		if got.Label != m.Label {
			t.Errorf("module %q label = %q, want %q", m.ID, got.Label, m.Label)
		}
		// Selectable is what an agent puts in --with, so it must exclude both the
		// always-on modules and the ones the generator derives.
		if want := !m.Always && !m.Internal; got.Selectable != want {
			t.Errorf("module %q selectable = %v, want %v", m.ID, got.Selectable, want)
		}
	}

	for _, svc := range catalog.ComposeServices {
		idx := slices.IndexFunc(info.Services, func(e AgentServiceInfo) bool { return e.ID == string(svc.ID) })
		if idx < 0 {
			t.Fatalf("service %q missing from the catalogue dump", svc.ID)
		}
		if want := !svc.Always && !svc.Internal; info.Services[idx].Selectable != want {
			t.Errorf("service %q selectable = %v, want %v", svc.ID, info.Services[idx].Selectable, want)
		}
	}

	for _, id := range types.RemoteVariants {
		if !slices.Contains(info.RemoteVariants, id) {
			t.Errorf("remote variant %q missing from the catalogue dump", id)
		}
	}
}

// The skills module is the one an agent must never pass to --with: the
// generator adds it from --skill. Reported as internal, it can be filtered out.
func TestAgentInfoMarksInternalModules(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	info := AgentService{Report: nopReporter{}}.Info("test")

	idx := slices.IndexFunc(info.Modules, func(e AgentModuleInfo) bool { return e.ID == string(types.ModuleSkills) })
	if idx < 0 {
		t.Fatal("skills module missing from the catalogue dump")
	}
	if !info.Modules[idx].Internal || info.Modules[idx].Selectable {
		t.Errorf("skills module = %+v, want internal and not selectable", info.Modules[idx])
	}
}

func TestAgentInfoScriptsAndSkills(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	info := AgentService{Report: nopReporter{}}.Info("test")

	if len(info.Scripts.Whens) != len(types.ScriptWhens) {
		t.Fatalf("script whens = %d, want %d", len(info.Scripts.Whens), len(types.ScriptWhens))
	}
	for i, w := range types.ScriptWhens {
		got := info.Scripts.Whens[i]
		if got.When != string(w) {
			t.Errorf("script when[%d] = %q, want %q", i, got.When, w)
		}
		if got.Description == "" || got.Location == "" {
			t.Errorf("script when %q has an empty description or location: %+v", w, got)
		}
	}
	if info.Scripts.Default != string(types.DefaultScriptWhen) {
		t.Errorf("script default = %q, want %q", info.Scripts.Default, types.DefaultScriptWhen)
	}

	for _, sk := range catalog.AllAgentSkills() {
		idx := slices.IndexFunc(info.Skills, func(e AgentSkillInfo) bool { return e.ID == string(sk.ID) })
		if idx < 0 {
			t.Fatalf("skill %q missing from the catalogue dump", sk.ID)
		}
		if info.Skills[idx].InstallRef != sk.InstallRef() {
			t.Errorf("skill %q installRef = %q, want %q", sk.ID, info.Skills[idx].InstallRef, sk.InstallRef())
		}
	}
}

// The paths section is what keeps an agent off a bare /workspace, so it must
// carry the per-project mount rather than the root.
func TestAgentInfoPaths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	paths := AgentService{Report: nopReporter{}}.Info("test").Paths

	if want := types.WorkspaceDir(paths.WorkspacePlaceholder); paths.WorkspaceMount != want {
		t.Errorf("workspaceMount = %q, want %q", paths.WorkspaceMount, want)
	}
	if want := types.WorkspaceAlias(paths.WorkspacePlaceholder); paths.WorkspaceAlias != want {
		t.Errorf("workspaceAlias = %q, want %q", paths.WorkspaceAlias, want)
	}
	if paths.DevUserHome != types.DevUserHome {
		t.Errorf("devUserHome = %q, want %q", paths.DevUserHome, types.DevUserHome)
	}
}

func TestAgentInfoSerializesToJSON(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	data, err := json.Marshal(AgentService{Report: nopReporter{}}.Info("test"))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, key := range []string{"version", "modules", "services", "profiles", "skills", "scripts", "assets", "remoteVariants", "paths", "buildModes"} {
		if _, ok := round[key]; !ok {
			t.Errorf("JSON output is missing the %q key", key)
		}
	}
}

// imageListRunner answers `docker images` with a canned managed-image table and
// every other call with success, so image resolution can be exercised without a
// daemon.
type imageListRunner struct {
	mu     sync.Mutex
	calls  [][]string
	images string
}

func (r *imageListRunner) Run(_ context.Context, args []string, _ string, _ string, _ map[string]string) (int, string, string) {
	if len(args) >= 2 && args[1] == "version" {
		return 0, "27.0.0", ""
	}
	r.mu.Lock()
	r.calls = append(r.calls, args)
	r.mu.Unlock()
	if len(args) >= 2 && args[1] == "images" {
		return 0, r.images, ""
	}
	return 0, "", ""
}

func (r *imageListRunner) callContaining(token string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, call := range r.calls {
		if slices.Contains(call, token) {
			return call
		}
	}
	return nil
}

// agentCleanProject lays out a project on disk and returns its destroy target.
func agentCleanProject(t *testing.T) DestroyTarget {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	projectDir := filepath.Join(tmp, ".dc_ws")
	composeFile := filepath.Join(projectDir, "build", "docker-compose.yml")
	configPath := filepath.Join(tmp, "devcontainer.config.json")
	if err := os.MkdirAll(filepath.Dir(composeFile), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{composeFile, configPath} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return DestroyTarget{
		Workspace:   "ws",
		ComposeFile: composeFile,
		ProjectDir:  projectDir,
		ConfigPath:  configPath,
		ProjectKey:  tmp,
	}
}

func TestAgentCleanDestroysProjectAndRemovesItsImage(t *testing.T) {
	target := agentCleanProject(t)
	const image = types.ImageNamespace + "/abc123def456:latest"

	runner := &imageListRunner{images: image + "\tsha256:abc"}
	defer useFakeDocker(runner)()

	svc := AgentService{Report: nopReporter{}, Prompt: scriptedPrompter{}}
	if err := svc.Clean(target, image, AgentCleanOptions{Yes: true}); err != nil {
		t.Fatalf("Clean: %v", err)
	}

	if call := runner.callContaining("down"); call == nil {
		t.Errorf("expected a compose down call; calls=%v", runner.calls)
	} else if !slices.Contains(call, "-v") {
		t.Errorf("compose down must take the volumes with it; got %v", call)
	}
	if call := runner.callContaining("rmi"); call == nil {
		t.Errorf("expected the project image to be removed; calls=%v", runner.calls)
	} else if !slices.Contains(call, image) {
		t.Errorf("rmi targeted %v, want %s", call, image)
	}
	if fileExists(target.ProjectDir) {
		t.Error("project dir should have been removed")
	}
	if fileExists(target.ConfigPath) {
		t.Error("config file should have been removed")
	}
}

// A pulled image backs every project on the same profile, so one project's
// cleanup must leave it alone.
func TestAgentCleanKeepsAPulledImage(t *testing.T) {
	target := agentCleanProject(t)
	const image = "ghcr.io/joacohbc/devcontainer-nodejs:latest"

	runner := &imageListRunner{images: image + "\tsha256:abc"}
	defer useFakeDocker(runner)()

	svc := AgentService{Report: nopReporter{}, Prompt: scriptedPrompter{}}
	if err := svc.Clean(target, image, AgentCleanOptions{Yes: true}); err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if call := runner.callContaining("rmi"); call != nil {
		t.Errorf("a pulled image must not be removed; got %v", call)
	}
}

// An image that is already gone is not an error: the destroy still has to
// finish and report success.
func TestAgentCleanToleratesAMissingImage(t *testing.T) {
	target := agentCleanProject(t)

	runner := &imageListRunner{images: ""}
	defer useFakeDocker(runner)()

	svc := AgentService{Report: nopReporter{}, Prompt: scriptedPrompter{}}
	if err := svc.Clean(target, types.ImageNamespace+"/deadbeef1234:latest", AgentCleanOptions{Yes: true}); err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if call := runner.callContaining("rmi"); call != nil {
		t.Errorf("nothing to remove, yet rmi ran: %v", call)
	}
	if fileExists(target.ProjectDir) {
		t.Error("project dir should have been removed")
	}
}

// Without --all the sweep stops at this project: nothing goes looking at the
// machine's other managed networks or volumes.
func TestAgentCleanStaysScopedToTheProject(t *testing.T) {
	target := agentCleanProject(t)

	runner := &imageListRunner{}
	defer useFakeDocker(runner)()

	svc := AgentService{Report: nopReporter{}, Prompt: scriptedPrompter{}}
	if err := svc.Clean(target, "", AgentCleanOptions{Yes: true}); err != nil {
		t.Fatalf("Clean: %v", err)
	}
	for _, token := range []string{"network", "volume"} {
		if call := runner.callContaining(token); call != nil {
			t.Errorf("project-scoped clean queried %q: %v", token, call)
		}
	}
}

func TestAgentCleanAllSweepsBeyondTheProject(t *testing.T) {
	target := agentCleanProject(t)

	runner := &imageListRunner{}
	defer useFakeDocker(runner)()

	svc := AgentService{Report: nopReporter{}, Prompt: scriptedPrompter{}}
	if err := svc.Clean(target, "", AgentCleanOptions{Yes: true, All: true}); err != nil {
		t.Fatalf("Clean: %v", err)
	}
	for _, token := range []string{"network", "volume"} {
		if call := runner.callContaining(token); call == nil {
			t.Errorf("--all must sweep %s too; calls=%v", token, runner.calls)
		}
	}
}

func TestAgentCleanRequiresYesWhenNonInteractive(t *testing.T) {
	target := agentCleanProject(t)

	runner := &imageListRunner{}
	defer useFakeDocker(runner)()

	svc := AgentService{Report: nopReporter{}, Prompt: scriptedPrompter{}}
	err := svc.Clean(target, "", AgentCleanOptions{})
	if err == nil {
		t.Fatal("expected an error without --yes in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error = %q, want it to name --yes", err)
	}
	if len(runner.calls) > 0 {
		t.Errorf("nothing may run before the confirmation: %v", runner.calls)
	}
	if !fileExists(target.ProjectDir) {
		t.Error("project dir must survive a refused clean")
	}
}

func TestAgentCleanDryRunRemovesNothing(t *testing.T) {
	target := agentCleanProject(t)
	const image = types.ImageNamespace + "/abc123def456:latest"

	runner := &imageListRunner{images: image + "\tsha256:abc"}
	defer useFakeDocker(runner)()

	svc := AgentService{Report: nopReporter{}, Prompt: scriptedPrompter{}}
	if err := svc.Clean(target, image, AgentCleanOptions{Yes: true, DryRun: true}); err != nil {
		t.Fatalf("Clean: %v", err)
	}

	for _, token := range []string{"down", "rmi", "rm"} {
		if call := runner.callContaining(token); call != nil {
			t.Errorf("dry run ran a destructive %q: %v", token, call)
		}
	}
	if !fileExists(target.ProjectDir) {
		t.Error("dry run must not delete the project dir")
	}
	if !fileExists(target.ConfigPath) {
		t.Error("dry run must not delete the config file")
	}
}

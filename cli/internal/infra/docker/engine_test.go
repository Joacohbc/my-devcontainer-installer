package docker_test

import (
	"context"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

type enginesRunner struct {
	availableBinaries map[string]bool
	calls             [][]string
	lastEnv           map[string]string
}

func (r *enginesRunner) Run(_ context.Context, args []string, _ string, _ string, env map[string]string) (int, string, string) {
	r.calls = append(r.calls, args)
	r.lastEnv = env
	isVersionProbe := len(args) >= 2 && args[1] == "version"
	if isVersionProbe && !r.availableBinaries[args[0]] {
		return 1, "", "not found"
	}
	return 0, "", ""
}

func useEngines(t *testing.T, availableBinaries ...string) *enginesRunner {
	t.Helper()
	runner := &enginesRunner{availableBinaries: map[string]bool{}}
	for _, binary := range availableBinaries {
		runner.availableBinaries[binary] = true
	}
	docker.SetRunner(runner)
	docker.ResetEngine()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetEngine()
	})
	return runner
}

func TestParseEnginePreference(t *testing.T) {
	cases := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{"", docker.PreferenceAuto, false},
		{"auto", docker.PreferenceAuto, false},
		{"docker", docker.PreferenceDocker, false},
		{"  Podman ", docker.PreferencePodman, false},
		{"containerd", "", true},
	}
	for _, c := range cases {
		got, err := docker.ParseEnginePreference(c.raw)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseEnginePreference(%q) error = %v, wantErr %v", c.raw, err, c.wantErr)
		}
		if got != c.want {
			t.Errorf("ParseEnginePreference(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestEnginePreferences_listsEveryAcceptedValue(t *testing.T) {
	got := strings.Join(docker.EnginePreferences(), ",")
	if got != "auto,docker,podman" {
		t.Errorf("EnginePreferences = %q, want auto,docker,podman", got)
	}
}

func TestCurrentEngine_autoPrefersDocker(t *testing.T) {
	useEngines(t, "docker", "podman")

	if got := docker.CurrentEngine(); got != docker.EngineDocker {
		t.Errorf("CurrentEngine = %+v, want docker", got)
	}
}

func TestCurrentEngine_autoFallsBackToPodman(t *testing.T) {
	useEngines(t, "podman")

	if got := docker.CurrentEngine(); got != docker.EnginePodman {
		t.Errorf("CurrentEngine = %+v, want podman", got)
	}
	if !docker.IsPodman() {
		t.Error("IsPodman = false, want true")
	}
}

func TestCurrentEngine_autoWithNothingInstalledStaysDocker(t *testing.T) {
	useEngines(t)

	if got := docker.CurrentEngine(); got != docker.EngineDocker {
		t.Errorf("CurrentEngine = %+v, want docker", got)
	}
	err := docker.EnsureDocker()
	if err == nil {
		t.Fatal("EnsureDocker = nil, want an error when no engine is available")
	}
	if !strings.Contains(err.Error(), "docker is not installed") || !strings.Contains(err.Error(), "podman") {
		t.Errorf("error = %q, want it to name both engines", err)
	}
}

func TestCurrentEngine_explicitPreferenceSkipsProbing(t *testing.T) {
	runner := useEngines(t, "docker")
	docker.SetEnginePreference(docker.PreferencePodman)

	if got := docker.CurrentEngine(); got != docker.EnginePodman {
		t.Errorf("CurrentEngine = %+v, want podman", got)
	}
	if len(runner.calls) != 0 {
		t.Errorf("explicit preference probed the runner: %v", runner.calls)
	}
}

func TestEnsureDocker_namesTheSelectedEngineWhenItIsMissing(t *testing.T) {
	useEngines(t, "docker")
	docker.SetEnginePreference(docker.PreferencePodman)

	err := docker.EnsureDocker()
	if err == nil || !strings.HasPrefix(err.Error(), "podman is not installed") {
		t.Errorf("EnsureDocker error = %v, want one naming podman", err)
	}
}

func TestSetEnginePreference_resetsAResolvedEngine(t *testing.T) {
	useEngines(t, "docker", "podman")
	docker.CurrentEngine()

	docker.SetEnginePreference(docker.PreferencePodman)

	if got := docker.CurrentEngine(); got != docker.EnginePodman {
		t.Errorf("CurrentEngine after SetEnginePreference = %+v, want podman", got)
	}
}

func TestPodmanEngine_runsThePodmanBinary(t *testing.T) {
	runner := useEngines(t, "podman")
	docker.SetEngine(docker.EnginePodman)

	if _, _, _, err := docker.DockerCapture([]string{"ps"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	last := runner.calls[len(runner.calls)-1]
	if last[0] != "podman" || last[1] != "ps" {
		t.Errorf("runner received %v, want podman ps", last)
	}
}

func TestPodmanEngine_composeUsesPodmanCompose(t *testing.T) {
	runner := useEngines(t, "podman")
	docker.SetEngine(docker.EnginePodman)

	if _, err := docker.DockerCompose("/tmp/dc.yml", []string{"up", "-d"}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.Join(runner.calls[len(runner.calls)-1], " ")
	if got != "podman compose -f /tmp/dc.yml up -d" {
		t.Errorf("compose invocation = %q", got)
	}
}

func TestPodmanEngine_composeFailureNamesPodman(t *testing.T) {
	useRunner(t, &argMockRunner{versionStatus: 0, otherStatus: 3})
	docker.SetEngine(docker.EnginePodman)

	err := docker.DockerComposeOrThrow("/tmp/dc.yml", []string{"down"}, nil)
	if err == nil || !strings.HasPrefix(err.Error(), "podman compose") {
		t.Errorf("error = %v, want one starting with 'podman compose'", err)
	}
}

func TestHostOverride_usesTheEnginesRemoteHostVariable(t *testing.T) {
	cases := []struct {
		engine  docker.Engine
		wantVar string
	}{
		{docker.EngineDocker, "DOCKER_HOST"},
		{docker.EnginePodman, "CONTAINER_HOST"},
	}
	for _, c := range cases {
		runner := useEngines(t, c.engine.Binary)
		docker.SetEngine(c.engine)
		docker.SetHostOverride("me@remote-host")

		if _, _, _, err := docker.DockerCapture([]string{"ps"}); err != nil {
			t.Fatalf("%s: unexpected error: %v", c.engine.Name, err)
		}
		docker.ResetHostOverride()

		if got, want := runner.lastEnv[c.wantVar], "ssh://me@remote-host"; got != want {
			t.Errorf("%s: %s = %q, want %q", c.engine.Name, c.wantVar, got, want)
		}
	}
}

func TestPodmanEngine_failedComposePrintsTheProviderHint(t *testing.T) {
	useRunner(t, &argMockRunner{versionStatus: 0, otherStatus: 1})
	var hint strings.Builder
	defer docker.CaptureComposeHint(&hint)()

	docker.SetEngine(docker.EnginePodman)
	if status, _ := docker.DockerCompose("/tmp/dc.yml", []string{"up"}, nil); status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	for _, want := range []string{"podman.socket", "PODMAN_COMPOSE_PROVIDER"} {
		if !strings.Contains(hint.String(), want) {
			t.Errorf("hint = %q, want it to mention %s", hint.String(), want)
		}
	}
}

func TestDockerEngine_failedComposePrintsNoPodmanHint(t *testing.T) {
	useRunner(t, &argMockRunner{versionStatus: 0, otherStatus: 1})
	var hint strings.Builder
	defer docker.CaptureComposeHint(&hint)()

	docker.SetEngine(docker.EngineDocker)
	docker.DockerCompose("/tmp/dc.yml", []string{"up"}, nil)
	if hint.Len() != 0 {
		t.Errorf("docker compose failure printed %q, want nothing", hint.String())
	}
}

func TestPodmanEngine_successfulComposePrintsNoHint(t *testing.T) {
	useRunner(t, &argMockRunner{versionStatus: 0, otherStatus: 0})
	var hint strings.Builder
	defer docker.CaptureComposeHint(&hint)()

	docker.SetEngine(docker.EnginePodman)
	docker.DockerCompose("/tmp/dc.yml", []string{"up"}, nil)
	if hint.Len() != 0 {
		t.Errorf("successful compose printed %q, want nothing", hint.String())
	}
}

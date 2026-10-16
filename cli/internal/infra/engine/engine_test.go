package engine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/engine"
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
	engine.SetRunner(runner)
	engine.ResetEngine()
	t.Cleanup(func() {
		engine.ResetRunner()
		engine.ResetEngine()
	})
	return runner
}

func TestParseEnginePreference(t *testing.T) {
	cases := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{"", engine.PreferenceAuto, false},
		{"auto", engine.PreferenceAuto, false},
		{"docker", engine.PreferenceDocker, false},
		{"  Podman ", engine.PreferencePodman, false},
		{"containerd", "", true},
	}
	for _, c := range cases {
		got, err := engine.ParseEnginePreference(c.raw)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseEnginePreference(%q) error = %v, wantErr %v", c.raw, err, c.wantErr)
		}
		if got != c.want {
			t.Errorf("ParseEnginePreference(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestEnginePreferences_listsEveryAcceptedValue(t *testing.T) {
	got := strings.Join(engine.EnginePreferences(), ",")
	if got != "auto,docker,podman" {
		t.Errorf("EnginePreferences = %q, want auto,docker,podman", got)
	}
}

func TestCurrentEngine_autoPrefersDocker(t *testing.T) {
	useEngines(t, "docker", "podman")

	if got := engine.CurrentEngine(); got != engine.EngineDocker {
		t.Errorf("CurrentEngine = %+v, want docker", got)
	}
}

func TestCurrentEngine_autoFallsBackToPodman(t *testing.T) {
	useEngines(t, "podman")

	if got := engine.CurrentEngine(); got != engine.EnginePodman {
		t.Errorf("CurrentEngine = %+v, want podman", got)
	}
	if !engine.IsPodman() {
		t.Error("IsPodman = false, want true")
	}
}

func TestCurrentEngine_autoWithNothingInstalledStaysDocker(t *testing.T) {
	useEngines(t)

	if got := engine.CurrentEngine(); got != engine.EngineDocker {
		t.Errorf("CurrentEngine = %+v, want docker", got)
	}
	err := engine.EnsureAvailable()
	if err == nil {
		t.Fatal("EnsureAvailable = nil, want an error when no engine is available")
	}
	if !strings.Contains(err.Error(), "docker is not installed") || !strings.Contains(err.Error(), "podman") {
		t.Errorf("error = %q, want it to name both engines", err)
	}
}

func TestCurrentEngine_explicitPreferenceSkipsProbing(t *testing.T) {
	runner := useEngines(t, "docker")
	engine.SetEnginePreference(engine.PreferencePodman)

	if got := engine.CurrentEngine(); got != engine.EnginePodman {
		t.Errorf("CurrentEngine = %+v, want podman", got)
	}
	if len(runner.calls) != 0 {
		t.Errorf("explicit preference probed the runner: %v", runner.calls)
	}
}

func TestEnsureDocker_namesTheSelectedEngineWhenItIsMissing(t *testing.T) {
	useEngines(t, "docker")
	engine.SetEnginePreference(engine.PreferencePodman)

	err := engine.EnsureAvailable()
	if err == nil || !strings.HasPrefix(err.Error(), "podman is not installed") {
		t.Errorf("EnsureAvailable error = %v, want one naming podman", err)
	}
}

func TestSetEnginePreference_resetsAResolvedEngine(t *testing.T) {
	useEngines(t, "docker", "podman")
	engine.CurrentEngine()

	engine.SetEnginePreference(engine.PreferencePodman)

	if got := engine.CurrentEngine(); got != engine.EnginePodman {
		t.Errorf("CurrentEngine after SetEnginePreference = %+v, want podman", got)
	}
}

func TestPodmanEngine_runsThePodmanBinary(t *testing.T) {
	runner := useEngines(t, "podman")
	engine.SetEngine(engine.EnginePodman)

	if _, _, _, err := engine.Capture([]string{"ps"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	last := runner.calls[len(runner.calls)-1]
	if last[0] != "podman" || last[1] != "ps" {
		t.Errorf("runner received %v, want podman ps", last)
	}
}

func TestPodmanEngine_composeUsesPodmanCompose(t *testing.T) {
	runner := useEngines(t, "podman")
	engine.SetEngine(engine.EnginePodman)

	if _, err := engine.Compose("/tmp/dc.yml", []string{"up", "-d"}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.Join(runner.calls[len(runner.calls)-1], " ")
	if got != "podman compose -f /tmp/dc.yml up -d" {
		t.Errorf("compose invocation = %q", got)
	}
}

func TestPodmanEngine_composeFailureNamesPodman(t *testing.T) {
	useRunner(t, &argMockRunner{versionStatus: 0, otherStatus: 3})
	engine.SetEngine(engine.EnginePodman)

	err := engine.ComposeOrThrow("/tmp/dc.yml", []string{"down"}, nil)
	if err == nil || !strings.HasPrefix(err.Error(), "podman compose") {
		t.Errorf("error = %v, want one starting with 'podman compose'", err)
	}
}

func TestHostOverride_usesTheEnginesRemoteHostVariable(t *testing.T) {
	cases := []struct {
		engine  engine.Engine
		wantVar string
	}{
		{engine.EngineDocker, "DOCKER_HOST"},
		{engine.EnginePodman, "CONTAINER_HOST"},
	}
	for _, c := range cases {
		runner := useEngines(t, c.engine.Binary)
		engine.SetEngine(c.engine)
		engine.SetHostOverride("me@remote-host")

		if _, _, _, err := engine.Capture([]string{"ps"}); err != nil {
			t.Fatalf("%s: unexpected error: %v", c.engine.Name, err)
		}
		engine.ResetHostOverride()

		if got, want := runner.lastEnv[c.wantVar], "ssh://me@remote-host"; got != want {
			t.Errorf("%s: %s = %q, want %q", c.engine.Name, c.wantVar, got, want)
		}
	}
}

func TestPodmanEngine_failedComposePrintsTheProviderHint(t *testing.T) {
	useRunner(t, &argMockRunner{versionStatus: 0, otherStatus: 1})
	var hint strings.Builder
	defer engine.CaptureComposeHint(&hint)()

	engine.SetEngine(engine.EnginePodman)
	if status, _ := engine.Compose("/tmp/dc.yml", []string{"up"}, nil); status != 1 {
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
	defer engine.CaptureComposeHint(&hint)()

	engine.SetEngine(engine.EngineDocker)
	engine.Compose("/tmp/dc.yml", []string{"up"}, nil)
	if hint.Len() != 0 {
		t.Errorf("docker compose failure printed %q, want nothing", hint.String())
	}
}

func TestPodmanEngine_successfulComposePrintsNoHint(t *testing.T) {
	useRunner(t, &argMockRunner{versionStatus: 0, otherStatus: 0})
	var hint strings.Builder
	defer engine.CaptureComposeHint(&hint)()

	engine.SetEngine(engine.EnginePodman)
	engine.Compose("/tmp/dc.yml", []string{"up"}, nil)
	if hint.Len() != 0 {
		t.Errorf("successful compose printed %q, want nothing", hint.String())
	}
}

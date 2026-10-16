package engine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/engine"
)

type mockRunner struct {
	callCount int
	status    int
	stdout    string
	stderr    string
}

func (m *mockRunner) Run(ctx context.Context, args []string, stdio string, cwd string, env map[string]string) (int, string, string) {
	m.callCount++
	return m.status, m.stdout, m.stderr
}

func TestIsDockerAvailable_returnsTrueWhenRunnerSucceeds(t *testing.T) {
	mock := &mockRunner{status: 0, stdout: "27.0.0", stderr: ""}
	engine.SetRunner(mock)
	engine.ResetAvailabilityCache()
	t.Cleanup(func() {
		engine.ResetRunner()
		engine.ResetAvailabilityCache()
	})

	if !engine.IsAvailable() {
		t.Error("expected IsAvailable to return true when runner exits 0")
	}
}

func TestIsDockerAvailable_returnsFalseWhenRunnerFails(t *testing.T) {
	mock := &mockRunner{status: 1, stdout: "", stderr: "Cannot connect to the Docker daemon"}
	engine.SetRunner(mock)
	engine.ResetAvailabilityCache()
	t.Cleanup(func() {
		engine.ResetRunner()
		engine.ResetAvailabilityCache()
	})

	if engine.IsAvailable() {
		t.Error("expected IsAvailable to return false when runner exits non-zero")
	}
}

func TestIsDockerAvailable_cachesPreviousResult(t *testing.T) {
	mock := &mockRunner{status: 0, stdout: "27.0.0", stderr: ""}
	engine.SetRunner(mock)
	engine.ResetAvailabilityCache()
	t.Cleanup(func() {
		engine.ResetRunner()
		engine.ResetAvailabilityCache()
	})

	engine.IsAvailable()
	engine.IsAvailable()
	engine.IsAvailable()

	if mock.callCount != 1 {
		t.Errorf("expected runner to be called once (cached), got %d calls", mock.callCount)
	}
}

func TestResetDockerCache_clearsCache(t *testing.T) {
	mock := &mockRunner{status: 0, stdout: "27.0.0", stderr: ""}
	engine.SetRunner(mock)
	engine.ResetAvailabilityCache()
	t.Cleanup(func() {
		engine.ResetRunner()
		engine.ResetAvailabilityCache()
	})

	engine.IsAvailable()
	engine.ResetAvailabilityCache()
	engine.IsAvailable()

	if mock.callCount != 2 {
		t.Errorf("expected runner to be called twice after cache reset, got %d calls", mock.callCount)
	}
}

func TestEnsureDocker_returnsNilWhenAvailable(t *testing.T) {
	mock := &mockRunner{status: 0, stdout: "27.0.0", stderr: ""}
	engine.SetRunner(mock)
	engine.ResetAvailabilityCache()
	t.Cleanup(func() {
		engine.ResetRunner()
		engine.ResetAvailabilityCache()
	})

	if err := engine.EnsureAvailable(); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestEnsureDocker_returnsErrorWhenUnavailable(t *testing.T) {
	mock := &mockRunner{status: 1, stdout: "", stderr: "not found"}
	engine.SetRunner(mock)
	engine.ResetAvailabilityCache()
	t.Cleanup(func() {
		engine.ResetRunner()
		engine.ResetAvailabilityCache()
	})

	if err := engine.EnsureAvailable(); err == nil {
		t.Error("expected error when docker unavailable, got nil")
	}
}

type argMockRunner struct {
	versionStatus int
	otherStatus   int
	otherStdout   string
	lastArgs      []string
	lastEnv       map[string]string
}

func (m *argMockRunner) Run(ctx context.Context, args []string, stdio string, cwd string, env map[string]string) (int, string, string) {
	m.lastArgs = args
	m.lastEnv = env
	if len(args) >= 2 && args[1] == "version" {
		return m.versionStatus, "27.0.0", ""
	}
	return m.otherStatus, m.otherStdout, ""
}

// useRunner swaps in a fake runner and pins the docker engine, so a test never
// depends on which engine an earlier test left selected.
func useRunner(t *testing.T, r engine.Runner) {
	engine.SetRunner(r)
	engine.SetEngine(engine.EngineDocker)
	t.Cleanup(func() {
		engine.ResetRunner()
		engine.ResetEngine()
	})
}

func TestDockerCapture_returnsRunnerOutput(t *testing.T) {
	mock := &argMockRunner{versionStatus: 0, otherStatus: 0, otherStdout: "container-a\n"}
	useRunner(t, mock)

	status, stdout, _, err := engine.Capture([]string{"ps", "-a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 0 || stdout != "container-a\n" {
		t.Errorf("Capture = (%d, %q), want (0, %q)", status, stdout, "container-a\n")
	}
	if len(mock.lastArgs) == 0 || mock.lastArgs[0] != "docker" || mock.lastArgs[1] != "ps" {
		t.Errorf("runner received unexpected args: %v", mock.lastArgs)
	}
}

func TestDockerCapture_errorsWhenDockerUnavailable(t *testing.T) {
	useRunner(t, &argMockRunner{versionStatus: 1})

	if _, _, _, err := engine.Capture([]string{"ps"}); err == nil {
		t.Error("expected error when docker unavailable")
	}
}

func TestDockerCompose_buildsArgs(t *testing.T) {
	mock := &argMockRunner{versionStatus: 0, otherStatus: 0}
	useRunner(t, mock)

	status, err := engine.Compose("/tmp/dc.yml", []string{"up", "-d"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 0 {
		t.Errorf("status = %d, want 0", status)
	}
	want := []string{"docker", "compose", "-f", "/tmp/dc.yml", "up", "-d"}
	if len(mock.lastArgs) != len(want) {
		t.Fatalf("args = %v, want %v", mock.lastArgs, want)
	}
	for i := range want {
		if mock.lastArgs[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q", i, mock.lastArgs[i], want[i])
		}
	}
}

func TestDockerComposeOrThrow_nilOnSuccess(t *testing.T) {
	useRunner(t, &argMockRunner{versionStatus: 0, otherStatus: 0})

	if err := engine.ComposeOrThrow("/tmp/dc.yml", []string{"down"}, nil); err != nil {
		t.Errorf("expected nil on success, got %v", err)
	}
}

func TestDockerComposeOrThrow_errorOnNonZeroExit(t *testing.T) {
	useRunner(t, &argMockRunner{versionStatus: 0, otherStatus: 5})

	if err := engine.ComposeOrThrow("/tmp/dc.yml", []string{"down"}, nil); err == nil {
		t.Error("expected error when compose exits non-zero")
	}
}

func TestDockerCapture_hostOverrideUnset_passesNilEnv(t *testing.T) {
	mock := &argMockRunner{versionStatus: 0, otherStatus: 0}
	useRunner(t, mock)

	if _, _, _, err := engine.Capture([]string{"ps"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.lastEnv != nil {
		t.Errorf("expected nil env with no host override, got %v", mock.lastEnv)
	}
}

func TestDockerCapture_hostOverrideSet_setsDockerHost(t *testing.T) {
	mock := &argMockRunner{versionStatus: 0, otherStatus: 0}
	useRunner(t, mock)
	engine.SetHostOverride("me@remote-host")
	t.Cleanup(engine.ResetHostOverride)

	if _, _, _, err := engine.Capture([]string{"ps"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := mock.lastEnv["DOCKER_HOST"], "ssh://me@remote-host"; got != want {
		t.Errorf("DOCKER_HOST = %q, want %q", got, want)
	}
}

func TestDockerInherit_hostOverrideSet_setsDockerHost(t *testing.T) {
	mock := &argMockRunner{versionStatus: 0, otherStatus: 0}
	useRunner(t, mock)
	engine.SetHostOverride("me@remote-host")
	t.Cleanup(engine.ResetHostOverride)

	if _, err := engine.Inherit([]string{"exec", "c", "true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := mock.lastEnv["DOCKER_HOST"], "ssh://me@remote-host"; got != want {
		t.Errorf("DOCKER_HOST = %q, want %q", got, want)
	}
}

func TestResetHostOverride_clearsIt(t *testing.T) {
	mock := &argMockRunner{versionStatus: 0, otherStatus: 0}
	useRunner(t, mock)
	engine.SetHostOverride("me@remote-host")
	engine.ResetHostOverride()

	if _, _, _, err := engine.Capture([]string{"ps"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.lastEnv != nil {
		t.Errorf("expected nil env after ResetHostOverride, got %v", mock.lastEnv)
	}
}

func TestRun_hostOverrideReplacesInheritedDockerHost(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///wrong.sock")
	engine.SetHostOverride("me@remote-host")
	t.Cleanup(engine.ResetHostOverride)

	runner := &engine.DefaultRunner{}
	// "env" (a shell builtin-less binary) prints the child's environment; used
	// here purely to observe what Run actually put in cmd.Env.
	_, stdout, _ := runner.Run(context.Background(), []string{"env"}, "pipe", "", map[string]string{"DOCKER_HOST": "ssh://me@remote-host"})
	count := strings.Count(stdout, "DOCKER_HOST=")
	if count != 1 {
		t.Fatalf("expected exactly one DOCKER_HOST in child env, got %d in:\n%s", count, stdout)
	}
	if !strings.Contains(stdout, "DOCKER_HOST=ssh://me@remote-host") {
		t.Errorf("expected DOCKER_HOST=ssh://me@remote-host, got:\n%s", stdout)
	}
}

func TestDockerGlobalContext_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	engine.SetContext(ctx)
	t.Cleanup(func() {
		engine.SetContext(context.Background())
	})

	runner := &engine.DefaultRunner{}
	engine.SetRunner(runner)
	t.Cleanup(func() {
		engine.ResetRunner()
	})

	// Cancel the context immediately
	cancel()

	// Since context is canceled, any docker command using default runner should fail immediately
	// sleep 10 is used as a test command that would otherwise block
	status, _, _ := runner.Run(ctx, []string{"sleep", "10"}, "pipe", "", nil)
	if status == 0 {
		t.Error("expected non-zero status or failure when context is canceled")
	}
}

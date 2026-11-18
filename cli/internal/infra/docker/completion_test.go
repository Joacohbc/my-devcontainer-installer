package docker_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

type completionMockRunner struct {
	mu       sync.Mutex
	calls    int
	lastArgs []string
	lastEnv  map[string]string
	hasDeadl bool
	status   int
	stdout   string
	stderr   string
	delay    time.Duration
}

func (m *completionMockRunner) Run(ctx context.Context, args []string, stdio string, cwd string, env map[string]string) (int, string, string) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return 1, "", ctx.Err().Error()
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.lastArgs = append([]string(nil), args...)
	m.lastEnv = env
	_, m.hasDeadl = ctx.Deadline()
	return m.status, m.stdout, m.stderr
}

func (m *completionMockRunner) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func TestWithRemoteCompletion_CacheBehavior(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		seedAge         *time.Duration
		seedStdout      string
		runnerStatus    int
		runnerStdout    string
		wantStdout      string
		wantCalls       int
		wantRefreshCall bool
	}{
		{
			name:         "cache miss executes runner and writes cache",
			runnerStatus: 0,
			runnerStdout: "ctr-alpha\n",
			wantStdout:   "ctr-alpha\n",
			wantCalls:    1,
		},
		{
			name:         "fresh cache hit skips runner",
			seedAge:      ptrDuration(10 * time.Second),
			seedStdout:   "ctr-cached\n",
			runnerStatus: 0,
			runnerStdout: "ctr-new\n",
			wantStdout:   "ctr-cached\n",
			wantCalls:    0,
		},
		{
			name:            "stale cache hit returns stale data and triggers refresh",
			seedAge:         ptrDuration(5 * time.Minute),
			seedStdout:      "ctr-stale\n",
			runnerStatus:    0,
			runnerStdout:    "ctr-refreshed\n",
			wantStdout:      "ctr-stale\n",
			wantCalls:       0,
			wantRefreshCall: true,
		},
		{
			name:         "expired cache beyond stale TTL executes runner",
			seedAge:      ptrDuration(45 * time.Minute),
			seedStdout:   "ctr-expired\n",
			runnerStatus: 0,
			runnerStdout: "ctr-live\n",
			wantStdout:   "ctr-live\n",
			wantCalls:    1,
		},
		{
			name:         "non-zero runner exit does not overwrite valid cache",
			seedAge:      ptrDuration(45 * time.Minute),
			seedStdout:   "ctr-old\n",
			runnerStatus: 1,
			runnerStdout: "",
			wantStdout:   "",
			wantCalls:    1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			docker.SetCompletionCacheDir(cacheDir)
			docker.SetCompletionNow(func() time.Time { return now })
			t.Cleanup(func() {
				docker.SetCompletionCacheDir("")
				docker.SetCompletionNow(nil)
				docker.SetCompletionRefreshHook(nil)
			})

			mock := &completionMockRunner{
				status: tc.runnerStatus,
				stdout: tc.runnerStdout,
			}
			useRunner(t, mock)

			if tc.seedAge != nil {
				docker.SetCompletionNow(func() time.Time { return now.Add(-*tc.seedAge) })
				seedMock := &completionMockRunner{status: 0, stdout: tc.seedStdout}
				docker.SetRunner(seedMock)
				_ = docker.WithRemoteCompletion("me@host", func() error {
					_, _, _, err := docker.DockerCapture([]string{"ps", "-a"})
					return err
				})
				docker.SetRunner(mock)
				docker.SetCompletionNow(func() time.Time { return now })
			}

			refreshCalled := false
			var refreshHost string
			var refreshArgs []string
			docker.SetCompletionRefreshHook(func(host string, args []string) {
				refreshCalled = true
				refreshHost = host
				refreshArgs = append([]string(nil), args...)
			})

			var gotStdout string
			err := docker.WithRemoteCompletion("me@host", func() error {
				_, out, _, derr := docker.DockerCapture([]string{"ps", "-a"})
				gotStdout = out
				return derr
			})
			if err != nil {
				t.Fatalf("WithRemoteCompletion error: %v", err)
			}
			if gotStdout != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", gotStdout, tc.wantStdout)
			}
			if mock.callCount() != tc.wantCalls {
				t.Errorf("runner calls = %d, want %d", mock.callCount(), tc.wantCalls)
			}
			if refreshCalled != tc.wantRefreshCall {
				t.Errorf("refreshCalled = %v, want %v", refreshCalled, tc.wantRefreshCall)
			}
			if tc.wantRefreshCall && (refreshHost != "me@host" || len(refreshArgs) != 2 || refreshArgs[0] != "ps") {
				t.Errorf("unexpected refresh target (%q, %v)", refreshHost, refreshArgs)
			}
		})
	}
}

func TestWithRemoteCompletion_EnforcesTimeoutAndSkipsEnsureDocker(t *testing.T) {
	cacheDir := t.TempDir()
	docker.SetCompletionCacheDir(cacheDir)
	docker.SetCompletionTimeout(50 * time.Millisecond)
	t.Cleanup(func() {
		docker.SetCompletionCacheDir("")
		docker.SetCompletionTimeout(0)
		docker.SetCompletionRefreshHook(nil)
	})

	refreshTriggered := false
	docker.SetCompletionRefreshHook(func(host string, args []string) {
		refreshTriggered = host == "me@slow-host" && len(args) == 1 && args[0] == "ps"
	})

	// If EnsureDocker ran "docker version", this mock would record an extra call
	// and fail if versionStatus != 0; with completion mode it is skipped.
	mock := &completionMockRunner{
		status: 0,
		stdout: "ok",
		delay:  250 * time.Millisecond,
	}
	useRunner(t, mock)

	var status int
	err := docker.WithRemoteCompletion("me@slow-host", func() error {
		var derr error
		status, _, _, derr = docker.DockerCapture([]string{"ps"})
		return derr
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status == 0 {
		t.Errorf("expected non-zero status on timeout, got 0")
	}
	if !refreshTriggered {
		t.Errorf("expected background refresh to trigger after foreground timeout")
	}
}

func TestWithRemoteCompletion_SetsWrapperAndCompletionEnv(t *testing.T) {
	cacheDir := t.TempDir()
	docker.SetCompletionCacheDir(cacheDir)
	t.Cleanup(func() {
		docker.SetCompletionCacheDir("")
	})

	mock := &completionMockRunner{status: 0, stdout: "ctr-1\n"}
	useRunner(t, mock)

	err := docker.WithRemoteCompletion("me@remote", func() error {
		_, _, _, derr := docker.DockerCapture([]string{"ps"})
		return derr
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.hasDeadl {
		t.Error("expected completion DockerCapture to pass a context with deadline")
	}
	if mock.lastEnv["DOCKER_HOST"] != "ssh://me@remote" {
		t.Errorf("DOCKER_HOST = %q, want ssh://me@remote", mock.lastEnv["DOCKER_HOST"])
	}
	if mock.lastEnv["DC_SSH_COMPLETION"] != "1" {
		t.Errorf("DC_SSH_COMPLETION = %q, want 1", mock.lastEnv["DC_SSH_COMPLETION"])
	}
	if pathVal := mock.lastEnv["PATH"]; pathVal != "" {
		sshBinDir := filepath.Join(cacheDir, "devcontainer-cli", "ssh-bin")
		if !strings.HasPrefix(pathVal, sshBinDir+string(os.PathListSeparator)) {
			t.Errorf("PATH = %q, expected prefix %q", pathVal, sshBinDir)
		}
		wrapperBytes, rerr := os.ReadFile(filepath.Join(sshBinDir, "ssh"))
		if rerr != nil {
			t.Fatalf("expected ssh wrapper to be written: %v", rerr)
		}
		wrapper := string(wrapperBytes)
		for _, want := range []string{"BatchMode=yes", "ConnectTimeout=4"} {
			if !strings.Contains(wrapper, want) {
				t.Errorf("ssh wrapper missing %q:\n%s", want, wrapper)
			}
		}
		if strings.Contains(wrapper, "ControlPersist") {
			t.Errorf("ssh wrapper must not set ControlPersist (causes pipe hang):\n%s", wrapper)
		}
	}
}

func TestRunCompletionRefreshIfRequested_SetsDockerHostWithoutHostOverride(t *testing.T) {
	cacheDir := t.TempDir()
	docker.SetCompletionCacheDir(cacheDir)
	t.Cleanup(func() { docker.SetCompletionCacheDir("") })

	origArgs := os.Args
	os.Args = []string{"devcontainer-cli", "ps", "-a"}
	t.Cleanup(func() { os.Args = origArgs })

	t.Setenv("DC_COMPLETION_REFRESH", "1")
	t.Setenv("DC_COMPLETION_HOST", "me@remote")

	mock := &completionMockRunner{status: 0, stdout: "ctr-refreshed\n"}
	useRunner(t, mock)

	if handled := docker.RunCompletionRefreshIfRequested(context.Background()); !handled {
		t.Fatal("expected RunCompletionRefreshIfRequested to handle request")
	}
	if got, want := mock.lastEnv["DOCKER_HOST"], "ssh://me@remote"; got != want {
		t.Errorf("DOCKER_HOST = %q, want %q", got, want)
	}

	// Subsequent foreground lookup must hit the freshly written cache without calling runner.
	checkMock := &completionMockRunner{status: 1, stdout: "wrong"}
	docker.SetRunner(checkMock)
	var gotStdout string
	err := docker.WithRemoteCompletion("me@remote", func() error {
		_, out, _, derr := docker.DockerCapture([]string{"ps", "-a"})
		gotStdout = out
		return derr
	})
	if err != nil {
		t.Fatalf("WithRemoteCompletion error: %v", err)
	}
	if gotStdout != "ctr-refreshed\n" {
		t.Errorf("cached stdout = %q, want %q", gotStdout, "ctr-refreshed\n")
	}
	if checkMock.callCount() != 0 {
		t.Errorf("expected cache hit with 0 runner calls, got %d", checkMock.callCount())
	}
}

func TestPrewarmRemoteCompletion_TriggersRefreshWhenMissingOrStale(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	cacheDir := t.TempDir()
	docker.SetCompletionCacheDir(cacheDir)
	docker.SetCompletionNow(func() time.Time { return now })
	t.Cleanup(func() {
		docker.SetCompletionCacheDir("")
		docker.SetCompletionNow(nil)
		docker.SetCompletionRefreshHook(nil)
	})

	calls := 0
	docker.SetCompletionRefreshHook(func(host string, args []string) {
		calls++
	})

	docker.PrewarmRemoteCompletion("me@remote", []string{"ps", "-a"})
	if calls != 1 {
		t.Fatalf("expected 1 prewarm call on cache miss, got %d", calls)
	}

	// Seed a fresh cache entry: prewarm must skip it.
	useRunner(t, &completionMockRunner{status: 0, stdout: "fresh\n"})
	_ = docker.WithRemoteCompletion("me@remote", func() error {
		_, _, _, derr := docker.DockerCapture([]string{"ps", "-a"})
		return derr
	})

	docker.PrewarmRemoteCompletion("me@remote", []string{"ps", "-a"})
	if calls != 1 {
		t.Errorf("expected prewarm to skip fresh cache, got %d calls", calls)
	}

	// Advance clock past fresh TTL (3m): prewarm must refresh again.
	docker.SetCompletionNow(func() time.Time { return now.Add(5 * time.Minute) })
	docker.PrewarmRemoteCompletion("me@remote", []string{"ps", "-a"})
	if calls != 2 {
		t.Errorf("expected prewarm to refresh stale cache, got %d calls", calls)
	}
}

func ptrDuration(d time.Duration) *time.Duration {
	return &d
}

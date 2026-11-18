package service

import (
	"errors"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

func TestWithHostOverride_RedirectsDockerCallsDuringFn(t *testing.T) {
	fake := &fakeRunner{status: 0}
	defer useFakeDocker(fake)()

	err := WithHostOverride("me@remote-host", func() error {
		_, _, _, derr := docker.DockerCapture([]string{"ps"})
		return derr
	})
	if err != nil {
		t.Fatalf("WithHostOverride: %v", err)
	}
	if got, want := fake.lastEnv["DOCKER_HOST"], "ssh://me@remote-host"; got != want {
		t.Errorf("DOCKER_HOST during fn = %q, want %q", got, want)
	}

	// The override must not leak past WithHostOverride's return.
	if _, _, _, derr := docker.DockerCapture([]string{"ps"}); derr != nil {
		t.Fatalf("DockerCapture after WithHostOverride: %v", derr)
	}
	if fake.lastEnv != nil {
		t.Errorf("DOCKER_HOST leaked after WithHostOverride returned: %v", fake.lastEnv)
	}
}

func TestWithHostOverride_EmptyHostRunsUnchanged(t *testing.T) {
	fake := &fakeRunner{status: 0}
	defer useFakeDocker(fake)()

	called := false
	err := WithHostOverride("", func() error {
		called = true
		_, _, _, derr := docker.DockerCapture([]string{"ps"})
		return derr
	})
	if err != nil {
		t.Fatalf("WithHostOverride: %v", err)
	}
	if !called {
		t.Error("expected fn to be called with an empty host")
	}
	if fake.lastEnv != nil {
		t.Errorf("expected no env override with an empty host, got %v", fake.lastEnv)
	}
}

func TestWithHostOverride_PropagatesFnError(t *testing.T) {
	wantErr := errors.New("boom")
	err := WithHostOverride("me@remote-host", func() error { return wantErr })
	if err != wantErr {
		t.Errorf("WithHostOverride error = %v, want %v", err, wantErr)
	}
}

func TestWithRemoteCompletion_DelegatesToInfra(t *testing.T) {
	docker.SetCompletionCacheDir(t.TempDir())
	t.Cleanup(func() { docker.SetCompletionCacheDir("") })

	fake := &fakeRunner{status: 0, stdout: "ctr-a\n"}
	defer useFakeDocker(fake)()

	err := WithRemoteCompletion("me@remote-host", func() error {
		_, _, _, derr := docker.DockerCapture([]string{"ps"})
		return derr
	})
	if err != nil {
		t.Fatalf("WithRemoteCompletion: %v", err)
	}
	if got, want := fake.lastEnv["DOCKER_HOST"], "ssh://me@remote-host"; got != want {
		t.Errorf("DOCKER_HOST = %q, want %q", got, want)
	}
	if got, want := fake.lastEnv["DC_SSH_COMPLETION"], "1"; got != want {
		t.Errorf("DC_SSH_COMPLETION = %q, want %q", got, want)
	}
}

func TestPrewarmRemoteContainers_TriggersInfraPrewarm(t *testing.T) {
	docker.SetCompletionCacheDir(t.TempDir())
	t.Cleanup(func() {
		docker.SetCompletionCacheDir("")
		docker.SetCompletionRefreshHook(nil)
	})

	var hosts []string
	docker.SetCompletionRefreshHook(func(host string, args []string) {
		hosts = append(hosts, host)
	})

	PrewarmRemoteContainers("", "host-1", "host-2", "host-3", "host-4", "host-5")
	if len(hosts) != 4 {
		t.Fatalf("expected prewarm to cap at 4 hosts, got %v", hosts)
	}
	if hosts[0] != "host-1" || hosts[3] != "host-4" {
		t.Errorf("unexpected prewarmed hosts order: %v", hosts)
	}
}

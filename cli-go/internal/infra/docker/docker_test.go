package docker_test

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/infra/docker"
)

type mockRunner struct {
	callCount int
	status    int
	stdout    string
	stderr    string
}

func (m *mockRunner) Run(args []string, stdio string, cwd string, env map[string]string) (int, string, string) {
	m.callCount++
	return m.status, m.stdout, m.stderr
}

func TestIsDockerAvailable_returnsTrueWhenRunnerSucceeds(t *testing.T) {
	mock := &mockRunner{status: 0, stdout: "27.0.0", stderr: ""}
	docker.SetRunner(mock)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	if !docker.IsDockerAvailable() {
		t.Error("expected IsDockerAvailable to return true when runner exits 0")
	}
}

func TestIsDockerAvailable_returnsFalseWhenRunnerFails(t *testing.T) {
	mock := &mockRunner{status: 1, stdout: "", stderr: "Cannot connect to the Docker daemon"}
	docker.SetRunner(mock)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	if docker.IsDockerAvailable() {
		t.Error("expected IsDockerAvailable to return false when runner exits non-zero")
	}
}

func TestIsDockerAvailable_cachesPreviousResult(t *testing.T) {
	mock := &mockRunner{status: 0, stdout: "27.0.0", stderr: ""}
	docker.SetRunner(mock)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	docker.IsDockerAvailable()
	docker.IsDockerAvailable()
	docker.IsDockerAvailable()

	if mock.callCount != 1 {
		t.Errorf("expected runner to be called once (cached), got %d calls", mock.callCount)
	}
}

func TestResetDockerCache_clearsCache(t *testing.T) {
	mock := &mockRunner{status: 0, stdout: "27.0.0", stderr: ""}
	docker.SetRunner(mock)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	docker.IsDockerAvailable()
	docker.ResetDockerCache()
	docker.IsDockerAvailable()

	if mock.callCount != 2 {
		t.Errorf("expected runner to be called twice after cache reset, got %d calls", mock.callCount)
	}
}

func TestEnsureDocker_returnsNilWhenAvailable(t *testing.T) {
	mock := &mockRunner{status: 0, stdout: "27.0.0", stderr: ""}
	docker.SetRunner(mock)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	if err := docker.EnsureDocker(); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestEnsureDocker_returnsErrorWhenUnavailable(t *testing.T) {
	mock := &mockRunner{status: 1, stdout: "", stderr: "not found"}
	docker.SetRunner(mock)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	if err := docker.EnsureDocker(); err == nil {
		t.Error("expected error when docker unavailable, got nil")
	}
}

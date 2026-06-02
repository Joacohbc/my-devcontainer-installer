package service

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// seqRunner returns responses in order of call index (skipping the docker version check).
type seqRunner struct {
	mu        sync.Mutex
	responses []struct {
		status int
		stdout string
	}
	calls [][]string
	idx   int
}

func (r *seqRunner) Run(_ context.Context, args []string, _, _ string, _ map[string]string) (int, string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(args) >= 2 && args[1] == "version" {
		return 0, "27.0.0", ""
	}
	r.calls = append(r.calls, args)
	if r.idx >= len(r.responses) {
		return 0, "", ""
	}
	resp := r.responses[r.idx]
	r.idx++
	return resp.status, resp.stdout, ""
}

func (r *seqRunner) callContaining(token string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if slices.Contains(c, token) {
			return c
		}
	}
	return nil
}

func TestPasswordShowReturnsEntry(t *testing.T) {
	shadowEntry := "devuser:$6$hash:19000:0:99999:7:::"
	runner := &seqRunner{responses: []struct {
		status int
		stdout string
	}{
		{0, "running"},   // inspect (ensureRunning)
		{0, shadowEntry}, // getent shadow
	}}
	docker.SetRunner(runner)
	docker.ResetDockerCache()
	defer func() { docker.ResetRunner(); docker.ResetDockerCache() }()

	svc := PasswordService{Report: nopReporter{}}
	out, err := svc.ShowPassword("c1")
	if err != nil {
		t.Fatalf("ShowPassword: %v", err)
	}
	if out != shadowEntry {
		t.Errorf("ShowPassword = %q, want %q", out, shadowEntry)
	}
	call := runner.callContaining("getent")
	if call == nil {
		t.Fatal("expected getent call")
	}
	if !slices.Contains(call, "shadow") || !slices.Contains(call, "devuser") {
		t.Errorf("getent call missing shadow/devuser: %v", call)
	}
}

func TestPasswordShowFailsWhenNotRunning(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "exited"}
	defer useFakeDocker(runner)()

	svc := PasswordService{Report: nopReporter{}}
	_, err := svc.ShowPassword("c1")
	if err == nil {
		t.Fatal("expected error when container not running")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("error should mention 'not running': %v", err)
	}
}

func TestPasswordChangeNonInteractive(t *testing.T) {
	runner := &seqRunner{responses: []struct {
		status int
		stdout string
	}{
		{0, "running"}, // inspect (ensureRunning)
		{0, ""},        // chpasswd
	}}
	docker.SetRunner(runner)
	docker.ResetDockerCache()
	defer func() { docker.ResetRunner(); docker.ResetDockerCache() }()

	svc := PasswordService{Report: nopReporter{}}
	if err := svc.ChangePasswordNonInteractive("c1", "s3cr3t"); err != nil {
		t.Fatalf("ChangePasswordNonInteractive: %v", err)
	}
	call := runner.callContaining("chpasswd")
	if call == nil {
		t.Fatal("expected chpasswd call")
	}
	if !slices.Contains(call, "-i") || !slices.Contains(call, "c1") {
		t.Errorf("chpasswd call unexpected: %v", call)
	}
}

func TestPasswordChangeNonInteractiveRejectsEmpty(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "running"}
	defer useFakeDocker(runner)()

	svc := PasswordService{Report: nopReporter{}}
	if err := svc.ChangePasswordNonInteractive("c1", ""); err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestPasswordChangeNonInteractiveFailsWhenNotRunning(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "exited"}
	defer useFakeDocker(runner)()

	svc := PasswordService{Report: nopReporter{}}
	if err := svc.ChangePasswordNonInteractive("c1", "pass"); err == nil {
		t.Fatal("expected error when container not running")
	}
}

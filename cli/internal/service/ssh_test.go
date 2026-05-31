package service

import (
	"slices"
	"strings"
	"testing"
)

func TestSshContainerRunning(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "alpha\nmyws-devcontainer\nbeta"}
	defer useFakeDocker(runner)()

	svc := SshService{Report: nopReporter{}}
	if !svc.ContainerRunning("myws-devcontainer") {
		t.Error("expected myws-devcontainer to be reported running")
	}
	if svc.ContainerRunning("absent") {
		t.Error("did not expect absent container to be running")
	}
}

func TestSshComposeUp(t *testing.T) {
	runner := &fakeRunner{status: 0}
	restore := useFakeDocker(runner)
	svc := SshService{Report: nopReporter{}}
	if err := svc.ComposeUp("compose.yml"); err != nil {
		t.Errorf("ComposeUp ok: %v", err)
	}
	if call := runner.callContaining("up"); call == nil || !slices.Contains(call, "-d") {
		t.Errorf("expected compose up -d; calls=%v", runner.calls)
	}
	restore()

	failRunner := &fakeRunner{status: 1}
	defer useFakeDocker(failRunner)()
	if err := (SshService{Report: nopReporter{}}).ComposeUp("compose.yml"); err == nil {
		t.Error("expected error when compose up fails")
	}
}

func TestSshServiceLogs(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "line1"}
	defer useFakeDocker(runner)()

	svc := SshService{Report: nopReporter{}}
	combined, ok := svc.ServiceLogs("compose.yml", "ssh")
	if !ok {
		t.Fatal("expected logs read to succeed")
	}
	if !strings.Contains(combined, "line1") {
		t.Errorf("combined logs = %q, want it to contain line1", combined)
	}
}

func TestSshInstallKeyLocal(t *testing.T) {
	okRunner := &fakeRunner{status: 0}
	restore := useFakeDocker(okRunner)
	svc := SshService{Report: nopReporter{}}
	if err := svc.InstallKeyLocal([]byte("k"), "devuser", "c1", "echo"); err != nil {
		t.Errorf("InstallKeyLocal ok: %v", err)
	}
	restore()

	failRunner := &fakeRunner{status: 1}
	defer useFakeDocker(failRunner)()
	if err := (SshService{Report: nopReporter{}}).InstallKeyLocal([]byte("k"), "devuser", "c1", "echo"); err == nil {
		t.Error("expected error when docker exec fails")
	}
}

func TestSshCommandExists(t *testing.T) {
	svc := SshService{Report: nopReporter{}}
	if !svc.CommandExists("go") {
		t.Error("expected 'go' to be on PATH in the test environment")
	}
	if svc.CommandExists("definitely-not-a-real-binary-xyz") {
		t.Error("did not expect a bogus binary to exist")
	}
}

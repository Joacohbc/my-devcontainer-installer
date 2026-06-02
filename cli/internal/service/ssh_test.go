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

func TestSshContainerIPs(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "net1 172.18.0.2\nnet2 172.19.0.2\nbroken\nempty \n"}
	defer useFakeDocker(runner)()

	svc := SshService{Report: nopReporter{}}
	entries, err := svc.ContainerIPs("c1")
	if err != nil {
		t.Fatalf("ContainerIPs ok: %v", err)
	}
	want := []NetworkIP{{Network: "net1", IP: "172.18.0.2"}, {Network: "net2", IP: "172.19.0.2"}}
	if !slices.Equal(entries, want) {
		t.Errorf("ContainerIPs = %v, want %v", entries, want)
	}

	failRunner := &fakeRunner{status: 1}
	defer useFakeDocker(failRunner)()
	if _, err := (SshService{Report: nopReporter{}}).ContainerIPs("c1"); err == nil {
		t.Error("expected error when docker inspect fails")
	}
}

func TestSshPasswordFromLogs(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "starting\ndevuser password: old\ndevuser password: new\nready"}
	defer useFakeDocker(runner)()

	svc := SshService{Report: nopReporter{}}
	line, ok := svc.PasswordFromLogs("compose.yml", "ssh", "devuser")
	if !ok {
		t.Fatal("expected a password line to be found")
	}
	if !strings.Contains(line, "new") {
		t.Errorf("PasswordFromLogs = %q, want the last match (new)", line)
	}

	noneRunner := &fakeRunner{status: 0, stdout: "no secrets here"}
	defer useFakeDocker(noneRunner)()
	if _, ok := (SshService{Report: nopReporter{}}).PasswordFromLogs("compose.yml", "ssh", "devuser"); ok {
		t.Error("did not expect a password line when logs have none")
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

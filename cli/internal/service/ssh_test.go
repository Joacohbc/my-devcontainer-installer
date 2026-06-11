package service

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSshEnsureKeyAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_devcontainer")
	if err := os.WriteFile(keyPath, []byte("private"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte("public"), 0o644); err != nil {
		t.Fatalf("write pub: %v", err)
	}

	svc := SshService{Report: nopReporter{}}
	created, err := svc.EnsureKey(keyPath)
	if err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	if created {
		t.Error("expected created=false when both key files already exist")
	}
}

func TestSshPublicAndPrivateKeyReads(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_devcontainer")
	if err := os.WriteFile(keyPath, []byte("PRIV"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte("PUB"), 0o644); err != nil {
		t.Fatalf("write pub: %v", err)
	}

	svc := SshService{Report: nopReporter{}}
	priv, err := svc.PrivateKey(keyPath)
	if err != nil || string(priv) != "PRIV" {
		t.Errorf("PrivateKey = %q, %v; want PRIV", priv, err)
	}
	pub, err := svc.PublicKey(keyPath)
	if err != nil || string(pub) != "PUB" {
		t.Errorf("PublicKey = %q, %v; want PUB", pub, err)
	}
}

func TestSshContainerRunning(t *testing.T) {
	stdout := `{"Names":"alpha","Image":"img1","Status":"Up","State":"running","Labels":"","Ports":""}
{"Names":"myws-devcontainer","Image":"img2","Status":"Up","State":"running","Labels":"","Ports":""}
{"Names":"stopped-one","Image":"img3","Status":"Exited (0)","State":"exited","Labels":"","Ports":""}`
	runner := &fakeRunner{status: 0, stdout: stdout}
	defer useFakeDocker(runner)()

	svc := SshService{Report: nopReporter{}}
	if !svc.ContainerRunning("myws-devcontainer") {
		t.Error("expected myws-devcontainer to be reported running")
	}
	if svc.ContainerRunning("absent") {
		t.Error("did not expect absent container to be running")
	}
	// Now that the list includes stopped containers, a non-running one must not
	// be reported as running.
	if svc.ContainerRunning("stopped-one") {
		t.Error("did not expect a stopped container to be reported running")
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

func TestSshInstallKeyLocal(t *testing.T) {
	okRunner := &fakeRunner{status: 0}
	restore := useFakeDocker(okRunner)
	svc := SshService{Report: nopReporter{}}
	err := svc.InstallKeyLocal(InstallKeySpec{
		PublicKey: []byte("k"),
		User:      "devuser",
		Container: "c1",
		Script:    "echo",
	})
	if err != nil {
		t.Errorf("InstallKeyLocal ok: %v", err)
	}
	restore()

	failRunner := &fakeRunner{status: 1}
	defer useFakeDocker(failRunner)()
	err = (SshService{Report: nopReporter{}}).InstallKeyLocal(InstallKeySpec{
		PublicKey: []byte("k"),
		User:      "devuser",
		Container: "c1",
		Script:    "echo",
	})
	if err == nil {
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
	if !slices.EqualFunc(entries, want, func(a, b NetworkIP) bool {
		return a.Network == b.Network && a.IP == b.IP && slices.Equal(a.Aliases, b.Aliases)
	}) {
		t.Errorf("ContainerIPs = %v, want %v", entries, want)
	}

	failRunner := &fakeRunner{status: 1}
	defer useFakeDocker(failRunner)()
	if _, err := (SshService{Report: nopReporter{}}).ContainerIPs("c1"); err == nil {
		t.Error("expected error when docker inspect fails")
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

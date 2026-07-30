package service

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeFakeSSH puts an executable "ssh" shell script (running body) at the
// front of PATH, so tests can control what a real `exec.Command("ssh", ...)`
// call does without depending on an actual ssh binary or network.
func writeFakeSSH(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

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

// cmdRoutingRunner answers "docker ps" and "docker inspect" with distinct
// canned outputs so ContainerLiveness (which calls both) can be exercised.
type cmdRoutingRunner struct {
	psOut      string
	inspectOut string
	psStatus   int
}

func (r *cmdRoutingRunner) Run(_ context.Context, args []string, _ string, _ string, _ map[string]string) (int, string, string) {
	if len(args) >= 2 && args[1] == "version" {
		return 0, "27.0.0", ""
	}
	for _, a := range args {
		switch a {
		case "ps":
			return r.psStatus, r.psOut, ""
		case "inspect":
			return 0, r.inspectOut, ""
		}
	}
	return 0, "", ""
}

func TestSshContainerLiveness(t *testing.T) {
	ps := `{"Names":"alpha","Image":"img1","Status":"Up","State":"running","Labels":"","Ports":""}
{"Names":"stopped-one","Image":"img3","Status":"Exited (0)","State":"exited","Labels":"","Ports":""}`

	t.Run("running returns its first IP", func(t *testing.T) {
		runner := &cmdRoutingRunner{psOut: ps, inspectOut: "bridge 172.25.0.14\n"}
		defer useFakeDocker(runner)()
		state, ip, err := (SshService{Report: nopReporter{}}).ContainerLiveness("alpha")
		if err != nil || state != TargetRunning || ip != "172.25.0.14" {
			t.Errorf("ContainerLiveness(alpha) = %v,%q,%v, want running,172.25.0.14,nil", state, ip, err)
		}
	})

	t.Run("stopped", func(t *testing.T) {
		runner := &cmdRoutingRunner{psOut: ps}
		defer useFakeDocker(runner)()
		state, ip, err := (SshService{Report: nopReporter{}}).ContainerLiveness("stopped-one")
		if err != nil || state != TargetStopped || ip != "" {
			t.Errorf("ContainerLiveness(stopped-one) = %v,%q,%v, want stopped,\"\",nil", state, ip, err)
		}
	})

	t.Run("absent", func(t *testing.T) {
		runner := &cmdRoutingRunner{psOut: ps}
		defer useFakeDocker(runner)()
		state, _, err := (SshService{Report: nopReporter{}}).ContainerLiveness("gone")
		if err != nil || state != TargetAbsent {
			t.Errorf("ContainerLiveness(gone) = %v,%v, want absent,nil", state, err)
		}
	})

	t.Run("unreachable daemon is an error, not absent", func(t *testing.T) {
		runner := &cmdRoutingRunner{psStatus: 1}
		defer useFakeDocker(runner)()
		_, _, err := (SshService{Report: nopReporter{}}).ContainerLiveness("alpha")
		if err == nil {
			t.Error("ContainerLiveness with an unreachable daemon = nil error, want an error so the target is not treated as stale")
		}
	})
}

// hostAwareRunner answers "docker ps" differently depending on whether the
// call carries a DOCKER_HOST override, so tests can exercise
// LiveTargetPredicates' existsRemoteContainer path without a real remote
// daemon: remoteFail simulates the jump host being unreachable right now.
type hostAwareRunner struct {
	localStdout  string
	remoteStdout string
	remoteFail   bool
}

func (r *hostAwareRunner) Run(_ context.Context, args []string, _ string, _ string, env map[string]string) (int, string, string) {
	if len(args) >= 2 && args[1] == "version" {
		return 0, "27.0.0", ""
	}
	if env["DOCKER_HOST"] != "" {
		if r.remoteFail {
			return 1, "", "connection refused"
		}
		return 0, r.remoteStdout, ""
	}
	return 0, r.localStdout, ""
}

func TestLiveTargetPredicates_ExistsRemoteContainer(t *testing.T) {
	runner := &hostAwareRunner{
		localStdout:  `{"Names":"local-ctr","Image":"img","Status":"Up","State":"running","Labels":"","Ports":""}`,
		remoteStdout: `{"Names":"remote-ctr","Image":"img","Status":"Up","State":"running","Labels":"","Ports":""}`,
	}
	defer useFakeDocker(runner)()

	svc := SshService{Report: nopReporter{}}
	_, existsContainer, existsRemoteContainer, err := svc.LiveTargetPredicates()
	if err != nil {
		t.Fatalf("LiveTargetPredicates: %v", err)
	}

	if !existsContainer("local-ctr") {
		t.Error("expected local-ctr to be found by the local predicate")
	}
	if existsContainer("remote-ctr") {
		t.Error("did not expect the local predicate to see a container that only exists remotely")
	}
	if alive, verified := existsRemoteContainer("rbpi", "remote-ctr"); !alive || !verified {
		t.Errorf("expected remote-ctr to be found on rbpi (alive=%v verified=%v)", alive, verified)
	}
	if alive, verified := existsRemoteContainer("rbpi", "nope"); alive || !verified {
		t.Errorf("expected a container absent from rbpi to be reported dead but verified (alive=%v verified=%v)", alive, verified)
	}
}

// An unreachable --via jump host must not cause its blocks to be treated as
// stale (alive=true) — clean-ssh could otherwise delete valid SSH access just
// because the remote happened to be down during this run — but it must also
// be reported unverified (verified=false) so the caller can still surface it
// and let the user remove it explicitly if they know it really is gone.
func TestLiveTargetPredicates_ExistsRemoteContainer_UnreachableIsUnverified(t *testing.T) {
	runner := &hostAwareRunner{remoteFail: true}
	defer useFakeDocker(runner)()

	svc := SshService{Report: nopReporter{}}
	_, _, existsRemoteContainer, err := svc.LiveTargetPredicates()
	if err != nil {
		t.Fatalf("LiveTargetPredicates: %v", err)
	}
	alive, verified := existsRemoteContainer("unreachable-host", "whatever")
	if !alive {
		t.Error("expected fail-open (reported alive) when the remote host cannot be reached")
	}
	if verified {
		t.Error("expected verified=false when the remote host cannot be reached")
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

func TestSshConnect(t *testing.T) {
	writeFakeSSH(t, "exit 0")
	svc := SshService{Report: nopReporter{}}
	if err := svc.Connect("myalias", nil); err != nil {
		t.Errorf("Connect ok: %v", err)
	}
}

func TestSshConnectPropagatesExitCode(t *testing.T) {
	writeFakeSSH(t, "exit 7")
	svc := SshService{Report: nopReporter{}}
	err := svc.Connect("myalias", []string{"go", "version"})
	if err == nil || !strings.Contains(err.Error(), "code 7") {
		t.Errorf("Connect exit code = %v, want error containing 'code 7'", err)
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

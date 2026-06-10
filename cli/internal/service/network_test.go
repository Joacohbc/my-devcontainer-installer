package service

import (
	"context"
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

func TestResolveNetwork_returnsManagedName(t *testing.T) {
	r := &fakeRunner{status: 0, stdout: "myws_myws-network\n"}
	defer useFakeDocker(r)()

	got, err := NetworkService{Report: nopReporter{}}.ResolveNetwork("myws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "myws_myws-network" {
		t.Errorf("network = %q, want %q", got, "myws_myws-network")
	}

	call := r.callContaining("ls")
	if call == nil {
		t.Fatalf("expected a 'docker network ls' call, got %v", r.calls)
	}
	if !slices.Contains(call, "label=com.docker.compose.project=myws") {
		t.Errorf("ls call missing compose project filter: %v", call)
	}
	if !slices.Contains(call, managedFilter) {
		t.Errorf("ls call missing managed filter: %v", call)
	}
}

func TestResolveNetwork_errorsWhenMissing(t *testing.T) {
	r := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(r)()

	if _, err := (NetworkService{Report: nopReporter{}}).ResolveNetwork("myws"); err == nil {
		t.Fatal("expected error when no managed network exists")
	}
}

func TestResolveNetwork_picksConventionalNameOnTie(t *testing.T) {
	r := &fakeRunner{status: 0, stdout: "other-net\nmyws_myws-network\n"}
	defer useFakeDocker(r)()

	got, err := NetworkService{Report: nopReporter{}}.ResolveNetwork("myws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "myws_myws-network" {
		t.Errorf("network = %q, want the <workspace>-network suffix match", got)
	}
}

func TestConnect_runsDockerNetworkConnect(t *testing.T) {
	r := &fakeRunner{status: 0, stdout: "myws-network\n"}
	defer useFakeDocker(r)()

	ok, failed, err := NetworkService{Report: nopReporter{}}.Connect("myws", []string{"app", "db"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok != 2 || failed != 0 {
		t.Fatalf("ok=%d failed=%d, want 2/0", ok, failed)
	}

	call := r.callContaining("connect")
	if call == nil {
		t.Fatalf("expected a 'docker network connect' call, got %v", r.calls)
	}
	if !slices.Contains(call, "myws-network") || !slices.Contains(call, "app") {
		t.Errorf("connect call missing network or container: %v", call)
	}
}

func TestDisconnect_runsDockerNetworkDisconnect(t *testing.T) {
	r := &fakeRunner{status: 0, stdout: "myws-network\n"}
	defer useFakeDocker(r)()

	ok, failed, err := NetworkService{Report: nopReporter{}}.Disconnect("myws", []string{"app"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok != 1 || failed != 0 {
		t.Fatalf("ok=%d failed=%d, want 1/0", ok, failed)
	}
	if r.callContaining("disconnect") == nil {
		t.Fatalf("expected a 'docker network disconnect' call, got %v", r.calls)
	}
}

// verbStatusRunner replies 0 for the resolving "network ls" query but a failing
// status for the connect/disconnect verb, so the failure tally can be exercised.
type verbStatusRunner struct {
	network  string
	failVerb string
}

func (r verbStatusRunner) Run(_ context.Context, args []string, _ string, _ string, _ map[string]string) (int, string, string) {
	if len(args) >= 2 && args[1] == "version" {
		return 0, "27.0.0", ""
	}
	if slices.Contains(args, "ls") {
		return 0, r.network + "\n", ""
	}
	if slices.Contains(args, r.failVerb) {
		return 1, "", "Error: no such container"
	}
	return 0, "", ""
}

func TestConnect_countsFailures(t *testing.T) {
	docker.SetRunner(verbStatusRunner{network: "myws-network", failVerb: "connect"})
	docker.ResetDockerCache()
	defer func() { docker.ResetRunner(); docker.ResetDockerCache() }()

	ok, failed, err := NetworkService{Report: nopReporter{}}.Connect("myws", []string{"ghost", "phantom"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok != 0 || failed != 2 {
		t.Errorf("ok=%d failed=%d, want 0/2", ok, failed)
	}
}

func TestContainerNames_listsAll(t *testing.T) {
	r := &fakeRunner{status: 0, stdout: "app\ndb\nredis\n"}
	defer useFakeDocker(r)()

	got := NetworkService{Report: nopReporter{}}.ContainerNames()
	want := []string{"app", "db", "redis"}
	if !slices.Equal(got, want) {
		t.Errorf("ContainerNames = %v, want %v", got, want)
	}
}

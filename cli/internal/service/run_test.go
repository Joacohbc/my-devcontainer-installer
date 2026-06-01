package service

import (
	"slices"
	"strings"
	"testing"
)

func TestRunCreatesWhenAbsent(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	err := svc.Run(QuickRunSpec{Variant: "nodejs", ContainerName: "dc-nodejs", Image: "ghcr.io/x/devcontainer-nodejs:latest"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	call := runner.callContaining("run")
	if call == nil || !slices.Contains(call, "--name") || !slices.Contains(call, "dc-nodejs") {
		t.Fatalf("unexpected run call: %v", call)
	}
	if !slices.Contains(call, "ghcr.io/x/devcontainer-nodejs:latest") {
		t.Errorf("run call missing image: %v", call)
	}
}

func TestRunStartsExisting(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "exited"}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	if err := svc.Run(QuickRunSpec{ContainerName: "dc-ssh"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if call := runner.callContaining("start"); call == nil || !slices.Contains(call, "dc-ssh") {
		t.Errorf("expected start dc-ssh; calls=%v", runner.calls)
	}
	if runner.callContaining("run") != nil {
		t.Errorf("must not docker run an existing container")
	}
}

func TestRunAlreadyRunningIsNoop(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "running"}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	if err := svc.Run(QuickRunSpec{ContainerName: "dc-ssh"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if runner.callContaining("run") != nil || runner.callContaining("start") != nil {
		t.Errorf("running container should not be started or recreated; calls=%v", runner.calls)
	}
}

func TestRunMountsVolumes(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	err := svc.Run(QuickRunSpec{
		ContainerName: "dc-ssh",
		Image:         "img",
		Volumes:       []string{"myvol:/workspace", "data:/data"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	call := runner.callContaining("run")
	if call == nil {
		t.Fatal("expected docker run call")
	}
	for _, v := range []string{"myvol:/workspace", "data:/data"} {
		if !slices.Contains(call, v) {
			t.Errorf("run call missing volume mount %q: %v", v, call)
		}
	}
}

func TestRunDoesNotMountDockerSocket(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	err := svc.Run(QuickRunSpec{
		ContainerName: "dc-ssh",
		Image:         "img",
		Volumes:       []string{"myvol:/workspace"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	call := runner.callContaining("run")
	if call == nil {
		t.Fatal("expected docker run call")
	}
	for _, arg := range call {
		if strings.Contains(arg, "docker.sock") {
			t.Errorf("docker.sock must never be mounted by default; calls=%v", call)
		}
	}
}

func TestRunMapsPorts(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	err := svc.Run(QuickRunSpec{
		ContainerName: "dc-ssh",
		Image:         "img",
		Ports:         []string{"2222:22", "8080:80"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	call := runner.callContaining("run")
	if call == nil {
		t.Fatal("expected docker run call")
	}
	for _, p := range []string{"2222:22", "8080:80"} {
		if !slices.Contains(call, p) {
			t.Errorf("run call missing port mapping %q: %v", p, call)
		}
	}
}

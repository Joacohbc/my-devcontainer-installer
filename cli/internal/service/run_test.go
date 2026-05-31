package service

import (
	"slices"
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

func TestRunMountsVolume(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	if err := svc.Run(QuickRunSpec{ContainerName: "dc-ssh", Image: "img", Volume: "myvol"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if call := runner.callContaining("volume"); call == nil || !slices.Contains(call, "create") {
		t.Errorf("expected volume create; calls=%v", runner.calls)
	}
	if call := runner.callContaining("run"); call == nil || !slices.Contains(call, "myvol:/workspace") {
		t.Errorf("run call missing volume mount: %v", call)
	}
}

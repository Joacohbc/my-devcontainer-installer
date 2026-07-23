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
	runner := &fakeRunner{status: 0, stdout: "exited|ssh"}
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
	runner := &fakeRunner{status: 0, stdout: "running|ssh"}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	if err := svc.Run(QuickRunSpec{ContainerName: "dc-ssh"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if runner.callContaining("run") != nil || runner.callContaining("start") != nil {
		t.Errorf("running container should not be started or recreated; calls=%v", runner.calls)
	}
}

func TestRunRejectsNameOfUnmanagedContainer(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "running|"}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	err := svc.Run(QuickRunSpec{ContainerName: "postgres", Image: "img"})
	if err == nil {
		t.Fatal("expected an error when the name collides with a non quick-run container")
	}
	if !strings.Contains(err.Error(), "not created by 'run'") {
		t.Errorf("unexpected error message: %v", err)
	}
	if runner.callContaining("run") != nil || runner.callContaining("start") != nil {
		t.Errorf("must not start or recreate a container it doesn't own; calls=%v", runner.calls)
	}
}

func TestValidateContainerName(t *testing.T) {
	cases := []struct {
		name    string
		stdout  string
		wantErr bool
	}{
		{"absent", "", false},
		{"existing quick-run container", "running|ssh", false},
		{"existing quick-run container stopped", "exited|nodejs", false},
		{"existing unmanaged container", "running|", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runner := &fakeRunner{status: 0, stdout: c.stdout}
			defer useFakeDocker(runner)()

			svc := RunService{Report: nopReporter{}}
			err := svc.ValidateContainerName("dc-ssh")
			if c.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
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

func TestRunMountsSharedConfig(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	err := svc.Run(QuickRunSpec{
		ContainerName: "dc-ssh",
		Image:         "img",
		SharedConfig:  true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	call := runner.callContaining("run")
	if call == nil {
		t.Fatal("expected docker run call")
	}
	if !slices.Contains(call, "devcontainer-shared-config:/mnt/shared-config") {
		t.Errorf("run call missing shared-config mount: %v", call)
	}
}

func TestRunNoSharedConfigByDefault(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	err := svc.Run(QuickRunSpec{ContainerName: "dc-ssh", Image: "img"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	call := runner.callContaining("run")
	if call == nil {
		t.Fatal("expected docker run call")
	}
	if slices.Contains(call, "devcontainer-shared-config:/mnt/shared-config") {
		t.Errorf("shared-config mount must be absent when SharedConfig=false: %v", call)
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

func TestRunMapsPortsBoundToLoopback(t *testing.T) {
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
	for _, p := range []string{"127.0.0.1:2222:22", "127.0.0.1:8080:80"} {
		if !slices.Contains(call, p) {
			t.Errorf("run call missing loopback-bound port mapping %q: %v", p, call)
		}
	}
	for _, p := range []string{"2222:22", "8080:80"} {
		if slices.Contains(call, p) {
			t.Errorf("port mapping %q must be bound to 127.0.0.1, not published raw: %v", p, call)
		}
	}
}

func TestRunCopyAIScripts(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "running"}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	if err := svc.CopyAIScripts("c1", []string{"install-claude-code"}); err != nil {
		t.Fatalf("CopyAIScripts: %v", err)
	}

	cp := runner.callContaining("cp")
	if cp == nil {
		t.Fatalf("expected a docker cp call; calls=%v", runner.calls)
	}
	if !slices.Contains(cp, "c1:/home/devuser/install-claude-code.sh") {
		t.Errorf("cp must target the devuser home dest: %v", cp)
	}

	// The copied script is left owned by devuser and executable.
	var chowned bool
	for _, call := range runner.calls {
		for _, arg := range call {
			if strings.Contains(arg, "chown devuser:devuser") && strings.Contains(arg, "chmod +x") {
				chowned = true
			}
		}
	}
	if !chowned {
		t.Errorf("expected a chown devuser:devuser + chmod +x call; calls=%v", runner.calls)
	}
}

func TestRunCopyAIScriptsNoopWhenEmpty(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "running"}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	if err := svc.CopyAIScripts("c1", nil); err != nil {
		t.Fatalf("CopyAIScripts: %v", err)
	}
	if runner.callContaining("cp") != nil {
		t.Errorf("no copy should happen for an empty list; calls=%v", runner.calls)
	}
}

func TestRunMapsPortsExposeAll(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(runner)()

	svc := RunService{Report: nopReporter{}}
	err := svc.Run(QuickRunSpec{
		ContainerName: "dc-ssh",
		Image:         "img",
		Ports:         []string{"2222:22", "8080:80"},
		ExposeAll:     true,
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
			t.Errorf("with ExposeAll the raw port mapping %q must be published: %v", p, call)
		}
	}
	for _, arg := range call {
		if strings.HasPrefix(arg, "127.0.0.1:") {
			t.Errorf("ExposeAll must not bind to 127.0.0.1: %v", call)
		}
	}
}

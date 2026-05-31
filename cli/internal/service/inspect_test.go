package service

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestInspectContainerStateAndImage(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "running"}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	state, err := svc.ContainerState("c1")
	if err != nil || state != "running" {
		t.Fatalf("ContainerState = %q, %v; want running", state, err)
	}
	if img := svc.ContainerImage("c1"); img != "running" {
		t.Errorf("ContainerImage = %q (mock echoes stdout)", img)
	}
}

func TestInspectShellBuildsExecArgs(t *testing.T) {
	cases := []struct {
		name     string
		user     string
		command  []string
		wantUser bool
	}{
		{"default shell", "", nil, false},
		{"as root with cmd", "root", []string{"echo", "hi"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runner := &fakeRunner{status: 0, stdout: "running"}
			defer useFakeDocker(runner)()

			svc := InspectService{Report: nopReporter{}}
			if err := svc.Shell("c1", c.user, c.command); err != nil {
				t.Fatalf("Shell: %v", err)
			}
			call := runner.callContaining("exec")
			if call == nil || !slices.Contains(call, "-it") || !slices.Contains(call, "c1") {
				t.Fatalf("unexpected exec call: %v", call)
			}
			if slices.Contains(call, "-u") != c.wantUser {
				t.Errorf("-u presence = %v, want %v (call %v)", !c.wantUser, c.wantUser, call)
			}
		})
	}
}

func TestInspectShellFailsWhenNotRunning(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "exited"}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	if err := svc.Shell("c1", "", nil); err == nil {
		t.Error("expected error when container is not running")
	}
}

func TestInspectLsArgs(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "running"}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	if err := svc.Ls("c1", "/tmp", true, true); err != nil {
		t.Fatalf("Ls: %v", err)
	}
	call := runner.callContaining("ls")
	if call == nil || !slices.Contains(call, "-l") || !slices.Contains(call, "-a") || !slices.Contains(call, "/tmp") {
		t.Errorf("unexpected ls call: %v", call)
	}
}

func TestInspectCopy(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "running"}
	defer useFakeDocker(runner)()

	local := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(local, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := InspectService{Report: nopReporter{}}
	if err := svc.Copy("c1", local, "/dest"); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if call := runner.callContaining("cp"); call == nil || !slices.Contains(call, "c1:/dest") {
		t.Errorf("unexpected cp call: %v", call)
	}

	if err := svc.Copy("c1", filepath.Join(t.TempDir(), "missing"), "/dest"); err == nil {
		t.Error("expected error for missing local path")
	}
}

func TestInspectComposeLogsArgs(t *testing.T) {
	runner := &fakeRunner{status: 0}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	if err := svc.ComposeLogs("compose.yml", true, "100", []string{"web"}); err != nil {
		t.Fatalf("ComposeLogs: %v", err)
	}
	call := runner.callContaining("logs")
	if call == nil || !slices.Contains(call, "--follow") || !slices.Contains(call, "--tail") || !slices.Contains(call, "web") {
		t.Errorf("unexpected compose logs call: %v", call)
	}
}

func TestInspectListDir(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "a\nb/\n\nc"}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	entries, err := svc.ListDir("c1", "/")
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	want := []string{"a", "b/", "c"}
	if !slices.Equal(entries, want) {
		t.Errorf("entries = %v, want %v", entries, want)
	}
}

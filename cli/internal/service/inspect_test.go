package service

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestInspectCopyAsset(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "running"}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}

	// Unknown asset must fail before touching docker.
	if err := svc.CopyAsset("c1", "not-a-real-asset", ""); err == nil {
		t.Error("expected error for unknown asset")
	}

	if err := svc.CopyAsset("c1", "install-claude-code", ""); err != nil {
		t.Fatalf("CopyAsset: %v", err)
	}
	cp := runner.callContaining("cp")
	if cp == nil || !slices.Contains(cp, "c1:/home/devuser/install-claude-code.sh") {
		t.Errorf("expected cp into devuser home, got %v", cp)
	}
	// A follow-up exec must fix ownership/permissions.
	exec := runner.callContaining("exec")
	if exec == nil || !strings.Contains(strings.Join(exec, " "), "chown devuser:devuser") || !strings.Contains(strings.Join(exec, " "), "chmod +x") {
		t.Errorf("expected chown/chmod exec call, got calls %v", runner.calls)
	}
}

func TestInspectCopyAssetCustomDest(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "running"}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	if err := svc.CopyAsset("c1", "install-opencode", "/tmp/oc.sh"); err != nil {
		t.Fatalf("CopyAsset: %v", err)
	}
	if cp := runner.callContaining("cp"); cp == nil || !slices.Contains(cp, "c1:/tmp/oc.sh") {
		t.Errorf("expected cp to custom dest, got %v", cp)
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

func TestInspectContainerDetails(t *testing.T) {
	jsonOut := `{
		"Name": "/mycontainer",
		"Config": {"Image": "myimage:latest"},
		"State": {"Status": "running", "StartedAt": "2024-05-15T10:30:05.000000000Z"},
		"Created": "2024-05-15T10:30:00.000000000Z",
		"Mounts": [
			{"Type": "volume", "Name": "myvol", "Source": "/var/lib/docker/volumes/myvol/_data", "Destination": "/home"},
			{"Type": "bind", "Source": "/var/run/docker.sock", "Destination": "/var/run/docker.sock"}
		],
		"NetworkSettings": {
			"Ports": {"22/tcp": [{"HostPort": "2222"}]},
			"Networks": {"bridge": {"IPAddress": "172.17.0.2"}}
		}
	}`
	runner := &fakeRunner{status: 0, stdout: jsonOut}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	info, err := svc.ContainerDetails("mycontainer")
	if err != nil {
		t.Fatalf("ContainerDetails: %v", err)
	}
	if info.Name != "mycontainer" {
		t.Errorf("Name = %q, want mycontainer", info.Name)
	}
	if info.Image != "myimage:latest" {
		t.Errorf("Image = %q", info.Image)
	}
	if info.Status != "running" {
		t.Errorf("Status = %q", info.Status)
	}
	if info.Created.Year() != 2024 {
		t.Errorf("Created year = %d", info.Created.Year())
	}
	if info.StartedAt.Year() != 2024 {
		t.Errorf("StartedAt year = %d", info.StartedAt.Year())
	}
	if len(info.Volumes) != 2 {
		t.Fatalf("Volumes = %v, want 2 entries", info.Volumes)
	}
	if len(info.Ports) != 1 || info.Ports[0] != "2222:22/tcp" {
		t.Errorf("Ports = %v, want [2222:22/tcp]", info.Ports)
	}
	if len(info.IPs) != 1 || info.IPs[0] != "bridge 172.17.0.2" {
		t.Errorf("IPs = %v, want [bridge 172.17.0.2]", info.IPs)
	}
}

func TestInspectContainerDetailsNotFound(t *testing.T) {
	runner := &fakeRunner{status: 1}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	if _, err := svc.ContainerDetails("missing"); err == nil {
		t.Error("expected error for missing container")
	}
}

func TestInspectContainerNetworkIPs(t *testing.T) {
	cases := []struct {
		name    string
		stdout  string
		status  int
		wantLen int
		wantErr bool
	}{
		{
			name:    "single network",
			stdout:  "bridge 172.17.0.2\n",
			status:  0,
			wantLen: 1,
		},
		{
			name:    "multiple networks",
			stdout:  "bridge 172.17.0.2\nmynet 10.0.0.5\n",
			status:  0,
			wantLen: 2,
		},
		{
			name:    "docker error",
			stdout:  "",
			status:  1,
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runner := &fakeRunner{status: c.status, stdout: c.stdout}
			defer useFakeDocker(runner)()

			svc := InspectService{Report: nopReporter{}}
			ips, err := svc.ContainerNetworkIPs("c1")
			if c.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ContainerNetworkIPs: %v", err)
			}
			if len(ips) != c.wantLen {
				t.Fatalf("got %d entries, want %d: %v", len(ips), c.wantLen, ips)
			}
		})
	}
}

func TestInspectListUsers(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "root\ndaemon\ndevuser\n"}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	users := svc.ListUsers("c1")
	want := []string{"root", "daemon", "devuser"}
	if !slices.Equal(users, want) {
		t.Errorf("ListUsers = %v, want %v", users, want)
	}
}

func TestInspectListUsersUnavailable(t *testing.T) {
	runner := &fakeRunner{status: 1}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	if users := svc.ListUsers("c1"); users != nil {
		t.Errorf("expected nil on docker error, got %v", users)
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

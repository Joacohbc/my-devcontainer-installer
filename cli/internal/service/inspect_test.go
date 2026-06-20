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
		name      string
		user      string
		shellType string
		command   []string
		noTTY     bool
		wantUser  bool
	}{
		{"default shell", "", "", nil, false, false},
		{"typed shell", "devuser", "zsh", nil, false, true},
		{"as root with cmd", "root", "", []string{"echo", "hi"}, false, true},
		{"no-tty with cmd", "", "", []string{"pg_dump", "devdb"}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runner := &fakeRunner{status: 0, stdout: "running"}
			defer useFakeDocker(runner)()

			svc := InspectService{Report: nopReporter{}}
			if err := svc.Shell("c1", c.user, c.shellType, c.command, c.noTTY); err != nil {
				t.Fatalf("Shell: %v", err)
			}
			call := runner.callContaining("exec")
			if call == nil || !slices.Contains(call, "c1") {
				t.Fatalf("unexpected exec call: %v", call)
			}
			// stdin stays open in both modes (-i); the TTY (-t) is dropped only
			// when noTTY is set, so a redirected stream isn't mangled.
			if !slices.Contains(call, "-i") {
				t.Errorf("expected -i in exec call: %v", call)
			}
			if hasTTY := slices.Contains(call, "-t"); hasTTY == c.noTTY {
				t.Errorf("-t presence = %v, want %v (noTTY=%v, call %v)", hasTTY, !c.noTTY, c.noTTY, call)
			}
			if slices.Contains(call, "-u") != c.wantUser {
				t.Errorf("-u presence = %v, want %v (call %v)", !c.wantUser, c.wantUser, call)
			}
			// With no command, launch a login shell so profiles/rc files load.
			// HOME must be re-exported from the live passwd entry: docker exec
			// delivers a stale HOME=/ after the entrypoint's runtime UID remap.
			if len(c.command) == 0 {
				joined := strings.Join(call, " ")
				if !strings.Contains(joined, "exec \"$SH\" -l") || !strings.Contains(joined, "getent passwd") {
					t.Errorf("expected login-shell launcher, got %v", call)
				}
				if !strings.Contains(joined, `export HOME="$H"`) {
					t.Errorf("launcher must re-export HOME from passwd, got %v", call)
				}
				// The session must cd into the user's home so the interactive
				// shell opens there (devuser's home for the interactive default).
				if !strings.Contains(joined, `cd "$H"`) {
					t.Errorf("launcher must cd into the user's home, got %v", call)
				}
				if c.shellType != "" && !strings.Contains(joined, "command -v "+c.shellType) {
					t.Errorf("launcher must resolve the typed shell %q, got %v", c.shellType, call)
				}
			}
		})
	}
}

func TestInspectShellFailsWhenNotRunning(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "exited"}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	if err := svc.Shell("c1", "", "", nil, false); err == nil {
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

func TestInspectCopyFromContainer(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "running"}
	defer useFakeDocker(runner)()

	dest := filepath.Join(t.TempDir(), "out.log")
	svc := InspectService{Report: nopReporter{}}
	if err := svc.CopyFromContainer("c1", "/var/log/out.log", dest); err != nil {
		t.Fatalf("CopyFromContainer: %v", err)
	}
	call := runner.callContaining("cp")
	if call == nil || !slices.Contains(call, "c1:/var/log/out.log") || !slices.Contains(call, dest) {
		t.Errorf("unexpected cp call: %v", call)
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
			"Networks": {"bridge": {"IPAddress": "172.17.0.2", "Aliases": ["api", "web"]}}
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
	if info.Status != StateRunning {
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
	if len(info.Ports) != 1 || info.Ports[0].HostPort != "2222" || info.Ports[0].ContainerPort != "22" || info.Ports[0].Protocol != "tcp" {
		t.Errorf("Ports = %v, want [{2222 22 tcp}]", info.Ports)
	}
	if len(info.IPs) != 1 || info.IPs[0].Network != "bridge" || info.IPs[0].IP != "172.17.0.2" {
		t.Errorf("IPs = %v, want [{bridge 172.17.0.2}]", info.IPs)
	}
	if !slices.Equal(info.IPs[0].Aliases, []string{"api", "web"}) {
		t.Errorf("IPs[0].Aliases = %v, want [api web]", info.IPs[0].Aliases)
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

func TestParsePSLines(t *testing.T) {
	stdout := `{"Names":"ws1-devcontainer-ssh","Image":"img1","Status":"Up 2 minutes","State":"running","Labels":"a=b","Ports":"22/tcp"}
{"Names":"ws2-devcontainer-ssh","Image":"img2","Status":"Exited (0)","State":"exited","Labels":"","Ports":""}

not-json-should-be-skipped
{"Image":"no-name-skipped","State":"running"}
`
	lines := parsePSLines(stdout)
	if len(lines) != 2 {
		t.Fatalf("expected 2 parsed lines, got %d: %+v", len(lines), lines)
	}
	if lines[0].Names != "ws1-devcontainer-ssh" || lines[0].Image != "img1" || lines[0].State != "running" {
		t.Errorf("line[0] mis-parsed: %+v", lines[0])
	}
	if lines[1].Names != "ws2-devcontainer-ssh" || lines[1].State != "exited" {
		t.Errorf("line[1] mis-parsed: %+v", lines[1])
	}
}

func TestParsePSLines_Empty(t *testing.T) {
	if lines := parsePSLines("   \n\n"); len(lines) != 0 {
		t.Errorf("expected no lines for blank input, got %+v", lines)
	}
}

func TestInspectListManaged(t *testing.T) {
	stdout := `{"Names":"c1","Image":"img1","Status":"Up 2 minutes","State":"running","Labels":"dev.devcontainer-installer.managed=true","Ports":"22/tcp"}`
	runner := &fakeRunner{status: 0, stdout: stdout}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	containers := svc.ListManaged()
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	c := containers[0]
	if c.Name != "c1" || c.Image != "img1" || !c.Managed || c.State != "running" {
		t.Errorf("unexpected container: %+v", c)
	}
}

func TestInspectListAll(t *testing.T) {
	stdout := `{"Names":"c1","Image":"img1","Status":"Up 2 minutes","State":"running","Labels":"dev.devcontainer-installer.managed=true","Ports":"22/tcp"}
{"Names":"c2","Image":"img2","Status":"Up 5 minutes","State":"running","Labels":"other=label","Ports":"80/tcp"}`
	runner := &fakeRunner{status: 0, stdout: stdout}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	containers := svc.ListAll()
	if len(containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(containers))
	}
	if !containers[0].Managed {
		t.Errorf("expected container 0 to be managed")
	}
	if containers[1].Managed {
		t.Errorf("expected container 1 to not be managed")
	}
}

func TestInspectListContainers(t *testing.T) {
	stdout := `{"Names":"c1","Image":"img1","Status":"Up 2 minutes","State":"running","Labels":"dev.devcontainer-installer.managed=true","Ports":"22/tcp"}
{"Names":"c2","Image":"img2","Status":"Up 5 minutes","State":"running","Labels":"other=label","Ports":"80/tcp"}`
	runner := &fakeRunner{status: 0, stdout: stdout}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	containers, err := svc.ListContainers(false)
	if err != nil {
		t.Fatalf("ListContainers error: %v", err)
	}
	if len(containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(containers))
	}
	if !containers[0].Managed {
		t.Errorf("expected container 0 to be managed")
	}
	if containers[1].Managed {
		t.Errorf("expected container 1 to not be managed")
	}

	call := runner.callContaining("ps")
	if call == nil {
		t.Fatalf("expected docker ps call")
	}
	foundA := false
	for _, arg := range call {
		if arg == "-a" {
			foundA = true
			break
		}
	}
	if !foundA {
		t.Errorf("expected -a flag in docker ps call: %v", call)
	}
}

func TestInspectListContainersWorkspace(t *testing.T) {
	// A compose-managed devcontainer (workspace from the compose project label),
	// a quick-run standalone container, and an unmanaged container.
	stdout := `{"Names":"myws-devcontainer-ssh","Image":"img1","Status":"Up","State":"running","Labels":"dev.devcontainer-installer.managed=true,com.docker.compose.project=myws","Ports":""}
{"Names":"quick-node","Image":"img2","Status":"Exited (0)","State":"exited","Labels":"dev.devcontainer-installer.managed=true,dev.devcontainer-installer.quick-run=nodejs","Ports":""}
{"Names":"other","Image":"img3","Status":"Up","State":"running","Labels":"foo=bar","Ports":""}`
	runner := &fakeRunner{status: 0, stdout: stdout}
	defer useFakeDocker(runner)()

	svc := InspectService{Report: nopReporter{}}
	containers, err := svc.ListContainers(false)
	if err != nil {
		t.Fatalf("ListContainers error: %v", err)
	}
	if len(containers) != 3 {
		t.Fatalf("expected 3 containers, got %d", len(containers))
	}
	want := map[string]struct {
		workspace string
		managed   bool
	}{
		"myws-devcontainer-ssh": {"myws", true},
		"quick-node":            {"standalone", true},
		"other":                 {"", false},
	}
	for _, c := range containers {
		w, ok := want[c.Name]
		if !ok {
			t.Errorf("unexpected container %q", c.Name)
			continue
		}
		if c.Workspace != w.workspace {
			t.Errorf("%s: Workspace = %q, want %q", c.Name, c.Workspace, w.workspace)
		}
		if c.Managed != w.managed {
			t.Errorf("%s: Managed = %v, want %v", c.Name, c.Managed, w.managed)
		}
	}
}

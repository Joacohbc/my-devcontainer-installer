package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTunnel_ForwardSpec(t *testing.T) {
	cases := []struct {
		name string
		t    Tunnel
		want string
	}{
		{
			name: "forward default bind",
			t:    Tunnel{LocalPort: 3000, ContainerPort: 3000, TargetHost: "localhost"},
			want: "127.0.0.1:3000:localhost:3000",
		},
		{
			name: "forward custom bind",
			t:    Tunnel{BindAddress: "100.64.0.1", LocalPort: 8080, ContainerPort: 80, TargetHost: "localhost"},
			want: "100.64.0.1:8080:localhost:80",
		},
		{
			name: "forward target service",
			t:    Tunnel{LocalPort: 5432, ContainerPort: 5432, TargetHost: "postgres"},
			want: "127.0.0.1:5432:postgres:5432",
		},
		{
			name: "reverse",
			t:    Tunnel{LocalPort: 11434, ContainerPort: 11434, Reverse: true},
			want: "127.0.0.1:11434:127.0.0.1:11434",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.t.ForwardSpec(); got != c.want {
				t.Errorf("ForwardSpec() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestPortForwardService_BuildSshArgs_Persistent(t *testing.T) {
	svc := PortForwardService{Report: nopReporter{}}
	args, err := svc.buildSshArgs([]Tunnel{
		{LocalPort: 3000, ContainerPort: 3000, TargetHost: "localhost", Alias: "myws"},
		{LocalPort: 5432, ContainerPort: 5432, TargetHost: "postgres", Alias: "myws"},
		{LocalPort: 11434, ContainerPort: 11434, Reverse: true, Alias: "myws"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-N", "-o ExitOnForwardFailure=yes", "-L 127.0.0.1:3000:localhost:3000", "-L 127.0.0.1:5432:postgres:5432", "-R 127.0.0.1:11434:127.0.0.1:11434", "myws"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q missing %q", joined, want)
		}
	}
}

func TestPortForwardService_BuildSshArgs_EphemeralWithVia(t *testing.T) {
	svc := PortForwardService{Report: nopReporter{}}
	args, err := svc.buildSshArgs([]Tunnel{
		{
			LocalPort:     3000,
			ContainerPort: 3000,
			TargetHost:    "localhost",
			Ephemeral:     true,
			Target: EphemeralSshTarget{
				IP:      "172.20.0.2",
				User:    "devuser",
				KeyPath: "/tmp/id_devcontainer",
				Via:     "me@docker-host",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-J me@docker-host", "-i /tmp/id_devcontainer", "-N", "-L 127.0.0.1:3000:localhost:3000", "devuser@172.20.0.2"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q missing %q", joined, want)
		}
	}
}

func TestPortForwardService_BuildSshArgs_RejectsMixedModes(t *testing.T) {
	svc := PortForwardService{Report: nopReporter{}}
	_, err := svc.buildSshArgs([]Tunnel{
		{LocalPort: 3000, ContainerPort: 3000, Alias: "myws"},
		{LocalPort: 8080, ContainerPort: 80, Ephemeral: true, Target: EphemeralSshTarget{IP: "172.20.0.2", User: "devuser", KeyPath: "/tmp/k"}},
	})
	if err == nil {
		t.Fatal("expected error mixing persistent and ephemeral tunnels")
	}
}

func TestPortForwardService_BuildEphemeralTunnelVia(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_devcontainer")
	if err := os.WriteFile(keyPath, []byte("PRIVATE"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItest devcli"), 0o644); err != nil {
		t.Fatal(err)
	}

	fake := &fakeRunner{
		outputs: map[string]string{
			"ssh me@docker-host docker inspect --format '{{.State.Running}}' remote-ctr":                                          "true",
			"ssh me@docker-host docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' remote-ctr":  "172.24.0.5",
			"ssh me@docker-host docker exec -u root remote-ctr sh -c":                                                           "",
		},
	}
	prev := defaultRunner
	defaultRunner = fake
	t.Cleanup(func() { defaultRunner = prev })

	// A real key pair on disk: BuildEphemeralTunnel now resolves ephemeral access,
	// which reads the public half and installs it in the container.
	svc := PortForwardService{Report: nopReporter{}}
	tunnel, err := svc.BuildEphemeralTunnelVia(Tunnel{
		LocalPort:     3000,
		ContainerPort: 3000,
		TargetHost:    "localhost",
		ContainerName: "remote-ctr",
	}, "devuser", keyPath, "me@docker-host")
	if err != nil {
		t.Fatalf("BuildEphemeralTunnelVia: %v", err)
	}
	if !tunnel.Ephemeral || !tunnel.IsDevcontainer {
		t.Errorf("got Ephemeral=%v IsDevcontainer=%v; want true, true", tunnel.Ephemeral, tunnel.IsDevcontainer)
	}
	if tunnel.Target.Via != "me@docker-host" || tunnel.Target.IP != "172.24.0.5" {
		t.Errorf("got Target=%+v; want Via=me@docker-host IP=172.24.0.5", tunnel.Target)
	}
}

package service

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

func TestOpenTunnelsNoTunnelsIsNoop(t *testing.T) {
	svc := PortForwardService{Report: nopReporter{}}
	if err := svc.OpenTunnels(nil); err != nil {
		t.Errorf("OpenTunnels with no tunnels should be a no-op, got %v", err)
	}
}

func TestOpenTunnelsDetachedNoTunnelsIsNoop(t *testing.T) {
	svc := PortForwardService{Report: nopReporter{}}
	stop, err := svc.OpenTunnelsDetached(nil)
	if err != nil {
		t.Errorf("OpenTunnelsDetached with no tunnels should be a no-op, got %v", err)
	}
	stop() // must not panic on an empty tunnel set
}

func TestOpenTunnelsDetachedStartsAndStops(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	svc := PortForwardService{Report: nopReporter{}}
	tunnels := []Tunnel{
		{LocalPort: 12345, ContainerPort: 80, TargetHost: "localhost", Alias: "myalias"},
		{LocalPort: 12346, ContainerPort: 8080, TargetHost: "localhost", Ephemeral: true, Target: EphemeralTarget{IP: "172.18.0.2", User: "devuser", KeyPath: "/tmp/testkey"}},
	}
	stop, err := svc.OpenTunnelsDetached(tunnels)
	if err != nil {
		t.Fatalf("OpenTunnelsDetached: %v", err)
	}
	stop()
}

func TestTunnelCommand_AliasAndEphemeral(t *testing.T) {
	aliasTunnel := Tunnel{
		LocalPort:     3000,
		ContainerPort: 3000,
		TargetHost:    "localhost",
		Alias:         "myalias",
	}
	aliasCmd := tunnelCommand(aliasTunnel)
	expectedAliasArgs := []string{"ssh", "-N", "-L", "127.0.0.1:3000:localhost:3000", "myalias"}
	if len(aliasCmd.Args) != len(expectedAliasArgs) {
		t.Fatalf("expected %d args for alias command, got %v", len(expectedAliasArgs), aliasCmd.Args)
	}
	for i, arg := range expectedAliasArgs {
		if aliasCmd.Args[i] != arg {
			t.Errorf("alias arg[%d] = %q, want %q", i, aliasCmd.Args[i], arg)
		}
	}

	ephemeralTunnel := Tunnel{
		LocalPort:     8080,
		ContainerPort: 80,
		TargetHost:    "localhost",
		Ephemeral:     true,
		Target: EphemeralTarget{
			IP:      "172.20.0.5",
			User:    "devuser",
			KeyPath: "/home/devuser/.config/devcontainer-cli/id_ed25519",
		},
	}
	ephCmd := tunnelCommand(ephemeralTunnel)
	// The dial flags themselves are sshdefaults' contract; what this pins is that
	// an ephemeral tunnel dials with exactly them, and nothing of its own.
	expectedArgs := []string{"ssh", "-N", "-L", "127.0.0.1:8080:localhost:80"}
	expectedArgs = append(expectedArgs, sshdefaults.EphemeralDialArgs(ephemeralTunnel.Target.KeyPath)...)
	expectedArgs = append(expectedArgs, "devuser@172.20.0.5")
	if len(ephCmd.Args) != len(expectedArgs) {
		t.Fatalf("expected %d args, got %d: %v", len(expectedArgs), len(ephCmd.Args), ephCmd.Args)
	}
	for i, arg := range expectedArgs {
		if ephCmd.Args[i] != arg {
			t.Errorf("arg[%d] = %q, want %q", i, ephCmd.Args[i], arg)
		}
	}
}

// A reverse tunnel is the same ssh process with -R and the two ports swapped:
// the container binds ContainerPort and the connection is dialed from here to
// TargetHost:LocalPort.
func TestTunnelCommand_Reverse(t *testing.T) {
	aliasCmd := tunnelCommand(Tunnel{
		LocalPort:     5432,
		ContainerPort: 5432,
		Reverse:       true,
		TargetHost:    "localhost",
		Alias:         "myalias",
	})
	want := []string{"ssh", "-N", "-R", "127.0.0.1:5432:localhost:5432", "myalias"}
	if !slices.Equal(aliasCmd.Args, want) {
		t.Errorf("reverse alias args = %v, want %v", aliasCmd.Args, want)
	}

	// Distinct ports show which side each one binds: the container listens on
	// 80, this machine's 8080 is what gets dialed.
	mapped := tunnelCommand(Tunnel{
		LocalPort:     8080,
		ContainerPort: 80,
		Reverse:       true,
		TargetHost:    "192.168.1.20",
		Alias:         "myalias",
	})
	wantMapped := []string{"ssh", "-N", "-R", "127.0.0.1:80:192.168.1.20:8080", "myalias"}
	if !slices.Equal(mapped.Args, wantMapped) {
		t.Errorf("reverse mapped args = %v, want %v", mapped.Args, wantMapped)
	}

	ephemeral := tunnelCommand(Tunnel{
		LocalPort:     5432,
		ContainerPort: 5432,
		Reverse:       true,
		TargetHost:    "localhost",
		Ephemeral:     true,
		Target:        EphemeralTarget{IP: "172.18.0.2", User: "devuser", KeyPath: "/tmp/testkey"},
	})
	if !slices.Contains(ephemeral.Args, "-R") || slices.Contains(ephemeral.Args, "-L") {
		t.Errorf("ephemeral reverse args = %v, want -R and no -L", ephemeral.Args)
	}
}

func TestTunnelDescription(t *testing.T) {
	forward := Tunnel{LocalPort: 3000, ContainerPort: 80, TargetHost: "localhost"}
	if got, want := forward.Description(), "3000→localhost:80"; got != want {
		t.Errorf("forward description = %q, want %q", got, want)
	}
	reverse := Tunnel{LocalPort: 3000, ContainerPort: 80, TargetHost: "localhost", Reverse: true}
	if got, want := reverse.Description(), "container:80→localhost:3000"; got != want {
		t.Errorf("reverse description = %q, want %q", got, want)
	}
}

func TestBuildEphemeralTunnel(t *testing.T) {
	psOut := `{"Names":"my-container","Image":"img1","Status":"Up","State":"running","Labels":"","Ports":""}`
	inspectOut := "my-net 172.20.0.10\n"
	runner := &cmdRoutingRunner{psOut: psOut, inspectOut: inspectOut}
	defer useFakeDocker(runner)()

	// A real key pair on disk: BuildEphemeralTunnel now resolves ephemeral access,
	// which reads the public half and installs it in the container.
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, []byte("private"), 0o600); err != nil {
		t.Fatalf("writing key: %v", err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte("ssh-ed25519 AAAA test\n"), 0o644); err != nil {
		t.Fatalf("writing pubkey: %v", err)
	}

	svc := PortForwardService{Report: nopReporter{}}
	tunnel, err := svc.BuildEphemeralTunnel(Tunnel{
		LocalPort:     3000,
		ContainerPort: 3000,
		TargetHost:    "localhost",
		ContainerName: "my-container",
	}, "", keyPath)
	if err != nil {
		t.Fatalf("BuildEphemeralTunnel: %v", err)
	}
	if !tunnel.Ephemeral {
		t.Error("expected tunnel.Ephemeral to be true")
	}
	want := EphemeralTarget{IP: "172.20.0.10", User: "devuser", KeyPath: keyPath}
	if tunnel.Target != want {
		t.Errorf("tunnel.Target = %+v, want %+v", tunnel.Target, want)
	}
	// The public key must have been piped into the container: without it a
	// container that never ran `ssh --setup` refuses the tunnel's key.
	if !runner.sawKeyInstall() {
		t.Error("BuildEphemeralTunnel must install the public key in the container")
	}

	// The caller owns the direction; the service only fills in the connection.
	reverse, err := svc.BuildEphemeralTunnel(Tunnel{
		LocalPort:     5432,
		ContainerPort: 5432,
		Reverse:       true,
		TargetHost:    "localhost",
		ContainerName: "my-container",
	}, "", keyPath)
	if err != nil {
		t.Fatalf("BuildEphemeralTunnel (reverse): %v", err)
	}
	if !reverse.Reverse || !reverse.Ephemeral {
		t.Errorf("reverse ephemeral tunnel = %+v, want both Reverse and Ephemeral", reverse)
	}
}

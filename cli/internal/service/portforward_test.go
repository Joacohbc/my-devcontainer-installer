package service

import (
	"os"
	"path/filepath"
	"testing"
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
		{LocalPort: 12346, ContainerPort: 8080, TargetHost: "localhost", Ephemeral: true, HostIP: "172.18.0.2", User: "devuser", KeyPath: "/tmp/testkey"},
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
		HostIP:        "172.20.0.5",
		User:          "devuser",
		KeyPath:       "/home/devuser/.config/devcontainer-cli/id_ed25519",
	}
	ephCmd := tunnelCommand(ephemeralTunnel)
	expectedArgs := []string{
		"ssh", "-N", "-L", "127.0.0.1:8080:localhost:80",
		"-p", "2222",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", "/home/devuser/.config/devcontainer-cli/id_ed25519",
		"devuser@172.20.0.5",
	}
	if len(ephCmd.Args) != len(expectedArgs) {
		t.Fatalf("expected %d args, got %d: %v", len(expectedArgs), len(ephCmd.Args), ephCmd.Args)
	}
	for i, arg := range expectedArgs {
		if ephCmd.Args[i] != arg {
			t.Errorf("arg[%d] = %q, want %q", i, ephCmd.Args[i], arg)
		}
	}
}

func TestBuildEphemeralTunnel(t *testing.T) {
	psOut := `{"Names":"my-container","Image":"img1","Status":"Up","State":"running","Labels":"","Ports":""}`
	inspectOut := "my-net 172.20.0.10\n"
	runner := &cmdRoutingRunner{psOut: psOut, inspectOut: inspectOut}
	defer useFakeDocker(runner)()

	svc := PortForwardService{Report: nopReporter{}}
	tunnel, err := svc.BuildEphemeralTunnel(3000, 3000, "localhost", "my-container", "", "/custom/key")
	if err != nil {
		t.Fatalf("BuildEphemeralTunnel: %v", err)
	}
	if !tunnel.Ephemeral {
		t.Error("expected tunnel.Ephemeral to be true")
	}
	if tunnel.HostIP != "172.20.0.10" {
		t.Errorf("tunnel.HostIP = %q, want 172.20.0.10", tunnel.HostIP)
	}
	if tunnel.User != "devuser" {
		t.Errorf("tunnel.User = %q, want devuser", tunnel.User)
	}
	if tunnel.KeyPath != "/custom/key" {
		t.Errorf("tunnel.KeyPath = %q, want /custom/key", tunnel.KeyPath)
	}
}

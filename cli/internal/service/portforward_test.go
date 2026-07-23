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
	tunnels := []Tunnel{{LocalPort: 12345, ContainerPort: 80, TargetHost: "localhost", Alias: "myalias"}}
	stop, err := svc.OpenTunnelsDetached(tunnels)
	if err != nil {
		t.Fatalf("OpenTunnelsDetached: %v", err)
	}
	stop()
}

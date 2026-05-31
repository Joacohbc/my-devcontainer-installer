package service

import "testing"

func TestOpenTunnelsNoTunnelsIsNoop(t *testing.T) {
	svc := PortForwardService{Report: nopReporter{}}
	if err := svc.OpenTunnels(nil); err != nil {
		t.Errorf("OpenTunnels with no tunnels should be a no-op, got %v", err)
	}
}

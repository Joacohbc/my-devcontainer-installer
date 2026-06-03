package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestForwardID(t *testing.T) {
	if got := ForwardID("demo", 3000); got != "demo-3000" {
		t.Errorf("ForwardID = %q, want demo-3000", got)
	}
}

func TestForwardStateRoundTrip(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), ".dc_demo", "port-forwards.json")

	// Missing file → empty, no error.
	got, err := LoadForwardState(stateFile)
	if err != nil {
		t.Fatalf("LoadForwardState(missing): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("missing file = %d entries, want 0", len(got))
	}

	want := []RunningForward{
		{ID: "demo-3000", PID: 111, LocalPort: 3000, ContainerPort: 3000, TargetHost: "localhost", Alias: "demo", StartedAt: "2026-06-03T00:00:00Z"},
		{ID: "demo-8080", PID: 222, LocalPort: 8080, ContainerPort: 80, TargetHost: "web", Alias: "demo"},
	}
	if err := SaveForwardState(stateFile, want); err != nil {
		t.Fatalf("SaveForwardState: %v", err)
	}
	got, err = LoadForwardState(stateFile)
	if err != nil {
		t.Fatalf("LoadForwardState: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("loaded %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSaveForwardStateEmptyRemovesFile(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), ".dc_demo", "port-forwards.json")
	if err := SaveForwardState(stateFile, []RunningForward{{ID: "demo-3000", PID: 1}}); err != nil {
		t.Fatalf("SaveForwardState: %v", err)
	}
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("expected state file to exist: %v", err)
	}

	if err := SaveForwardState(stateFile, nil); err != nil {
		t.Fatalf("SaveForwardState(empty): %v", err)
	}
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Errorf("expected state file removed, stat err = %v", err)
	}

	// Removing an already-absent file is not an error.
	if err := SaveForwardState(stateFile, nil); err != nil {
		t.Errorf("SaveForwardState(empty, absent) = %v, want nil", err)
	}
}

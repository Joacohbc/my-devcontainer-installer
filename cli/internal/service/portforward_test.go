package service

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
)

func TestOpenTunnelsNoTunnelsIsNoop(t *testing.T) {
	svc := PortForwardService{Report: nopReporter{}}
	if err := svc.OpenTunnels(nil); err != nil {
		t.Errorf("OpenTunnels with no tunnels should be a no-op, got %v", err)
	}
}

// fakeProc emulates the OS process operations so tests never spawn ssh or signal
// real processes. Spawned pids are "alive" until killed.
type fakeProc struct {
	nextPID int
	alive   map[int]bool
	spawns  [][]string
	killed  []int
}

func newFakeProc() *fakeProc {
	return &fakeProc{nextPID: 1000, alive: map[int]bool{}}
}

func (f *fakeProc) install() func() {
	spawn := func(args []string, _ string) (int, error) {
		f.nextPID++
		pid := f.nextPID
		f.alive[pid] = true
		f.spawns = append(f.spawns, args)
		return pid, nil
	}
	alive := func(pid int) bool { return f.alive[pid] }
	kill := func(pid int) error {
		delete(f.alive, pid)
		f.killed = append(f.killed, pid)
		return nil
	}
	SetForwardOps(spawn, alive, kill)
	return ResetForwardOps
}

func TestStartConfiguredSpawnsAndPersists(t *testing.T) {
	fp := newFakeProc()
	defer fp.install()()

	cwd := t.TempDir()
	ws := "demo"
	forwards := []types.PortForward{
		{LocalPort: 3000, ContainerPort: 3000},
		{LocalPort: 8080, ContainerPort: 80, TargetHost: "web"},
	}

	svc := PortForwardService{Report: nopReporter{}}
	started, err := svc.StartConfigured(cwd, ws, forwards)
	if err != nil {
		t.Fatalf("StartConfigured: %v", err)
	}
	if len(started) != 2 {
		t.Fatalf("started = %d, want 2", len(started))
	}
	if len(fp.spawns) != 2 {
		t.Fatalf("spawns = %d, want 2", len(fp.spawns))
	}

	// The registry on disk should hold both forwards with their derived ids.
	state, err := domain.LoadForwardState(project.ProjectPaths(cwd, ws).ForwardStateFile)
	if err != nil {
		t.Fatalf("LoadForwardState: %v", err)
	}
	if len(state) != 2 {
		t.Fatalf("registry size = %d, want 2", len(state))
	}
	if state[0].ID != domain.ForwardID(ws, 3000) {
		t.Errorf("id = %q, want %q", state[0].ID, domain.ForwardID(ws, 3000))
	}
	if state[1].TargetHost != "web" {
		t.Errorf("targetHost = %q, want web", state[1].TargetHost)
	}

	// Re-running with the same (alive) forwards must not duplicate them.
	again, err := svc.StartConfigured(cwd, ws, forwards)
	if err != nil {
		t.Fatalf("StartConfigured (re-run): %v", err)
	}
	if len(again) != 0 {
		t.Errorf("re-run started = %d, want 0 (already running)", len(again))
	}
	if len(fp.spawns) != 2 {
		t.Errorf("re-run spawned more; spawns = %d, want 2", len(fp.spawns))
	}
}

func TestStartConfiguredDefaultsAlias(t *testing.T) {
	fp := newFakeProc()
	defer fp.install()()

	cwd := t.TempDir()
	ws := "myws"
	svc := PortForwardService{Report: nopReporter{}}
	if _, err := svc.StartConfigured(cwd, ws, []types.PortForward{{LocalPort: 5432, ContainerPort: 5432}}); err != nil {
		t.Fatalf("StartConfigured: %v", err)
	}
	state, _ := domain.LoadForwardState(project.ProjectPaths(cwd, ws).ForwardStateFile)
	if len(state) != 1 {
		t.Fatalf("registry size = %d, want 1", len(state))
	}
	if state[0].Alias != ws {
		t.Errorf("alias = %q, want workspace %q", state[0].Alias, ws)
	}
	if state[0].TargetHost != "localhost" {
		t.Errorf("targetHost = %q, want localhost", state[0].TargetHost)
	}
}

func TestListForwardsPrunesDead(t *testing.T) {
	fp := newFakeProc()
	defer fp.install()()

	cwd := t.TempDir()
	ws := "demo"
	svc := PortForwardService{Report: nopReporter{}}
	if _, err := svc.StartConfigured(cwd, ws, []types.PortForward{
		{LocalPort: 3000, ContainerPort: 3000},
		{LocalPort: 8080, ContainerPort: 8080},
	}); err != nil {
		t.Fatalf("StartConfigured: %v", err)
	}

	// Kill one process out from under the registry.
	stateFile := project.ProjectPaths(cwd, ws).ForwardStateFile
	state, _ := domain.LoadForwardState(stateFile)
	delete(fp.alive, state[0].PID)

	alive, err := svc.ListForwards(cwd, ws)
	if err != nil {
		t.Fatalf("ListForwards: %v", err)
	}
	if len(alive) != 1 {
		t.Fatalf("alive = %d, want 1", len(alive))
	}
	// The dead entry must have been pruned from disk.
	persisted, _ := domain.LoadForwardState(stateFile)
	if len(persisted) != 1 {
		t.Errorf("persisted = %d, want 1 after prune", len(persisted))
	}
}

func TestStopForwardByID(t *testing.T) {
	fp := newFakeProc()
	defer fp.install()()

	cwd := t.TempDir()
	ws := "demo"
	svc := PortForwardService{Report: nopReporter{}}
	if _, err := svc.StartConfigured(cwd, ws, []types.PortForward{
		{LocalPort: 3000, ContainerPort: 3000},
		{LocalPort: 8080, ContainerPort: 8080},
	}); err != nil {
		t.Fatalf("StartConfigured: %v", err)
	}

	id := domain.ForwardID(ws, 3000)
	if err := svc.StopForward(cwd, ws, id); err != nil {
		t.Fatalf("StopForward: %v", err)
	}
	if len(fp.killed) != 1 {
		t.Errorf("killed = %d, want 1", len(fp.killed))
	}
	remaining, _ := domain.LoadForwardState(project.ProjectPaths(cwd, ws).ForwardStateFile)
	if len(remaining) != 1 || remaining[0].ID != domain.ForwardID(ws, 8080) {
		t.Errorf("remaining = %+v, want only 8080", remaining)
	}

	if err := svc.StopForward(cwd, ws, "no-such-id"); err == nil {
		t.Error("StopForward(unknown) = nil, want error")
	}
}

func TestStopAllForwards(t *testing.T) {
	fp := newFakeProc()
	defer fp.install()()

	cwd := t.TempDir()
	ws := "demo"
	svc := PortForwardService{Report: nopReporter{}}
	if _, err := svc.StartConfigured(cwd, ws, []types.PortForward{
		{LocalPort: 3000, ContainerPort: 3000},
		{LocalPort: 8080, ContainerPort: 8080},
	}); err != nil {
		t.Fatalf("StartConfigured: %v", err)
	}

	killed, err := svc.StopAllForwards(cwd, ws)
	if err != nil {
		t.Fatalf("StopAllForwards: %v", err)
	}
	if killed != 2 {
		t.Errorf("killed = %d, want 2", killed)
	}
	// Registry file should be cleared.
	state, _ := domain.LoadForwardState(project.ProjectPaths(cwd, ws).ForwardStateFile)
	if len(state) != 0 {
		t.Errorf("registry = %d, want 0 after stop-all", len(state))
	}

	// No-op on an empty project must not error.
	if _, err := svc.StopAllForwards(t.TempDir(), "empty"); err != nil {
		t.Errorf("StopAllForwards(empty) = %v, want nil", err)
	}
}

func TestParsePortForwards(t *testing.T) {
	cases := []struct {
		in      string
		want    []types.PortForward
		wantErr bool
	}{
		{in: "", want: nil},
		{in: "3000", want: []types.PortForward{{LocalPort: 3000, ContainerPort: 3000}}},
		{in: "8080:80", want: []types.PortForward{{LocalPort: 8080, ContainerPort: 80}}},
		{in: "5432:postgres:5432", want: []types.PortForward{{LocalPort: 5432, TargetHost: "postgres", ContainerPort: 5432}}},
		{in: "3000, 8080:80", want: []types.PortForward{{LocalPort: 3000, ContainerPort: 3000}, {LocalPort: 8080, ContainerPort: 80}}},
		{in: "abc", wantErr: true},
		{in: "70000", wantErr: true},
		{in: "3000:host:", wantErr: true},
	}
	for _, c := range cases {
		got, err := ParsePortForwards(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParsePortForwards(%q) = nil err, want error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParsePortForwards(%q): %v", c.in, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("ParsePortForwards(%q) = %+v, want %+v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ParsePortForwards(%q)[%d] = %+v, want %+v", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestFormatPortForwardsRoundTrip(t *testing.T) {
	forwards := []types.PortForward{
		{LocalPort: 3000, ContainerPort: 3000},
		{LocalPort: 8080, ContainerPort: 80},
		{LocalPort: 5432, TargetHost: "postgres", ContainerPort: 5432},
	}
	got := formatPortForwards(forwards)
	want := "3000, 8080:80, 5432:postgres:5432"
	if got != want {
		t.Fatalf("formatPortForwards = %q, want %q", got, want)
	}
	parsed, err := ParsePortForwards(got)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	for i := range forwards {
		if parsed[i] != forwards[i] {
			t.Errorf("round-trip[%d] = %+v, want %+v", i, parsed[i], forwards[i])
		}
	}
}

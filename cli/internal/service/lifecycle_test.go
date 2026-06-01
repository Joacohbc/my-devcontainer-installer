package service

import (
	"slices"
	"testing"
)

func TestLifecycleUpArgs(t *testing.T) {
	cases := []struct {
		name      string
		build     bool
		wantBuild bool
	}{
		{"plain up", false, false},
		{"up with build", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runner := &fakeRunner{status: 0}
			defer useFakeDocker(runner)()

			svc := LifecycleService{Report: nopReporter{}}
			if err := svc.Up("compose.yml", "ws", c.build); err != nil {
				t.Fatalf("Up: %v", err)
			}
			call := runner.callContaining("up")
			if call == nil || !slices.Contains(call, "-d") {
				t.Fatalf("expected 'up -d' call, got %v", call)
			}
			if slices.Contains(call, "--build") != c.wantBuild {
				t.Errorf("--build presence = %v, want %v (call %v)", !c.wantBuild, c.wantBuild, call)
			}
		})
	}
}

func TestLifecycleDownVolumes(t *testing.T) {
	for _, removeVolumes := range []bool{false, true} {
		runner := &fakeRunner{status: 0}
		restore := useFakeDocker(runner)
		svc := LifecycleService{Report: nopReporter{}}
		if err := svc.Down("compose.yml", "ws", removeVolumes); err != nil {
			t.Fatalf("Down: %v", err)
		}
		call := runner.callContaining("down")
		if call == nil {
			t.Fatalf("expected a down call")
		}
		if slices.Contains(call, "-v") != removeVolumes {
			t.Errorf("-v presence = %v, want %v (call %v)", !removeVolumes, removeVolumes, call)
		}
		restore()
	}
}

func TestLifecycleComposeVerb(t *testing.T) {
	runner := &fakeRunner{status: 0}
	defer useFakeDocker(runner)()

	svc := LifecycleService{Report: nopReporter{}}
	if err := svc.Compose("compose.yml", "restart"); err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if runner.callContaining("restart") == nil {
		t.Errorf("expected a compose restart call; calls=%v", runner.calls)
	}
}

func TestLifecycleContainerOps(t *testing.T) {
	runner := &fakeRunner{status: 0}
	defer useFakeDocker(runner)()

	svc := LifecycleService{Report: nopReporter{}}
	if err := svc.StartContainer("c1"); err != nil {
		t.Fatalf("StartContainer: %v", err)
	}
	if call := runner.callContaining("start"); call == nil || !slices.Contains(call, "c1") {
		t.Errorf("expected start c1; calls=%v", runner.calls)
	}

	if err := svc.RemoveContainer("c1"); err != nil {
		t.Fatalf("RemoveContainer: %v", err)
	}
	if runner.callContaining("stop") == nil {
		t.Errorf("expected a stop call; calls=%v", runner.calls)
	}
	if call := runner.callContaining("rm"); call == nil || !slices.Contains(call, "c1") {
		t.Errorf("expected rm c1; calls=%v", runner.calls)
	}
}

func TestLifecycleStopContainer(t *testing.T) {
	runner := &fakeRunner{status: 0}
	defer useFakeDocker(runner)()

	svc := LifecycleService{Report: nopReporter{}}
	if err := svc.StopContainer("c1"); err != nil {
		t.Fatalf("StopContainer: %v", err)
	}
	call := runner.callContaining("stop")
	if call == nil || !slices.Contains(call, "c1") {
		t.Errorf("expected docker stop c1; calls=%v", runner.calls)
	}
}

func TestLifecycleStopContainerFails(t *testing.T) {
	runner := &fakeRunner{status: 1}
	defer useFakeDocker(runner)()

	svc := LifecycleService{Report: nopReporter{}}
	if err := svc.StopContainer("c1"); err == nil {
		t.Error("expected error on non-zero exit")
	}
}

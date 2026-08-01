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

func TestLifecyclePassthrough(t *testing.T) {
	cases := []struct {
		name    string
		envFile string
		args    []string
		// wantSeq is the ordered tail the recorded call must contain, starting at
		// "compose": docker compose -f <file> [--env-file <env>] <args...>.
		wantSeq []string
	}{
		{
			name:    "with env file",
			envFile: "/p/.dc_ws/build/.env",
			args:    []string{"exec", "-it", "postgres", "psql"},
			wantSeq: []string{"compose", "-f", "compose.yml", "--env-file", "/p/.dc_ws/build/.env", "exec", "-it", "postgres", "psql"},
		},
		{
			name:    "no env file falls back to autoload",
			envFile: "",
			args:    []string{"ps"},
			wantSeq: []string{"compose", "-f", "compose.yml", "ps"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runner := &fakeRunner{status: 0}
			defer useFakeDocker(runner)()

			svc := LifecycleService{Report: nopReporter{}}
			if err := svc.Passthrough("compose.yml", c.envFile, c.args); err != nil {
				t.Fatalf("Passthrough: %v", err)
			}
			call := runner.callContaining(c.args[0])
			if call == nil {
				t.Fatalf("expected a compose call containing %q; calls=%v", c.args[0], runner.calls)
			}
			if !containsSubseq(call, c.wantSeq) {
				t.Errorf("call %v does not contain ordered %v", call, c.wantSeq)
			}
			if c.envFile == "" && slices.Contains(call, "--env-file") {
				t.Errorf("did not expect --env-file when envFile is empty; call=%v", call)
			}
		})
	}
}

func TestLifecyclePassthroughNonZero(t *testing.T) {
	runner := &fakeRunner{status: 2}
	defer useFakeDocker(runner)()

	svc := LifecycleService{Report: nopReporter{}}
	if err := svc.Passthrough("compose.yml", "", []string{"config"}); err == nil {
		t.Error("expected error on non-zero compose exit")
	}
}

// containsSubseq reports whether want appears as a contiguous run inside call.
func containsSubseq(call, want []string) bool {
	if len(want) == 0 {
		return true
	}
	for i := 0; i+len(want) <= len(call); i++ {
		if slices.Equal(call[i:i+len(want)], want) {
			return true
		}
	}
	return false
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

func TestLifecycleRestartContainer(t *testing.T) {
	runner := &fakeRunner{status: 0}
	defer useFakeDocker(runner)()

	svc := LifecycleService{Report: nopReporter{}}
	if err := svc.RestartContainer("c1"); err != nil {
		t.Fatalf("RestartContainer: %v", err)
	}
	call := runner.callContaining("restart")
	if call == nil || !slices.Contains(call, "c1") {
		t.Errorf("expected docker restart c1; calls=%v", runner.calls)
	}
}

func TestLifecycleRestartContainerFails(t *testing.T) {
	runner := &fakeRunner{status: 1}
	defer useFakeDocker(runner)()

	svc := LifecycleService{Report: nopReporter{}}
	if err := svc.RestartContainer("c1"); err == nil {
		t.Error("expected error on non-zero exit")
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

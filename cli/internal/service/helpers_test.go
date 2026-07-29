package service

import (
	"context"
	"slices"
	"sync"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

type nopReporter struct{}

func (nopReporter) Info(string, ...any)    {}
func (nopReporter) Warn(string, ...any)    {}
func (nopReporter) Success(string, ...any) {}
func (nopReporter) Error(string, ...any)   {}
func (nopReporter) Fatal(string, ...any)   {}
func (nopReporter) Debug(string, ...any)   {}

// fakeRunner stands in for the real docker Runner: it records every invocation
// and replies with a canned status/stdout so services can be exercised without
// a Docker daemon.
type fakeRunner struct {
	mu     sync.Mutex
	calls  [][]string
	status int
	stdout string
}

func (r *fakeRunner) Run(_ context.Context, args []string, _ string, _ string, _ map[string]string) (int, string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(args) >= 2 && args[1] == "version" {
		return 0, "27.0.0", ""
	}
	r.calls = append(r.calls, args)
	return r.status, r.stdout, ""
}

// callContaining returns the first recorded invocation that includes token, or
// nil if none did.
func (r *fakeRunner) callContaining(token string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, call := range r.calls {
		if slices.Contains(call, token) {
			return call
		}
	}
	return nil
}

// lastCallContaining returns the LAST recorded invocation that includes token.
// Use it when an earlier call legitimately mentions the same token (e.g. a
// `test -x <path>` probe preceding the real `exec <path>`).
func (r *fakeRunner) lastCallContaining(token string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.calls) - 1; i >= 0; i-- {
		if slices.Contains(r.calls[i], token) {
			return r.calls[i]
		}
	}
	return nil
}

// scriptedPrompter drives the wizard forward-only, answering each step from
// answers (keyed by step Key) or falling back to the field's seeded default.
// The non-wizard Prompter methods are unused by the wizard and return zero values.
type scriptedPrompter struct {
	answers map[string]any
}

func (p scriptedPrompter) Ask(string) (string, error)   { return "", nil }
func (p scriptedPrompter) Confirm(string) (bool, error) { return false, nil }
func (p scriptedPrompter) Select(string, []Option, Option) (Option, error) {
	return Option{}, nil
}
func (p scriptedPrompter) Multiselect(string, []Option, []Option) ([]Option, error) {
	return nil, nil
}

func (p scriptedPrompter) Wizard(build func(*State) []Step) (*State, error) {
	state := NewState()
	for index := 0; ; index++ {
		steps := build(state)
		if index >= len(steps) {
			return state, nil
		}
		step := steps[index]
		if v, ok := p.answers[step.Key]; ok {
			state.Set(step.Key, v)
		} else {
			state.Set(step.Key, defaultForField(step.Build(state)))
		}
	}
}

func defaultForField(f Field) any {
	switch f.Kind {
	case FieldMultiselect:
		if ss, ok := f.Initial.([]string); ok {
			return ss
		}
		return []string{}
	case FieldConfirm:
		b, _ := f.Initial.(bool)
		return b
	default:
		s, _ := f.Initial.(string)
		return s
	}
}

// useFakeDocker installs r as the docker Runner and returns a restore func that
// resets the runner and availability cache. It takes a docker.Runner rather
// than a concrete *fakeRunner so tests can wrap it (see missingScriptRunner in
// inspect_test.go).
func useFakeDocker(r docker.Runner) func() {
	docker.SetRunner(r)
	docker.ResetDockerCache()
	return func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	}
}

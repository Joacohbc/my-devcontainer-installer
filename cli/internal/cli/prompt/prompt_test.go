package prompt

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// TestFormModelEscCancels verifies a one-off prompt treats esc as cancel.
func TestFormModelEscCancels(t *testing.T) {
	f := huh.NewForm(huh.NewGroup(huh.NewInput().Title("x")))
	m := &formModel{form: f}
	m.Init()

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	fm := newModel.(*formModel)
	if !fm.cancelled {
		t.Error("expected esc to set cancelled")
	}
	if cmd == nil || cmd() != tea.Quit() {
		t.Error("expected esc to emit tea.Quit")
	}
}

// TestFormModelCompletedQuits guards the freeze bug: a completed embedded form
// must emit tea.Quit so the program terminates.
func TestFormModelCompletedQuits(t *testing.T) {
	f := huh.NewForm(huh.NewGroup(huh.NewInput().Title("x")))
	m := &formModel{form: f}
	m.Init()

	quit := drainToQuit(t, func(msg tea.Msg) (tea.Model, tea.Cmd) { return m.Update(msg) }, tea.KeyMsg{Type: tea.KeyEnter})
	if m.form.State != huh.StateCompleted {
		t.Fatalf("expected StateCompleted, got %v", m.form.State)
	}
	if !quit {
		t.Error("expected tea.Quit once the form completed")
	}
}

// TestStepperForward walks three static steps and asserts the answers land in State.
func TestStepperForward(t *testing.T) {
	w := NewStepper(func(s *State) []Step {
		return []Step{
			{Key: "a", Build: func(*State) Field { return Field{Kind: FieldInput, Title: "a"} }},
			{Key: "b", Build: func(*State) Field { return Field{Kind: FieldInput, Title: "b"} }},
			{Key: "c", Build: func(*State) Field { return Field{Kind: FieldInput, Title: "c"} }},
		}
	})
	answers := map[string]any{"a": "1", "b": "2", "c": "3"}
	runner := scriptedRunner(answers, nil)

	st, err := w.run(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for k, v := range answers {
		if st.String(k) != v {
			t.Errorf("state[%q]=%q, want %q", k, st.String(k), v)
		}
	}
}

// TestStepperBackReseeds verifies going back re-runs the prior step with its
// previously entered value seeded into the field's Initial.
func TestStepperBackReseeds(t *testing.T) {
	w := NewStepper(func(s *State) []Step {
		return []Step{
			{Key: "a", Build: func(s *State) Field { return Field{Kind: FieldInput, Title: "a", Initial: s.String("a")} }},
			{Key: "b", Build: func(s *State) Field { return Field{Kind: FieldInput, Title: "b"} }},
		}
	})

	// Sequence: enter a="first" (next), at b go back, then a is re-shown seeded
	// with "first" — the runner asserts that and answers "second", then b next.
	var sawReseed bool
	calls := 0
	runner := func(f Field) (any, outcome, error) {
		calls++
		switch calls {
		case 1: // step a, first visit
			return "first", outcomeNext, nil
		case 2: // step b, go back
			return nil, outcomeBack, nil
		case 3: // step a again — must be seeded with prior value
			if f.Initial == "first" {
				sawReseed = true
			}
			return "second", outcomeNext, nil
		default: // step b
			return "x", outcomeNext, nil
		}
	}

	st, err := w.run(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sawReseed {
		t.Error("expected step a to be re-seeded with the previously entered value")
	}
	if st.String("a") != "second" {
		t.Errorf("state[a]=%q, want %q", st.String("a"), "second")
	}
}

// TestStepperDynamicBranch verifies the step list recomputes from State so a
// selection inserts a downstream step.
func TestStepperDynamicBranch(t *testing.T) {
	w := NewStepper(func(s *State) []Step {
		steps := []Step{
			{Key: "mode", Build: func(*State) Field { return Field{Kind: FieldSelect, Title: "mode"} }},
		}
		if s.String("mode") == "advanced" {
			steps = append(steps, Step{Key: "extra", Build: func(*State) Field { return Field{Kind: FieldInput, Title: "extra"} }})
		}
		return steps
	})
	runner := scriptedRunner(map[string]any{"mode": "advanced", "extra": "v"}, nil)

	st, err := w.run(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !st.Has("extra") {
		t.Error("expected the dynamic 'extra' step to run and record its answer")
	}
}

// TestStepperEscAtFirstCancels verifies backing out of the first step cancels.
func TestStepperEscAtFirstCancels(t *testing.T) {
	w := NewStepper(func(*State) []Step {
		return []Step{{Key: "a", Build: func(*State) Field { return Field{Kind: FieldInput, Title: "a"} }}}
	})
	runner := func(Field) (any, outcome, error) { return nil, outcomeBack, nil }

	if _, err := w.run(runner); err != ErrCancelled {
		t.Errorf("expected ErrCancelled, got %v", err)
	}
}

// scriptedRunner answers each step by its Key from answers, with outcomeNext.
func scriptedRunner(answers map[string]any, _ error) stepRunner {
	// The runner doesn't know the Key, so map by Title (tests set Title==Key).
	return func(f Field) (any, outcome, error) {
		return answers[f.Title], outcomeNext, nil
	}
}

// drainToQuit feeds msg into update and chases returned commands until a
// tea.Quit is observed or the queue drains. Returns true if quit was seen.
func drainToQuit(t *testing.T, update func(tea.Msg) (tea.Model, tea.Cmd), initial tea.Msg) bool {
	t.Helper()
	queue := []tea.Msg{initial}
	for steps := 0; steps < 100 && len(queue) > 0; steps++ {
		msg := queue[0]
		queue = queue[1:]
		if msg == nil {
			continue
		}
		if _, ok := msg.(tea.QuitMsg); ok {
			return true
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				if c != nil {
					queue = append(queue, c())
				}
			}
			continue
		}
		_, cmd := update(msg)
		if cmd != nil {
			queue = append(queue, cmd())
		}
	}
	return false
}

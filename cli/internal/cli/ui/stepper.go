package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	sv "github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

// Stepper drives an ordered, dynamically-branchable sequence of steps with
// esc-back navigation. The step list is recomputed from State on every move, so
// changing an earlier answer transparently reshapes the downstream steps.
type Stepper struct {
	build func(s *sv.State) []sv.Step
	state *sv.State
}

// NewStepper builds a wizard whose step list is a function of the accumulated
// State. For a static wizard, return a fixed slice ignoring the argument.
func NewStepper(build func(s *sv.State) []sv.Step) *Stepper {
	return &Stepper{build: build, state: sv.NewState()}
}

// outcome is the result of running one step.
type outcome int

const (
	outcomeNext outcome = iota
	outcomeBack
	outcomeCancel
)

// stepRunner renders a field and reports the entered value plus the navigation
// outcome. It is injected so the navigation/state logic is testable without a TTY.
type stepRunner func(f sv.Field) (value any, oc outcome, err error)

// Run executes the wizard against a real terminal, returning the final State or
// ErrCancelled if the user backed out of the first step or aborted.
func (w *Stepper) Run() (*sv.State, error) {
	return w.run(runStepForm)
}

// run is the pure navigation loop; tests inject a scripted runner.
func (w *Stepper) run(runner stepRunner) (*sv.State, error) {
	index := 0
	for {
		steps := w.build(w.state)
		if index >= len(steps) {
			return w.state, nil
		}
		step := steps[index]
		value, oc, err := runner(step.Build(w.state))
		if err != nil {
			return nil, err
		}
		switch oc {
		case outcomeCancel:
			return nil, ErrCancelled
		case outcomeBack:
			if index == 0 {
				return nil, ErrCancelled
			}
			index--
		default: // outcomeNext
			w.state.Set(step.Key, value)
			index++
		}
	}
}

// stepModel wraps a step's huh form and intercepts esc as "go back".
type stepModel struct {
	form *huh.Form
	back bool
}

func (m *stepModel) Init() tea.Cmd { return m.form.Init() }

func (m *stepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "esc" {
		m.back = true
		return m, tea.Quit
	}
	newForm, cmd := m.form.Update(msg)
	m.form = newForm.(*huh.Form)
	if m.form.State == huh.StateCompleted || m.form.State == huh.StateAborted {
		return m, tea.Quit
	}
	return m, cmd
}

func (m *stepModel) View() string {
	return m.form.View() + hintStyle.Render(" esc go back") + "\n"
}

// runStepForm is the real terminal renderer behind Stepper.Run.
func runStepForm(f sv.Field) (any, outcome, error) {
	time.Sleep(50 * time.Millisecond)

	value, form := buildStepForm(f)
	m := &stepModel{form: form.WithTheme(devcontainerTheme())}
	finalModel, err := tea.NewProgram(m).Run()
	if err != nil {
		return nil, outcomeCancel, err
	}
	sm := finalModel.(*stepModel)
	if sm.back {
		return nil, outcomeBack, nil
	}
	if sm.form.State == huh.StateAborted {
		return nil, outcomeCancel, nil
	}
	return value(), outcomeNext, nil
}

// buildStepForm constructs the huh form for a field and returns a getter for
// the bound value (read after the form completes) plus the form to run.
func buildStepForm(f sv.Field) (func() any, *huh.Form) {
	switch f.Kind {
	case sv.FieldMultiselect:
		init, _ := f.Initial.([]string)
		opts := make([]huh.Option[string], len(f.Choices))
		for i, c := range f.Choices {
			opts[i] = huh.NewOption(c.Label, c.Value).Selected(contains(init, c.Value))
		}
		var res []string
		form := huh.NewForm(huh.NewGroup(
			huh.NewMultiSelect[string]().Title(f.Title).Options(opts...).Value(&res),
		))
		return func() any { return res }, form
	case sv.FieldConfirm:
		res, _ := f.Initial.(bool)
		form := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title(f.Title).Value(&res),
		))
		return func() any { return res }, form
	case sv.FieldInput:
		res, _ := f.Initial.(string)
		field := huh.NewInput().Title(f.Title).Value(&res)
		if f.Validate != nil {
			field = field.Validate(f.Validate)
		}
		form := huh.NewForm(huh.NewGroup(field))
		return func() any { return res }, form
	default: // FieldSelect
		res, _ := f.Initial.(string)
		opts := make([]huh.Option[string], len(f.Choices))
		for i, c := range f.Choices {
			opts[i] = huh.NewOption(c.Label, c.Value)
		}
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().Title(f.Title).Options(opts...).Value(&res),
		))
		return func() any { return res }, form
	}
}

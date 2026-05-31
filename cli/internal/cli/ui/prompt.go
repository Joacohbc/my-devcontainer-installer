package ui

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	sv "github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

// ErrCancelled is returned when the user aborts a prompt (esc or ctrl+c).
var ErrCancelled = errors.New("prompt cancelled")

// formModel runs a single one-off huh form. esc (and ctrl+c) cancel; there is
// no back navigation — that is exclusive to the wizard Stepper.
type formModel struct {
	form      *huh.Form
	cancelled bool
}

func (m *formModel) Init() tea.Cmd { return m.form.Init() }

func (m *formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "esc" {
		m.cancelled = true
		return m, tea.Quit
	}
	newForm, cmd := m.form.Update(msg)
	m.form = newForm.(*huh.Form)

	// huh does not emit tea.Quit on its own when embedded in a parent model;
	// the container must quit once the form is completed or aborted, otherwise
	// the program hangs after the field stops rendering.
	if m.form.State == huh.StateCompleted || m.form.State == huh.StateAborted {
		return m, tea.Quit
	}
	return m, cmd
}

func (m *formModel) View() string { return m.form.View() }

// runOnce runs a one-off form and maps an abort/esc to ErrCancelled.
func runOnce(f *huh.Form) error {
	// Tiny delay to let the TTY driver/terminal state settle between sequential programs.
	time.Sleep(50 * time.Millisecond)

	m := &formModel{form: f}
	finalModel, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	fm := finalModel.(*formModel)
	if fm.cancelled || fm.form.State == huh.StateAborted {
		return ErrCancelled
	}
	return nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func Multiselect(message string, choices []sv.Option, initial []sv.Option) ([]sv.Option, error) {
	options := make([]huh.Option[string], len(choices))

	initialStr := make([]string, len(initial))
	for i, c := range initial {
		initialStr[i] = c.Value
	}

	for i, c := range choices {
		options[i] = huh.NewOption(c.Label, c.Value).Selected(contains(initialStr, c.Value))
	}
	var result []string
	f := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title(message).
			Options(options...).
			Value(&result),
	)).WithTheme(devcontainerTheme())

	if err := runOnce(f); err != nil {
		return nil, err
	}

	selected := make([]sv.Option, 0, len(result))
	for _, r := range result {
		for _, c := range choices {
			if c.Value == r {
				selected = append(selected, c)
				break
			}
		}
	}

	return selected, nil
}

func Select(message string, choices []sv.Option, initial sv.Option) (sv.Option, error) {
	options := make([]huh.Option[string], len(choices))
	for i, c := range choices {
		options[i] = huh.NewOption(c.Label, c.Value)
	}
	result := initial.Value
	f := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(message).
			Options(options...).
			Value(&result),
	)).WithTheme(devcontainerTheme())

	if err := runOnce(f); err != nil {
		return sv.Option{}, err
	}
	for _, c := range choices {
		if c.Value == result {
			return c, nil
		}
	}
	return sv.Option{}, nil
}

func Input(message, initial string, validate func(string) error) (string, error) {
	result := initial
	field := huh.NewInput().Title(message).Value(&result)
	if validate != nil {
		field = field.Validate(validate)
	}
	f := huh.NewForm(huh.NewGroup(field)).WithTheme(devcontainerTheme())

	if err := runOnce(f); err != nil {
		return "", err
	}
	return result, nil
}

func Confirm(message string, initial bool) (bool, error) {
	result := initial
	f := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title(message).Value(&result),
	)).WithTheme(devcontainerTheme())

	if err := runOnce(f); err != nil {
		return false, err
	}
	return result, nil
}

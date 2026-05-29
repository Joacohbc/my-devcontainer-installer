package prompt

import (
	"errors"

	"github.com/charmbracelet/huh"
)

var ErrCancelled = errors.New("prompt cancelled")

type Choice struct {
	Value string
	Label string
}

func isAborted(err error) bool {
	return errors.Is(err, huh.ErrUserAborted)
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func Multiselect(message string, choices []Choice, initial []string) ([]string, error) {
	options := make([]huh.Option[string], len(choices))
	for i, c := range choices {
		options[i] = huh.NewOption(c.Label, c.Value).Selected(contains(initial, c.Value))
	}
	var result []string
	f := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title(message).
			Options(options...).
			Value(&result),
	)).WithTheme(devcontainerTheme())
	if err := f.Run(); err != nil {
		if isAborted(err) {
			return nil, ErrCancelled
		}
		return nil, err
	}
	return result, nil
}

func Select(message string, choices []Choice, initial string) (string, error) {
	options := make([]huh.Option[string], len(choices))
	for i, c := range choices {
		options[i] = huh.NewOption(c.Label, c.Value)
	}
	result := initial
	f := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(message).
			Options(options...).
			Value(&result),
	)).WithTheme(devcontainerTheme())
	if err := f.Run(); err != nil {
		if isAborted(err) {
			return "", ErrCancelled
		}
		return "", err
	}
	return result, nil
}

func Input(message, initial string, validate func(string) error) (string, error) {
	result := initial
	field := huh.NewInput().Title(message).Value(&result)
	if validate != nil {
		field = field.Validate(validate)
	}
	f := huh.NewForm(huh.NewGroup(field)).WithTheme(devcontainerTheme())
	if err := f.Run(); err != nil {
		if isAborted(err) {
			return "", ErrCancelled
		}
		return "", err
	}
	return result, nil
}

func Confirm(message string, initial bool) (bool, error) {
	result := initial
	f := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title(message).Value(&result),
	)).WithTheme(devcontainerTheme())
	if err := f.Run(); err != nil {
		if isAborted(err) {
			return false, ErrCancelled
		}
		return false, err
	}
	return result, nil
}

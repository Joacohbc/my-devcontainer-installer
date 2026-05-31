package ui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

func devcontainerTheme() *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Title = t.Focused.Title.Foreground(ColorPrimary).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(ColorSubtle)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(ColorSuccess).Bold(true)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(ColorPrimary).SetString("▸ ")
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(ColorPrimary).SetString("▸ ")
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(ColorSuccess).SetString("✓ ")
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(ColorSubtle).SetString("○ ")
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(lipgloss.Color("0")).Background(ColorPrimary).Bold(true)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(ColorWarning)

	t.Blurred.Title = t.Blurred.Title.Foreground(ColorSubtle)
	t.Blurred.Description = t.Blurred.Description.Foreground(ColorSubtle)

	return t
}

// hintStyle styles the navigation footer rendered under wizard steps.
var hintStyle = lipgloss.NewStyle().Foreground(ColorSubtle).Faint(true)

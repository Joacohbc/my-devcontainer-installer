package prompt

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
)

func devcontainerTheme() *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Title = t.Focused.Title.Foreground(ui.ColorPrimary).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(ui.ColorSubtle)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(ui.ColorSuccess).Bold(true)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(ui.ColorPrimary).SetString("▸ ")
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(ui.ColorPrimary).SetString("▸ ")
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(ui.ColorSuccess).SetString("✓ ")
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(ui.ColorSubtle).SetString("○ ")
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(lipgloss.Color("0")).Background(ui.ColorPrimary).Bold(true)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(ui.ColorWarning)

	t.Blurred.Title = t.Blurred.Title.Foreground(ui.ColorSubtle)
	t.Blurred.Description = t.Blurred.Description.Foreground(ui.ColorSubtle)

	return t
}

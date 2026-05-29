package prompt

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	primary = lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#33B5FF"} // azul
	accent  = lipgloss.AdaptiveColor{Light: "#10B981", Dark: "#4ADE80"} // verde
	warning = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"} // amarillo
	subtle  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
)

func devcontainerTheme() *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Title = t.Focused.Title.Foreground(primary).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(subtle)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(accent).Bold(true)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(primary).SetString("▸ ")
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(primary).SetString("▸ ")
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(accent).SetString("✓ ")
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(subtle).SetString("○ ")
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(lipgloss.Color("0")).Background(primary).Bold(true)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(warning)

	t.Blurred.Title = t.Blurred.Title.Foreground(subtle)
	t.Blurred.Description = t.Blurred.Description.Foreground(subtle)

	return t
}

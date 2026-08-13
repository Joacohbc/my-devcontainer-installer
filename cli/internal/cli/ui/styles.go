package ui

import "github.com/charmbracelet/lipgloss"

// ── Adaptive color palette (light/dark terminal) ──────────────
// Aligned with internal/cli/prompt/theme.go
var (
	ColorPrimary = lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#33B5FF"} // blue
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#10B981", Dark: "#4ADE80"} // green
	ColorWarning = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"} // yellow
	ColorDanger  = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"} // red
	ColorInfo    = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#22D3EE"} // cyan
	ColorSubtle  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"} // gray
)

// ── Reusable semantic styles ────────────────────────────────
var (
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleDanger  = lipgloss.NewStyle().Foreground(ColorDanger)
	StyleInfo    = lipgloss.NewStyle().Foreground(ColorInfo)
	StyleSubtle  = lipgloss.NewStyle().Foreground(ColorSubtle)
	StyleBold    = lipgloss.NewStyle().Bold(true)
	StyleHeader  = lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)

	// Composites (icons)
	styleArrow = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	styleCheck = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	styleBang  = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
)

// ── Pre-rendered icons ────────────────────────────────────────
var (
	IconArrow = styleArrow.Render("==>")
	IconCheck = styleCheck.Render("✓")
	IconBang  = styleBang.Render("!")
)

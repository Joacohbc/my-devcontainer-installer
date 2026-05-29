package ui

import "github.com/charmbracelet/lipgloss"

// ── Paleta de colores adaptativa (light/dark terminal) ──────────────
// Alineada con internal/cli/prompt/theme.go
var (
	ColorPrimary = lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#33B5FF"} // azul
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#10B981", Dark: "#4ADE80"} // verde
	ColorWarning = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"} // amarillo
	ColorDanger  = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"} // rojo
	ColorInfo    = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#22D3EE"} // cyan
	ColorSubtle  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"} // gris
)

// ── Estilos semánticos reutilizables ────────────────────────────────
var (
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleDanger  = lipgloss.NewStyle().Foreground(ColorDanger)
	StyleInfo    = lipgloss.NewStyle().Foreground(ColorInfo)
	StyleSubtle  = lipgloss.NewStyle().Foreground(ColorSubtle)
	StyleBold    = lipgloss.NewStyle().Bold(true)
	StyleHeader  = lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)

	// Compuestos (iconos)
	styleArrow = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	styleCheck = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	styleBang  = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
)

// ── Iconos pre-renderizados ────────────────────────────────────────
var (
	IconArrow = styleArrow.Render("==>")
	IconCheck = styleCheck.Render("✓")
	IconBang  = styleBang.Render("!")
)

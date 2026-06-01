package ui

import (
	"fmt"
	"strings"
)

var barLine = StyleSubtle.Render(strings.Repeat("─", 64))

func Log(s string)     { fmt.Printf("%s %s\n", IconArrow, s) }
func Ok(s string)      { fmt.Printf("%s   %s\n", IconCheck, s) }
func Warn(s string)    { fmt.Printf("%s   %s\n", IconBang, s) }
func Success(s string) { fmt.Println(StyleSuccess.Bold(true).Render(s)) }
func Bar()             { fmt.Println(barLine) }

// ── Helpers Printf-style (reemplazan color.Green/Yellow/Red/Cyan) ───
func Green(format string, a ...any)  { fmt.Println(StyleSuccess.Render(fmt.Sprintf(format, a...))) }
func Yellow(format string, a ...any) { fmt.Println(StyleWarning.Render(fmt.Sprintf(format, a...))) }
func Red(format string, a ...any)    { fmt.Println(StyleDanger.Render(fmt.Sprintf(format, a...))) }
func Cyan(format string, a ...any)   { fmt.Println(StyleInfo.Render(fmt.Sprintf(format, a...))) }

// ── Sprint variants (return string, no imprimen) ───────────────────
func GreenS(format string, a ...any) string  { return StyleSuccess.Render(fmt.Sprintf(format, a...)) }
func YellowS(format string, a ...any) string { return StyleWarning.Render(fmt.Sprintf(format, a...)) }
func RedS(format string, a ...any) string    { return StyleDanger.Render(fmt.Sprintf(format, a...)) }
func CyanS(format string, a ...any) string   { return StyleInfo.Render(fmt.Sprintf(format, a...)) }

// ── Inline styling (return string) ─────────────────────────────────
func Bold(s string) string   { return StyleBold.Render(s) }
func Subtle(s string) string { return StyleSubtle.Render(s) }
func Header(s string)        { fmt.Println(StyleHeader.Render(s)) }

// ── Patrones repetidos ─────────────────────────────────────────────
func Done()      { fmt.Print(StyleSuccess.Bold(true).Render("\nDone.") + "\n\n") }
func Cancelled() { Yellow("Cancelled.") }

func NewLine()                { fmt.Println() }
func Print(s string)          { fmt.Print(s) }
func HeaderS(s string) string { return StyleHeader.Render(s) }

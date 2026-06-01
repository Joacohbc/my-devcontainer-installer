package ui

import (
	"fmt"
	"os"

	sv "github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

// Console is the terminal-backed implementation of service.Reporter and
// service.Prompter. It bridges the service layer to the package-level ui
// helpers, so commands never need to define their own adapters.
type Console struct{}

var (
	_ sv.Reporter = Console{}
	_ sv.Prompter = Console{}
)

func (Console) Info(format string, args ...any)    { Log(fmt.Sprintf(format, args...)) }
func (Console) Warn(format string, args ...any)    { Yellow(format, args...) }
func (Console) Success(format string, args ...any) { Green(format, args...) }
func (Console) Error(format string, args ...any)   { Red(format, args...) }

func (Console) Fatal(format string, args ...any) {
	fmt.Fprintln(os.Stderr, RedS(format, args...))
}

func (Console) Debug(format string, args ...any) {
	fmt.Fprintln(os.Stderr, Subtle(fmt.Sprintf(format, args...)))
}

func (Console) Ask(prompt string) (string, error) {
	return Input(prompt, "", nil)
}

func (Console) AskDefault(prompt string, initial string, validate func(string) error) (string, error) {
	return Input(prompt, initial, validate)
}

func (Console) Confirm(prompt string) (bool, error) {
	return Confirm(prompt, false)
}

func (Console) ConfirmDefault(prompt string, initial bool) (bool, error) {
	return Confirm(prompt, initial)
}

func (Console) Multiselect(prompt string, choices []sv.Option, initial []sv.Option) ([]sv.Option, error) {
	return Multiselect(prompt, choices, initial)
}

func (Console) Select(prompt string, choices []sv.Option, initial sv.Option) (sv.Option, error) {
	return Select(prompt, choices, initial)
}

func (Console) Wizard(build func(*sv.State) []sv.Step) (*sv.State, error) {
	return NewStepper(build).Run()
}

func (Console) Done() {
	Done()
}

func (Console) Cancelled() {
	Cancelled()
}

func (Console) Header(format string, args ...any) {
	Header(fmt.Sprintf(format, args...))
}

func (Console) Bar() {
	Bar()
}

func (Console) NewLine()                { NewLine() }
func (Console) Print(s string)          { Print(s) }
func (Console) HeaderS(s string) string { return HeaderS(s) }

func (Console) Bold(s string) string {
	return Bold(s)
}

func (Console) Subtle(s string) string {
	return Subtle(s)
}

func (Console) SuccessS(format string, args ...any) string {
	return GreenS(format, args...)
}

func (Console) WarnS(format string, args ...any) string {
	return YellowS(format, args...)
}

func (Console) ErrorS(format string, args ...any) string {
	return RedS(format, args...)
}

func (Console) InfoS(format string, args ...any) string {
	return CyanS(format, args...)
}

func (Console) Ok(s string) {
	Ok(s)
}

func (Console) Log(s string) {
	Log(s)
}

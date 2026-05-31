package service

// Option is a single selectable choice presented by a Prompter. Value is the
// stable identity returned on selection; Label is what the user sees.
type Option struct {
	Value string
	Label string
}

// Reporter is the output side of the service↔cli boundary: services emit
// progress and results through it and never touch the terminal directly.
type Reporter interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Success(format string, args ...any)
	Error(format string, args ...any)
	Fatal(format string, args ...any)
	Debug(format string, args ...any)
}

// Prompter is the input side of the boundary: services request user decisions
// through it. Implementations live in cli/ui; aborts surface as an error the
// caller maps to cancellation.
type Prompter interface {
	Ask(prompt string) (string, error)
	Confirm(prompt string) (bool, error)
	Select(prompt string, choices []Option, initial Option) (Option, error)
	Multiselect(prompt string, choices []Option, initial []Option) ([]Option, error)
	Wizard(build func(*State) []Step) (*State, error)
}

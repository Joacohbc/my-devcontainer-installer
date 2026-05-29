// Package service holds whole-process business logic (the generate/update/
// destroy pipelines). It orchestrates the domain layer and reports progress
// through the Reporter port; it never prints directly nor interacts with the
// user. Interactive Q&A (flags, wizard, confirmations) lives in the cli layer,
// which resolves all decisions and feeds them to services as data.
package service

// Reporter is the one-way progress/diagnostics port. The cli layer implements
// it over its console UI; tests collect messages into a slice. It replaces the
// former global domain event channel.
type Reporter interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Success(format string, args ...any)
}

// NopReporter is a Reporter that discards everything. Useful as a default and
// in tests that don't assert on output.
type NopReporter struct{}

func (NopReporter) Info(string, ...any)    {}
func (NopReporter) Warn(string, ...any)    {}
func (NopReporter) Success(string, ...any) {}

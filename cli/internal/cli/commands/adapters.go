package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
)

// consoleReporter renders service progress messages on the terminal, adapting
// the service.Reporter port to the cli ui package.
type consoleReporter struct{}

func (consoleReporter) Info(format string, args ...any)    { ui.Log(fmt.Sprintf(format, args...)) }
func (consoleReporter) Warn(format string, args ...any)    { ui.Yellow(format, args...) }
func (consoleReporter) Success(format string, args ...any) { ui.Green(format, args...) }

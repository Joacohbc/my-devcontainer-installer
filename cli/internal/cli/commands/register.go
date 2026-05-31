package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/spf13/cobra"
)

// version is the CLI version, injected by NewRootCommand at startup.
var version = "dev"

// console is the package-wide Console instance used by commands to interact
// with the terminal, style output, and prompt users.
var console = ui.Console{}

// subcommands collects every registered subcommand. Command files append to it
// from their init() func.
var subcommands []*cobra.Command

func register(c *cobra.Command) {
	subcommands = append(subcommands, c)
}

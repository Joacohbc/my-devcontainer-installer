// Package commands wires every CLI subcommand into a Cobra command tree. Each
// command file registers itself via register() in an init() func, so the
// package compiles even while individual commands are still being added.
package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/infra/docker"
	"github.com/spf13/cobra"
)

// version is the CLI version, injected by NewRootCommand at startup.
var version = "dev"

// subcommands collects every registered subcommand. Command files append to it
// from their init() func.
var subcommands []*cobra.Command

func register(c *cobra.Command) {
	subcommands = append(subcommands, c)
}

// dockerCapture adapts the infra docker capture (which also returns an error)
// to the domain.CaptureFunc signature used by docker-querying domain helpers.
func dockerCapture(args []string) (int, string, string) {
	status, stdout, stderr, err := docker.DockerCapture(args)
	if err != nil {
		return 1, "", err.Error()
	}
	return status, stdout, stderr
}

// captureFunc returns the shared CaptureFunc closure for domain calls.
func captureFunc() domain.CaptureFunc {
	return dockerCapture
}

package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newContextCommand()) }

func newContextCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Print the container's context and installed tools",
		Long: `devcontainer-cli context — print what an AI agent (or a new teammate) needs to
know about the container: the conventions it runs under and the tools actually
installed in it.

It runs the container's own 'get-devcontainer-context' and streams the result,
so the report always describes the live container rather than what the project
config asked for. Output is ~/CONTEXT.md — generated when the image was built,
from the modules and services that project selected, so it names the actual
package managers, versions and database endpoints — followed by the detected
tools with their versions and the reachable services.

Unlike 'info', which reports Docker metadata (image, ports, mounts), this
reports what is inside the container. Inside a container the same report is one
command away: 'get-devcontainer-context'.`,
		Example: `  # Human-readable report for the current project's container
  devcontainer-cli context

  # Structured output, easier for an agent or a script to consume
  devcontainer-cli context --json

  # A specific container
  devcontainer-cli context --container myproject-devcontainer`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runContext,
	}
	addContextFlags(cmd)
	return cmd
}

// addContextFlags registers the flags 'context' shares with 'agent context', so
// each one is described in exactly one place.
func addContextFlags(cmd *cobra.Command) {
	addContainerFlag(cmd)
	cmd.Flags().Bool("json", false, "Emit structured JSON instead of the human-readable report")
}

func runContext(cmd *cobra.Command, _ []string) error {
	containerName, err := resolveContainer(cmd)
	if err != nil {
		return err
	}
	asJSON, _ := cmd.Flags().GetBool("json")
	svc := service.InspectService{Report: console}
	if err := svc.EnsureDocker(); err != nil {
		return err
	}
	return svc.Context(containerName, asJSON)
}

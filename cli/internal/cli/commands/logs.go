package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newLogsCommand()) }

func newLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [service]",
		Short: "View logs from the project's containers",
		Long: `devcontainer-cli logs — view container logs for the current project, a thin
wrapper over 'docker compose logs'.

With no argument it shows logs for every service in the project; pass a service
name to limit it to one. Use --container to read a single container's logs
directly (bypassing compose), -f to stream them live, and --tail to cap how many
past lines are shown.`,
		Example: `  # All project logs
  devcontainer-cli logs

  # Follow a single service
  devcontainer-cli logs postgres -f

  # Last 100 lines of a specific container
  devcontainer-cli logs --container myproject-app --tail 100`,
		RunE: runLogs,
	}
	cmd.Flags().BoolP("follow", "f", false, "Follow log output")
	cmd.Flags().String("tail", "all", "Number of lines to show from the end of the logs")
	addContainerFlag(cmd)
	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	follow, _ := cmd.Flags().GetBool("follow")
	tail, _ := cmd.Flags().GetString("tail")
	svc := service.InspectService{Report: ui.Console{}}

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd)
		if err != nil {
			return err
		}
		return svc.ContainerLogs(containerName, follow, tail)
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	composeFile, err := resolveProjectComposeFile(cwd)
	if err != nil {
		return err
	}
	return svc.ComposeLogs(composeFile, follow, tail, args)
}

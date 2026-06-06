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
		Short: "View logs from project containers",
		Long:  "devcontainer-cli logs — displays log output from project containers. Wraps 'docker compose logs'.\n\nEssential for debugging container startup issues, checking application output, or monitoring background service behavior within the devcontainer.\n\nScope: Active project",
		RunE:  runLogs,
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().BoolP("follow", "f", false, "Follow log output")
	cmd.Flags().String("tail", "all", "Number of lines to show from the end of the logs")
	addContainerFlag(cmd)
	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	wsFlag := workspaceFlag(cmd)
	follow, _ := cmd.Flags().GetBool("follow")
	tail, _ := cmd.Flags().GetString("tail")
	svc := service.InspectService{Report: ui.Console{}}

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return err
		}
		return svc.ContainerLogs(containerName, follow, tail)
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	composeFile, err := resolveProjectComposeFileWithWorkspace(cwd, wsFlag)
	if err != nil {
		return err
	}
	return svc.ComposeLogs(composeFile, follow, tail, args)
}

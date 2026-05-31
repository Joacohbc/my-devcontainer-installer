package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newShellCommand()) }

func newShellCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell [flags] [-- command args...]",
		Short: "Open an interactive shell in the devcontainer",
		Long:  `devcontainer-cli shell — shortcut for docker exec -it <container> <shell>`,
		RunE:  runShell,
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().String("user", "", "User to run the command as (e.g. root)")
	addContainerFlag(cmd)
	return cmd
}

func runShell(cmd *cobra.Command, args []string) error {
	wsFlag := workspaceFlag(cmd)
	userFlag, _ := cmd.Flags().GetString("user")

	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return err
	}

	svc := service.InspectService{Report: ui.Console{}}
	return svc.Shell(containerName, userFlag, args)
}

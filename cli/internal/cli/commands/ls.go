package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newLsCommand()) }

func newLsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls [container_path]",
		Short: "List files and directories inside the devcontainer",
		Long: `devcontainer-cli ls — wrapper for ls command inside the devcontainer

Supports real-time dynamic completion for paths inside the container.`,
		SilenceUsage:      true,
		ValidArgsFunction: runLsCompletion,
		RunE:              runLs,
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().BoolP("all", "a", false, "Show hidden files (ls -a)")
	cmd.Flags().BoolP("long", "l", false, "Use a long listing format (ls -l)")
	addContainerFlag(cmd)
	return cmd
}

func runLs(cmd *cobra.Command, args []string) error {
	containerPath := "."
	if len(args) > 0 {
		containerPath = args[0]
	}

	wsFlag := workspaceFlag(cmd)
	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return err
	}

	all, _ := cmd.Flags().GetBool("all")
	long, _ := cmd.Flags().GetBool("long")

	svc := service.InspectService{Report: ui.Console{}}
	return svc.Ls(containerName, containerPath, all, long)
}

func runLsCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	wsFlag := workspaceFlag(cmd)
	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeContainerPath(containerName, toComplete)
}

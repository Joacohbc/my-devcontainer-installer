package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newCopyCommand()) }

func newCopyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "copy [src_local_path] [dest_container_path]",
		Aliases: []string{"cp"},
		Short:   "Copy a file or directory from the host to the container",
		Long: `devcontainer-cli copy — copy a local file or directory into the running devcontainer

Supports real-time dynamic completion for paths inside the container.`,
		Args:              cobra.ExactArgs(2),
		SilenceUsage:      true,
		ValidArgsFunction: runCopyCompletion,
		RunE:              runCopy,
	}
	cmd.Flags().StringP("workspace", "w", "", "Workspace name")
	addContainerFlag(cmd)
	return cmd
}

func runCopy(cmd *cobra.Command, args []string) error {
	localPath := args[0]
	containerPath := args[1]

	wsFlag, _ := cmd.Flags().GetString("workspace")
	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return err
	}

	svc := service.InspectService{Report: ui.Console{}}
	return svc.Copy(containerName, localPath, containerPath)
}

func runCopyCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	if len(args) == 1 {
		wsFlag, _ := cmd.Flags().GetString("workspace")
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeContainerPath(containerName, toComplete)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

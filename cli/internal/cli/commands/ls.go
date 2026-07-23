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
		Short: "List files inside the devcontainer",
		Long: `devcontainer-cli ls — list files and directories inside the running devcontainer,
a convenience wrapper over 'ls' run via docker exec.

The optional path is interpreted inside the container and defaults to the current
working directory there; paths tab-complete in real time. -a shows hidden entries
and -l switches to a long listing.`,
		Example: `  # List the default working dir
  devcontainer-cli ls

  # Long listing of a path, including hidden files
  devcontainer-cli ls -la /home/devuser`,
		SilenceUsage:      true,
		ValidArgsFunction: runLsCompletion,
		RunE:              runLs,
	}
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

	containerName, err := resolveContainer(cmd)
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
	containerName, err := resolveContainer(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeContainerPath(containerName, toComplete)
}

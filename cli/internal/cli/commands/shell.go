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
	cmd.Flags().String("type", "", "Shell to open: bash, zsh or sh (default: auto-detect)")
	addContainerFlag(cmd)

	_ = cmd.RegisterFlagCompletionFunc("type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"bash", "zsh", "sh"}, cobra.ShellCompDirectiveNoFileComp
	})

	_ = cmd.RegisterFlagCompletionFunc("user", func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		wsFlag := workspaceFlag(cmd)
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return service.InspectService{Report: ui.Console{}}.ListUsers(containerName), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runShell(cmd *cobra.Command, args []string) error {
	wsFlag := workspaceFlag(cmd)
	userFlag, _ := cmd.Flags().GetString("user")
	shellType, _ := cmd.Flags().GetString("type")

	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return err
	}

	// With explicit args, --type prepends the shell to the raw command line;
	// with no args it picks the login shell the service launcher opens.
	command := args
	if shellType != "" && len(args) > 0 {
		command = append([]string{shellType}, args...)
	}

	svc := service.InspectService{Report: ui.Console{}}
	return svc.Shell(containerName, userFlag, shellType, command)
}

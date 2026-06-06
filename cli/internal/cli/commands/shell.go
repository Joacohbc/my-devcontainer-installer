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
		Long:  "devcontainer-cli shell — opens an interactive shell session in the devcontainer. Shortcut for 'docker exec -it <container> <shell>'.\n\nWhile SSH is the recommended way to connect, this provides a fallback method to execute commands directly inside the container when SSH is unavailable or being configured.\n\nScope: Container\n\nExamples:\n  devcontainer-cli shell\n  devcontainer-cli shell -w my-workspace\n  devcontainer-cli shell --user root\n  devcontainer-cli shell --type bash",
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

	command := args
	if shellType != "" {
		command = append([]string{shellType}, args...)
	}

	svc := service.InspectService{Report: ui.Console{}}
	return svc.Shell(containerName, userFlag, command)
}

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
		Long: `devcontainer-cli shell — shortcut for docker exec -it <container> <shell>

Pass -T/--no-tty to drop the pseudo-TTY (docker exec -i) when piping a command's
output to a file, e.g.:

  devcontainer-cli shell -c <ws>-postgres -T -- pg_dump -U devuser devdb > dump.sql`,
		RunE: runShell,
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().String("user", "", "User to run the command as (interactive shell defaults to devuser; ignored when an explicit command is passed)")
	cmd.Flags().String("type", "", "Shell to open: bash, zsh or sh (interactive shell defaults to zsh)")
	cmd.Flags().BoolP("no-tty", "T", false, "Disable pseudo-TTY allocation (use when piping output to a file, e.g. a DB dump)")
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

// shellInteractiveDefaults applies the interactive-shell defaults: when no
// explicit command is given, an unset --user becomes devuser and an unset --type
// becomes zsh (the service launcher then cd's into that user's home). With an
// explicit command the flags are left untouched so commands against containers
// without a devuser/zsh (e.g. `shell -c <ws>-postgres -- pg_dump ...`) keep
// working.
func shellInteractiveDefaults(user, shellType string, userSet, typeSet, hasCommand bool) (string, string) {
	if hasCommand {
		return user, shellType
	}
	if !userSet {
		user = "devuser"
	}
	if !typeSet {
		shellType = "zsh"
	}
	return user, shellType
}

func runShell(cmd *cobra.Command, args []string) error {
	wsFlag := workspaceFlag(cmd)
	userFlag, _ := cmd.Flags().GetString("user")
	shellType, _ := cmd.Flags().GetString("type")
	noTTY, _ := cmd.Flags().GetBool("no-tty")

	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return err
	}

	userFlag, shellType = shellInteractiveDefaults(
		userFlag, shellType,
		cmd.Flags().Changed("user"), cmd.Flags().Changed("type"),
		len(args) > 0,
	)

	// With explicit args, --type prepends the shell to the raw command line;
	// with no args it picks the login shell the service launcher opens.
	command := args
	if shellType != "" && len(args) > 0 {
		command = append([]string{shellType}, args...)
	}

	svc := service.InspectService{Report: ui.Console{}}
	return svc.Shell(containerName, userFlag, shellType, command, noTTY)
}

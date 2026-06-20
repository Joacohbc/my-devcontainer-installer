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
		Short: "Open a shell (or run a command) in the devcontainer",
		Long: `devcontainer-cli shell — open an interactive shell in the running devcontainer,
or run a one-off command in it. A convenience wrapper over 'docker exec -it'.

With no command after '--' it opens a login shell. Anything after '--' is run
inside the container instead and its exit code is propagated, which makes 'shell'
handy for scripting against the container.

Interactive defaults (only when no command is given): the shell runs as devuser
with zsh. With an explicit command those defaults are left alone, so commands
work against containers that have no devuser/zsh (e.g. a database container).

Flags:
  --container NAME  Target a specific container instead of the resolved one.
  --workspace NAME  Resolve the container from another workspace.
  --user USER       Run as this user (interactive default: devuser; ignored once
                    an explicit command is given). Tab-completes container users.
  --type SHELL      Shell to open: bash, zsh or sh (interactive default: zsh).
  -T, --no-tty      Drop the pseudo-TTY (docker exec -i) — use it when piping a
                    command's output to a file, so the stream isn't mangled.`,
		Example: `  # Interactive shell as devuser
  devcontainer-cli shell

  # Run a one-off command
  devcontainer-cli shell -- go version

  # Pipe a DB dump out without a TTY
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

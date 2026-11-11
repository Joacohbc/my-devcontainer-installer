package commands

import (
	"fmt"
	"os"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

func newShellCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell [flags] [-- command args...]",
		Short: "Open a shell (or run a command) in the devcontainer",
		Long: `devcontainer-cli access shell — open an interactive shell in the running devcontainer,
or run a one-off command in it. A convenience wrapper over 'docker exec -it'.

With no command after '--' it opens a login shell. Anything after '--' is run
inside the container instead and its exit code is propagated, which makes 'shell'
handy for scripting against the container.

Interactive defaults (only when no command is given): the shell runs as devuser
with zsh. With an explicit command those defaults are left alone, so commands
work against containers that have no devuser/zsh (e.g. a database container).
Pass -T when piping a command's output to a file so the stream isn't mangled.

An explicit command is exec'd directly, NOT through a shell. That is enough for
most of them: the image declares its toolchain PATH as environment (uv, node,
pnpm, cargo, bun, go, ~/.local/bin), which a bare 'docker exec' inherits. Wrap a
command in a login bash — 'shell -- bash -lc "cd /srv && ./build.sh | tee log"' —
when it needs pipes, '&&', globs or 'cd', or the container's pip/npm shell
indirections. Use bash, not zsh: a non-interactive 'zsh -lc' skips ~/.zshrc.

The exception is an image built before that declaration existed, which carries
the PATH only in devuser's rc files; there a login bash is also how a toolchain
is found, and 'image update --rebuild' fixes it for good.

Nothing in these images declares a WORKDIR, so a command lands in '/' and an
interactive shell in devuser's home — never in the project. Pass -w to run in
the project's workspace mount instead, which is what a build or a test command
almost always wants and what saves wrapping one in a login bash just to 'cd'.

Pass --via USER@HOST (with --container) to reach a container on a different
Docker host through an existing SSH connection to it — unlike 'ssh --via' this
needs nothing installed or configured in the container itself, since it rides
entirely on the 'docker exec' channel (no ssh-into-container step).`,
		Example: `  # Interactive shell as devuser
  devcontainer-cli access shell

  # Run a one-off command (in '/', the container default)
  devcontainer-cli access shell -- go version

  # Run it in the project instead
  devcontainer-cli access shell -w --user devuser -- go test ./...

  # The image PATH is inherited, so a plain command needs no login shell
  devcontainer-cli access shell --user devuser -- uv pip install requests

  # Shell syntax still needs one — but -w already did the cd
  devcontainer-cli access shell -w --user devuser -- bash -lc 'pnpm install && pnpm build'

  # Pipe a DB dump out without a TTY
  devcontainer-cli access shell -c <ws>-postgres -T -- pg_dump -U devuser devdb > dump.sql

  # Shell into a container on a different Docker host
  devcontainer-cli access shell --via me@docker-host -c dc-base`,
		RunE: runShell,
	}
	addShellFlags(cmd)
	return cmd
}

// addShellFlags registers the flags runShell reads. It is shared with the
// 'agent exec' facade, which reuses runShell but requires a command.
func addShellFlags(cmd *cobra.Command) {
	cmd.Flags().String("user", "", "User to run the command as; always honoured, but only an interactive shell defaults it to devuser")
	cmd.Flags().String("type", "", "Shell to open: bash, zsh or sh (interactive shell defaults to zsh)")
	cmd.Flags().BoolP("no-tty", "T", false, "Disable pseudo-TTY allocation (use when piping output to a file, e.g. a DB dump); implied when stdin is not a terminal")
	cmd.Flags().BoolP("workdir", "w", false, "Run in the project's workspace mount instead of the container default; only the devcontainer has that directory, so a database container needs it left off")
	cmd.Flags().String("via", "", "Reach the container through an existing SSH connection to its Docker host (requires --container): USER@HOST or an ssh-config alias")
	addContainerFlag(cmd)

	_ = cmd.RegisterFlagCompletionFunc("type", staticCompletion("bash", "zsh", "sh"))

	_ = cmd.RegisterFlagCompletionFunc("user", func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		containerName, err := resolveContainer(cmd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return service.InspectService{Report: ui.Console{}}.ListUsers(containerName), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("via", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listSshHosts(), cobra.ShellCompDirectiveNoFileComp
	})
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

// stdinIsTerminal reports whether stdin is a real terminal (not merely a
// character device such as /dev/null). Docker refuses `exec -t` without one
// ("cannot attach stdin to a TTY-enabled container"), which is exactly the
// situation of an agent or a pipeline calling 'agent exec'.
func stdinIsTerminal() bool {
	fd := os.Stdin.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// needsNoTTY drops the pseudo-TTY when asked to, or when stdin is not a
// terminal, since a TTY request would then fail before the command runs.
func needsNoTTY(noTTYFlag, stdinTerminal bool) bool {
	return noTTYFlag || !stdinTerminal
}

func runShell(cmd *cobra.Command, args []string) error {
	userFlag, _ := cmd.Flags().GetString("user")
	shellType, _ := cmd.Flags().GetString("type")
	noTTYFlag, _ := cmd.Flags().GetBool("no-tty")
	noTTY := needsNoTTY(noTTYFlag, stdinIsTerminal())
	via, _ := cmd.Flags().GetString("via")
	if via != "" && !cmd.Flags().Changed("container") {
		return fmt.Errorf("--via requires --container <name>: there is no local compose project describing a container on a remote host")
	}

	containerName, err := resolveContainer(cmd)
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

	workdir, err := shellWorkdir(cmd)
	if err != nil {
		return err
	}

	svc := service.InspectService{Report: ui.Console{}}
	return service.WithHostOverride(via, func() error {
		return svc.Shell(containerName, userFlag, shellType, workdir, command, noTTY)
	})
}

// shellWorkdir resolves --workdir to the project's in-container mount path, or
// "" when the flag is off. The path comes from the workspace the current
// directory resolves to, not from the target container, so pointing --container
// at something without that mount is the caller's call to make.
func shellWorkdir(cmd *cobra.Command) (string, error) {
	if w, _ := cmd.Flags().GetBool("workdir"); !w {
		return "", nil
	}
	cwd, err := currentDir()
	if err != nil {
		return "", err
	}
	return types.WorkspaceDir(resolveWorkspace(cwd)), nil
}

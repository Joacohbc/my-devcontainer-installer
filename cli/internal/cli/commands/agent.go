package commands

import (
	"encoding/json"
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newAgentCommand()) }

func newAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Drive the CLI from an AI agent with a small, non-interactive command set",
		Long: `devcontainer-cli agent — the agent-facing face of this CLI: a handful of
high-level commands that never open a wizard and never wait for an answer.

Every subcommand here is a thinner, safer spelling of a command you can also
run by hand. The difference is the defaults: prompts are off, so a missing
value is an error instead of a question, and no invocation can leave an
unattended session hanging on a TUI. Destructive work still needs --yes.

Start with 'agent cli-info', which prints the live catalogue — modules,
services, profiles, skills, how to add your own scripts — so a create command
can be composed without guessing at ids.

The human commands stay available and are the escape hatch: 'compose' for any
compose verb, 'access ssh --via' for another Docker host, 'run' for a throwaway
container, 'config' for global settings.`,
		Example: `  # Read the catalogue, then create an environment from it
  devcontainer-cli agent cli-info --json
  devcontainer-cli agent create --modules nodejs,pnpm --services postgres

  # Work inside it
  devcontainer-cli agent exec -- go test ./...
  devcontainer-cli agent list /home/devuser --json

  # Tear it down
  devcontainer-cli agent destroy --yes`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newAgentCLIInfoCommand(),
		newAgentCreateCommand(),
		newAgentRunCommand(),
		newAgentSshCommand(),
		newAgentExecCommand(),
		newAgentForwardCommand(),
		newAgentListCommand(),
		newAgentDestroyCommand(),
	)
	return cmd
}

// agentDefaults flips the flags the agent facade defaults differently from the
// human command, leaving anything the caller passed explicitly alone — so
// '--no-interactive=false' still opts back into prompting.
func agentDefaults(cmd *cobra.Command, values map[string]string) error {
	_ = cmd.InheritedFlags()
	for name, value := range values {
		if cmd.Flags().Changed(name) {
			continue
		}
		if err := cmd.Flags().Set(name, value); err != nil {
			return fmt.Errorf("could not apply the agent default for --%s: %w", name, err)
		}
	}
	return nil
}

// agentNonInteractive is the PreRunE shared by the subcommands whose only
// agent-facing change is that they never prompt.
func agentNonInteractive(cmd *cobra.Command, _ []string) error {
	return agentDefaults(cmd, map[string]string{flagNoInteractive: "true"})
}

func newAgentSshCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [flags] [-- command args...]",
		Short: "Open an SSH session into the project's devcontainer",
		Long: `devcontainer-cli agent ssh — connect to the project's devcontainer over
SSH.

Same as 'access ssh', with prompting off and ephemeral mode on by default (bypassing
~/.ssh/config Host block modifications). Give it a command after '--': without
one it opens an interactive session that never returns, which is of no use to
an agent — 'agent exec' is the better tool for running a command anyway, and
this one is for when the task genuinely needs the SSH path (agent forwarding,
a real tty, a tool that shells out to ssh).`,
		Example: `  # Run a command over SSH
  devcontainer-cli agent ssh -- go version

  # Reach the container on another Docker host
  devcontainer-cli agent ssh --via me@docker-host -c mycontainer -- uname -a`,
		SilenceUsage: true,
		PreRunE:      agentSshDefaults,
		RunE:         runSsh,
	}
	addAgentSshFlags(cmd)
	return cmd
}

func agentSshDefaults(cmd *cobra.Command, _ []string) error {
	return agentDefaults(cmd, map[string]string{
		flagNoInteractive: "true",
		"ephemeral":       "true",
	})
}

func addAgentSshFlags(cmd *cobra.Command) {
	addInteractiveFlag(cmd)
	addContainerFlag(cmd)
	cmd.Flags().Bool("ephemeral", false, "Connect directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (default true for agent)")
	cmd.Flags().String("via", "", "Reach the container through an existing SSH connection to its Docker host (requires --container): USER@HOST or an ssh-config alias")
	cmd.Flags().String("key", "", "Private key path (default: the shared managed key under the CLI config dir)")
	cmd.Flags().String("user", sshdefaults.User, "SSH user inside the container")

	_ = cmd.RegisterFlagCompletionFunc("via", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listSshHosts(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.MarkFlagFilename("key")
}

func newAgentExecCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [flags] -- command args...",
		Short: "Run a command inside the devcontainer",
		Long: `devcontainer-cli agent exec — run a one-off command in the running
devcontainer and propagate its exit code. This is the normal way to do work
inside a container.

Same as 'access shell --', with two differences. A command is required: 'access shell' with no
command opens a login shell, which for an unattended agent is a hang rather than
a session. And it runs as devuser, who owns the workspace files — 'access shell' with a
command runs as root, which silently leaves root-owned files behind in the bind
mount, on the real project directory.

That default is for the project's own devcontainer. Another container in the
stack (a database) has no devuser, so those need an explicit --user.

A command lands in '/' by default — nothing in the image declares a WORKDIR — so
pass -w to run it in the project's workspace mount, which is what a build or a
test almost always means.

The command is exec'd directly, NOT through a shell. Wrap it in a login bash —
'agent exec -- bash -lc "pnpm install | tee log"' — whenever it needs pipes,
'&&', globs, or the container's pip/npm indirections. Finding a binary is not one
of those reasons: the image declares its toolchain PATH, so
'agent exec -- uv pip install x' works as it stands. Neither is changing
directory, once -w is doing it.`,
		Example: `  # A plain command, as devuser, in the project
  devcontainer-cli agent exec -w -- go test ./...

  # Shell syntax still needs a login bash; -w already did the cd
  devcontainer-cli agent exec -w -- bash -lc 'pnpm install && pnpm build'

  # Something that genuinely needs root
  devcontainer-cli agent exec --user root -- apt-get install -y tree

  # A database container has no devuser (and no workspace): name the user it has
  devcontainer-cli agent exec -c myapp-postgres --user postgres -T -- pg_dump devdb > dump.sql`,
		Args:         agentExecArgs,
		SilenceUsage: true,
		PreRunE:      agentExecDefaults,
		RunE:         runShell,
	}
	addAgentExecFlags(cmd)
	return cmd
}

// agentExecDefaults runs the command as devuser unless the caller named another
// user, and enforces non-interactive mode.
func agentExecDefaults(cmd *cobra.Command, _ []string) error {
	return agentDefaults(cmd, map[string]string{
		"user":            sshdefaults.User,
		flagNoInteractive: "true",
	})
}

func addAgentExecFlags(cmd *cobra.Command) {
	cmd.Flags().String("user", "", "User to run the command as; always honoured, but only an interactive shell defaults it to devuser")
	cmd.Flags().BoolP("no-tty", "T", false, "Disable pseudo-TTY allocation (use when piping output to a file, e.g. a DB dump); implied when stdin is not a terminal")
	cmd.Flags().BoolP("workdir", "w", false, "Run in the project's workspace mount instead of the container default; only the devcontainer has that directory, so a database container needs it left off")
	cmd.Flags().String("via", "", "Reach the container through an existing SSH connection to its Docker host (requires --container): USER@HOST or an ssh-config alias")
	addContainerFlag(cmd)
	addInteractiveFlag(cmd)

	_ = cmd.RegisterFlagCompletionFunc("user", func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		containerName, err := resolveContainer(cmd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return service.InspectService{Report: console}.ListUsers(containerName), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("via", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listSshHosts(), cobra.ShellCompDirectiveNoFileComp
	})
}

// agentExecArgs requires the command 'shell' treats as optional.
func agentExecArgs(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent exec requires a command, e.g. 'agent exec -- go test ./...'; " +
			"use 'access shell' if you really want an interactive login shell")
	}
	return nil
}

func newAgentForwardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forward [port_mapping]",
		Short: "Forward a port between this machine and the running container over SSH",
		Long: `devcontainer-cli agent forward — open an SSH tunnel between 127.0.0.1 here and
a port inside the running container, in either direction. A mapping that starts
with an IP of this machine ('0.0.0.0:8000:8000') listens there instead of on
127.0.0.1, so other machines can reach the tunnel.

Same as 'access forward', with prompting off and ephemeral mode on by default,
so the port mapping is given as an argument and connects directly via SSH.

By default the tunnel listens here and reaches into the container. Prefix the
mapping with 'reverse:' (or pass --reverse) to turn it around, so a service
running on this machine becomes reachable inside the container. A reverse
tunnel takes no bind address: it listens inside the container, where its SSH
server keeps it on the loopback.

It stays in the FOREGROUND until interrupted: run it in the background if you
need to keep working. A port that should always be reachable belongs in the
project instead — regenerate with 'agent create --ports <spec>'.`,
		Example: `  # 127.0.0.1:3000 -> container:3000
  devcontainer-cli agent forward 3000

  # 127.0.0.1:8080 -> container:80
  devcontainer-cli agent forward 8080:80

  # Reach a sibling compose service through the container
  devcontainer-cli agent forward 5432:postgres:5432

  # 100.102.62.110:8000 (an IP of this machine) -> container:8000
  devcontainer-cli agent forward 100.102.62.110:8000:8000

  # Every interface of this machine -> container:8000
  devcontainer-cli agent forward 0.0.0.0:8000:8000

  # container:5432 -> this machine's 5432 (a database running on the host)
  devcontainer-cli agent forward reverse:5432`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		PreRunE:      agentForwardDefaults,
		RunE:         runPortForward,
	}
	addForwardFlags(cmd)
	return cmd
}

func agentForwardDefaults(cmd *cobra.Command, _ []string) error {
	return agentDefaults(cmd, map[string]string{
		flagNoInteractive: "true",
		"ephemeral":       "true",
	})
}

func newAgentListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [container_path]",
		Short: "List files inside the devcontainer",
		Long: `devcontainer-cli agent list — list a directory inside the running
devcontainer.

Same as 'inspect ls'. The path is interpreted inside the container and defaults to the
working directory there. With --json each entry comes back as an object with a
name and whether it is a directory, which is easier to consume than parsing
'ls' output. -a/--all includes hidden entries.

This lists files, not environments. For what a container has installed use
'inspect context'; for which containers exist use 'inspect status --all'.`,
		Example: `  # The container's working directory
  devcontainer-cli agent list

  # Structured output for a specific path
  devcontainer-cli agent list /home/devuser --json

  # Structured output including hidden files
  devcontainer-cli agent list /home/devuser --json -a`,
		Args:              cobra.MaximumNArgs(1),
		SilenceUsage:      true,
		PreRunE:           agentNonInteractive,
		ValidArgsFunction: runLsCompletion,
		RunE:              runAgentList,
	}
	addAgentListFlags(cmd)
	return cmd
}

func addAgentListFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP("all", "a", false, "Show hidden files (ls -a)")
	addContainerFlag(cmd)
	cmd.Flags().Bool("json", false, "Emit one JSON object per entry instead of the raw ls listing")
	addInteractiveFlag(cmd)
}

func runAgentList(cmd *cobra.Command, args []string) error {
	asJSON, _ := cmd.Flags().GetBool("json")
	if !asJSON {
		return runLs(cmd, args)
	}

	containerPath := "."
	if len(args) > 0 {
		containerPath = args[0]
	}
	containerName, err := resolveContainer(cmd)
	if err != nil {
		return err
	}

	all, _ := cmd.Flags().GetBool("all")
	entries, err := service.InspectService{Report: console}.ListDirEntries(containerName, containerPath, all)
	if err != nil {
		return err
	}

	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func newAgentRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Spin up an ephemeral container from a prebuilt remote image (no project files)",
		Long: `devcontainer-cli agent run — start a one-off devcontainer from a prebuilt remote
image without generating project files. Prompting is disabled by default:
every value must come from flags (--profile is required, --name falls back to dc-<profile>).`,
		Example: `  # Run a throwaway Python container
  devcontainer-cli agent run --profile python --name my-task

  # Expose ports on the LAN
  devcontainer-cli agent run --profile nodejs --ports 8080:80 --expose-all`,
		SilenceUsage: true,
		PreRunE:      agentNonInteractive,
		RunE:         runQuickRun,
	}
	cmd.Flags().String("profile", "", "Single [remote]-tagged profile id to run as a prebuilt image, e.g. base, nodejs, python; see 'profile list'")
	cmd.Flags().String("name", "", "Container name (default: dc-<profile>); if it belongs to an existing quick-run container it is reused/restarted, but a name already used by any other container is rejected")
	cmd.Flags().StringSlice("volumes", nil, "Volume mounts (e.g. myvol:/workspace); repeatable or comma-separated")
	cmd.Flags().StringSlice("ports", nil, "Port mappings (e.g. 2222:22); bound to 127.0.0.1 unless --expose-all; repeatable or comma-separated")
	cmd.Flags().Bool("expose-all", false, "Publish ports on all interfaces (0.0.0.0) instead of binding to 127.0.0.1")
	cmd.Flags().Bool("shared-config", true, "Mount the global shared AI/dev tool config volume (devcontainer-shared-config) so logins/sessions persist across containers; --shared-config=false to opt out")
	cmd.Flags().String("registry", "", "Registry prefix override")
	addInteractiveFlag(cmd)

	_ = cmd.RegisterFlagCompletionFunc("profile", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return profileIDs(true), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func newAgentDestroyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the current project and remove what it left behind",
		Long: `devcontainer-cli agent destroy — tear down the project in the current directory
and remove the resources it owns. This cannot be undone.

It brings the stack down with its volumes (all data in them is deleted),
deletes the generated .dc_<workspace>/ directory and devcontainer.config.json,
prunes the project's managed SSH host block and pinned host keys, and removes
the image built for it. An image pulled from a registry is left alone: other
projects on the same profile share it.

Pass -c, --container to instead tear down a standalone or temporal container that
isn't part of a workspace project.

By default nothing outside this project is touched. --all additionally sweeps
every managed container, image, network and volume on the machine, including
other projects' — get the user's agreement before using it.

Because it is irreversible it requires --yes. Preview with --dry-run first.`,
		Example: `  # See what would go
  devcontainer-cli agent destroy --dry-run

  # Tear down this project
  devcontainer-cli agent destroy --yes

  # Tear down a single container
  devcontainer-cli agent destroy --container dc-python --yes

  # Also sweep every other managed resource on the machine
  devcontainer-cli agent destroy --all --yes`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		PreRunE:      agentNonInteractive,
		RunE:         runAgentDestroy,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	addContainerFlag(cmd)
	cmd.Flags().Bool("all", false, "Also remove every OTHER managed container, image, network and volume on this machine")
	cmd.Flags().Bool("dry-run", false, "Report what would be removed without removing anything")
	return cmd
}

func runAgentDestroy(cmd *cobra.Command, _ []string) error {
	if cmd.Flags().Changed("container") {
		container, _ := cmd.Flags().GetString("container")
		return runDestroyContainer(cmd, console, container)
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	target, cfg := destroyTargetFor(cwd)

	image := ""
	if cfg != nil {
		image = cfg.Image
	}

	all, _ := cmd.Flags().GetBool("all")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	svc := service.AgentService{Report: console, Prompt: console}
	return svc.Clean(target, image, service.AgentCleanOptions{
		All:         all,
		DryRun:      dryRun,
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
}

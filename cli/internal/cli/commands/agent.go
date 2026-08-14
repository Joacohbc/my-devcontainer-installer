package commands

import (
	"encoding/json"
	"fmt"
	"strings"

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
compose verb, 'ssh --via' for another Docker host, 'run' for a throwaway
container, 'config' for global settings.`,
		Example: `  # Read the catalogue, then create an environment from it
  devcontainer-cli agent cli-info --json
  devcontainer-cli agent create --with nodejs,pnpm --service postgres

  # Work inside it
  devcontainer-cli agent exec -- go test ./...
  devcontainer-cli agent list /home/devuser --json

  # Tear it down
  devcontainer-cli agent clean --yes`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newAgentCLIInfoCommand(),
		newAgentCreateCommand(),
		newAgentConnectCommand(),
		newAgentExecCommand(),
		newAgentForwardCommand(),
		newAgentCopyCommand(),
		newAgentListCommand(),
		newAgentCleanCommand(),
	)
	return cmd
}

// agentDefaults flips the flags the agent facade defaults differently from the
// human command, leaving anything the caller passed explicitly alone — so
// '--no-interactive=false' still opts back into prompting.
func agentDefaults(cmd *cobra.Command, values map[string]string) error {
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

func newAgentConnectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect [flags] [-- command args...]",
		Short: "Open an SSH session into the project's devcontainer",
		Long: `devcontainer-cli agent connect — connect to the project's devcontainer over
SSH, configuring access (key + Host block) the first time.

Same as 'ssh', with prompting off by default. Give it a command after '--':
without one it opens an interactive session that never returns, which is of no
use to an agent — 'agent exec' is the better tool for running a command anyway,
and this one is for when the task genuinely needs the SSH path (agent forwarding,
a real tty, a tool that shells out to ssh).`,
		Example: `  # Run a command over SSH
  devcontainer-cli agent connect -- go version

  # Connect and forward ports for the session
  devcontainer-cli agent connect --forward --ports 3000,8080:80`,
		SilenceUsage: true,
		PreRunE:      agentNonInteractive,
		RunE:         runSsh,
	}
	addSshFlags(cmd)
	return cmd
}

func newAgentExecCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [flags] -- command args...",
		Short: "Run a command inside the devcontainer",
		Long: `devcontainer-cli agent exec — run a one-off command in the running
devcontainer and propagate its exit code. This is the normal way to do work
inside a container.

Same as 'shell --', with two differences. A command is required: 'shell' with no
command opens a login shell, which for an unattended agent is a hang rather than
a session. And it runs as devuser, who owns the workspace files — 'shell' with a
command runs as root, which silently leaves root-owned files behind in the bind
mount, on the real project directory.

That default is for the project's own devcontainer. Another container in the
stack (a database) has no devuser, so those need an explicit --user.

The command is exec'd directly, NOT through a shell. Wrap it in a login bash —
'agent exec -- bash -lc "uv pip install x"' — whenever it needs pipes, '&&',
globs, 'cd', or the container's pip/npm indirections.`,
		Example: `  # A plain command, as devuser
  devcontainer-cli agent exec -- go test ./...

  # Shell syntax and the container's package managers need a login bash
  devcontainer-cli agent exec -- bash -lc 'cd /workspaces/app && pnpm install'

  # Something that genuinely needs root
  devcontainer-cli agent exec --user root -- apt-get install -y tree

  # A database container has no devuser: name the one it does have
  devcontainer-cli agent exec -c myapp-postgres --user postgres -T -- pg_dump devdb > dump.sql`,
		Args:         agentExecArgs,
		SilenceUsage: true,
		PreRunE:      agentExecDefaults,
		RunE:         runShell,
	}
	addShellFlags(cmd)
	return cmd
}

// agentExecDefaults runs the command as devuser unless the caller named another
// user. 'shell' deliberately leaves --user unset with an explicit command so it
// keeps working against containers with no devuser; the facade takes the
// opposite trade, because the caller it serves is a program that would
// otherwise litter the user's own project directory with root-owned files. A
// container without devuser now fails loudly, which beats corrupting a
// checkout quietly.
func agentExecDefaults(cmd *cobra.Command, _ []string) error {
	return agentDefaults(cmd, map[string]string{"user": sshdefaults.User})
}

// agentExecArgs requires the command 'shell' treats as optional.
func agentExecArgs(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent exec requires a command, e.g. 'agent exec -- go test ./...'; " +
			"use 'shell' if you really want an interactive login shell")
	}
	return nil
}

func newAgentForwardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forward [port_mapping]",
		Short: "Forward a host port into the running container over SSH",
		Long: `devcontainer-cli agent forward — open an SSH tunnel from 127.0.0.1 to a port
inside the running container.

Same as 'port-forward', with prompting off by default, so the port mapping has
to be given as an argument rather than picked interactively.

It stays in the FOREGROUND until interrupted: run it in the background if you
need to keep working. A port that should always be reachable belongs in the
project instead — regenerate with 'agent create --ports <spec>'.`,
		Example: `  # 127.0.0.1:3000 -> container:3000
  devcontainer-cli agent forward 3000

  # 127.0.0.1:8080 -> container:80
  devcontainer-cli agent forward 8080:80

  # Reach a sibling compose service through the container
  devcontainer-cli agent forward 5432:postgres:5432`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		PreRunE:      agentNonInteractive,
		RunE:         runPortForward,
	}
	addPortForwardFlags(cmd)
	return cmd
}

func newAgentCopyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy [src] [dest]",
		Short: "Copy files between the host and the container",
		Long: `devcontainer-cli agent copy — copy a file or directory between this machine
and the running devcontainer.

Same as 'copy'. Prefix a path with ':' to mean "inside the container"; the
direction is inferred from which side carries it, and the container is resolved
for you. With --asset it copies one of the CLI's own built-in scripts in
instead ('agent cli-info' lists them).`,
		Example: `  # Host -> container
  devcontainer-cli agent copy ./seed.sql :/home/devuser/seed.sql

  # Container -> host
  devcontainer-cli agent copy :/home/devuser/out.log ./out.log

  # Drop a built-in installer script into the container
  devcontainer-cli agent copy --asset install-claude-code`,
		Args:              copyArgs,
		SilenceUsage:      true,
		ValidArgsFunction: runCopyCompletion,
		RunE:              runCopy,
	}
	addCopyFlags(cmd)
	return cmd
}

func newAgentListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [container_path]",
		Short: "List files inside the devcontainer",
		Long: `devcontainer-cli agent list — list a directory inside the running
devcontainer.

Same as 'ls'. The path is interpreted inside the container and defaults to the
working directory there. With --json each entry comes back as an object with a
name and whether it is a directory, which is easier to consume than parsing
'ls' output; -a and -l are ignored in that mode.

This lists files, not environments. For what a container has installed use
'context'; for which containers exist use 'status --all'.`,
		Example: `  # The container's working directory
  devcontainer-cli agent list

  # Structured output for a specific path
  devcontainer-cli agent list /home/devuser --json`,
		Args:              cobra.MaximumNArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: runLsCompletion,
		RunE:              runAgentList,
	}
	addLsFlags(cmd)
	cmd.Flags().Bool("json", false, "Emit one JSON object per entry instead of raw 'ls' output")
	return cmd
}

// agentDirEntry is one entry of a container directory listing.
type agentDirEntry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
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

	// ListDir runs `ls -1 -p`, where a trailing slash marks a directory.
	names, err := service.InspectService{Report: console}.ListDir(containerName, containerPath)
	if err != nil {
		return err
	}
	entries := make([]agentDirEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, agentDirEntry{
			Name: strings.TrimSuffix(name, "/"),
			Dir:  strings.HasSuffix(name, "/"),
		})
	}

	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func newAgentCleanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Destroy the current project and remove what it left behind",
		Long: `devcontainer-cli agent clean — tear down the project in the current directory
and remove the resources it owns. This cannot be undone.

It brings the stack down with its volumes (all data in them is deleted),
deletes the generated .dc_<workspace>/ directory and devcontainer.config.json,
prunes the project's managed SSH host block and pinned host keys, and removes
the image built for it. An image pulled from a registry is left alone: other
projects on the same profile share it.

By default nothing outside this project is touched. --all additionally sweeps
every managed container, image, network and volume on the machine, including
other projects' — get the user's agreement before using it.

Because it is irreversible it requires --yes. Preview with --dry-run first.`,
		Example: `  # See what would go
  devcontainer-cli agent clean --dry-run

  # Tear down this project
  devcontainer-cli agent clean --yes

  # Also sweep every other managed resource on the machine
  devcontainer-cli agent clean --all --yes`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		PreRunE:      agentNonInteractive,
		RunE:         runAgentClean,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("all", false, "Also remove every OTHER managed container, image, network and volume on this machine")
	cmd.Flags().Bool("dry-run", false, "Report what would be removed without removing anything")
	return cmd
}

func runAgentClean(cmd *cobra.Command, _ []string) error {
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

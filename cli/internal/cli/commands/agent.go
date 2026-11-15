package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service/sshdefaults"
	"github.com/spf13/cobra"
)

// newAgentCommand groups the non-interactive, schema-discoverable surface for
// autonomous agents (Claude Code, Codex, Cursor, Aider). Every command under
// 'agent' sets --no-interactive by default, refuses to hang on a prompt, and
// delegates to the same service layer the human commands use so behavior cannot
// drift between the two surfaces.
func newAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Non-interactive commands for AI agents (discoverable catalogue + one-shot creation)",
		Long: `devcontainer-cli agent — the programmatic interface designed for AI agents
(Claude Code, Codex, Cursor, Aider) and automation scripts.

Every command under 'agent' defaults --no-interactive to true so nothing can
ever block on a terminal prompt. Start with 'agent cli-info --json' to read the
full machine-readable catalogue of base images, supported versions, compose
services, profiles, skills, how to add your own scripts — so a create command
can be composed without guessing at ids.

The human commands stay available and are the escape hatch: 'compose' for any
compose verb, 'access ssh via' for another Docker host, 'run' for a throwaway
container, 'config' for global settings.`,
		Example: `  # Read the catalogue, then create an environment from it
  devcontainer-cli agent cli-info --json
  devcontainer-cli agent create --name my-api --image node:22 --services postgres,redis --json

  # Run a command inside the workspace and open a port tunnel
  devcontainer-cli agent exec -w -- go test ./...
  devcontainer-cli agent forward 3000 &`,
	}

	cmd.AddCommand(newAgentCreateCommand())
	cmd.AddCommand(newAgentCliInfoCommand())
	cmd.AddCommand(newAgentExecCommand())
	cmd.AddCommand(newAgentForwardCommand())
	cmd.AddCommand(newAgentSshCommand())
	return cmd
}

func newAgentSshCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [flags] [-- command args...]",
		Short: "Open an SSH session into the project's devcontainer",
		Long: `devcontainer-cli agent ssh — connect over SSH to the project's devcontainer
with prompting off and ephemeral mode on by default.

Ephemeral mode skips ~/.ssh/config and ~/.ssh/devcontainer-cli.config
entirely: it ensures the shared public key is in devuser's authorized_keys,
reads the container's current IP on the compose network, and runs ssh with
StrictHostKeyChecking=no and UserKnownHostsFile=/dev/null. Pass
--ephemeral=false to opt into the managed Host block instead.

Prefer 'agent exec' for normal command execution; use 'agent ssh' when you
specifically need an SSH transport (remote jump via 'agent ssh via', port
forwarding, a real tty, a tool that shells out to ssh).`,
		Example: `  # Run a command over SSH without touching ~/.ssh/
  devcontainer-cli agent ssh -- go version

  # Reach the container on another Docker host
  devcontainer-cli agent ssh via me@docker-host mycontainer -- uname -a`,
		SilenceUsage: true,
		PreRunE:      agentSshDefaults,
		RunE:         runSsh,
	}
	addAgentSshFlags(cmd)
	cmd.AddCommand(newAgentSshViaCommand())
	return cmd
}

func newAgentSshViaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "via <host> <container> [-- command args...]",
		Short: "Open an SSH session into a container on a remote Docker host",
		Long: `devcontainer-cli agent ssh via — connect to a container on a remote Docker host
over SSH with prompting off and ephemeral mode on by default.`,
		Example: `  # Run a command over SSH on a remote Docker host
  devcontainer-cli agent ssh via me@docker-host mycontainer -- uname -a`,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: completeViaArgs,
		SilenceUsage:      true,
		PreRunE:           agentSshDefaults,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSshWithTarget(cmd, args[0], args[1], true, args[2:])
		},
	}
	addAgentSshSessionFlags(cmd)
	return cmd
}

func agentSshDefaults(cmd *cobra.Command, _ []string) error {
	return applyAgentFlagDefaults(cmd, map[string]string{
		flagNoInteractive: "true",
		"ephemeral":       "true",
	})
}

func addAgentSshSessionFlags(cmd *cobra.Command) {
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("ephemeral", true, "Connect directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (--ephemeral=false opts into ~/.ssh/devcontainer-cli.config)")
	cmd.Flags().String("key", "", "Private key path (default: the shared managed key under the CLI config dir)")
	cmd.Flags().String("user", sshdefaults.User, "SSH user inside the container")
	_ = cmd.MarkFlagFilename("key")
}

func addAgentSshFlags(cmd *cobra.Command) {
	addContainerFlag(cmd)
	addAgentSshSessionFlags(cmd)
}

func newAgentExecCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [flags] -- command args...",
		Short: "Run a command inside the devcontainer",
		Long: `devcontainer-cli agent exec — run a one-off command inside the devcontainer
over 'docker exec' with prompting turned off.

This is the preferred way for an agent to run builds, tests, linters,
migrations, or generators in the container: it needs no sshd, does not
touch ~/.ssh, and preserves exit codes and stdout/stderr verbatim.

Pass -w (--workdir) to run inside the mounted workspace directory; without
-w the command runs in the container's default working directory (usually
/home/devuser, or / for database containers). Make a habit of passing -w for
every build, test, install, or git command — nothing looks broken when you
forget it except that the files you edited are in a different directory, and
an agent can burn several turns debugging why 'go test' says there are no Go
files before noticing it is in /home/devuser. 'devcontainer-cli workdir'
prints the mounted path if you ever want to check it, though you never have
to 'cd' into it once -w is doing it.`,
		Example: `  # Run tests inside the project's workspace directory
  devcontainer-cli agent exec -w -- go test ./...

  # Run a build as root when installing system packages
  devcontainer-cli agent exec --user root -- apt-get update

  # Query a sibling database container (no -w: postgres has no workspace mount)
  devcontainer-cli agent exec -c myproject-postgres -- psql -U devuser -d devdb -c '\dt'`,
		Args:         agentExecArgs,
		SilenceUsage: true,
		PreRunE:      agentExecDefaults,
		RunE:         runShell,
	}
	addAgentExecFlags(cmd)
	cmd.AddCommand(newAgentExecViaCommand())
	return cmd
}

func newAgentExecViaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "via <host> <container> -- command args...",
		Short: "Run a command inside a container on a remote Docker host",
		Long: `devcontainer-cli agent exec via — run a one-off command in a container on a
remote Docker host reached through an existing SSH connection to <host>.`,
		Example: `  # Run a command on a remote Docker host
  devcontainer-cli agent exec via me@docker-host mycontainer -- uname -a`,
		Args:              agentExecViaArgs,
		ValidArgsFunction: completeViaArgs,
		SilenceUsage:      true,
		PreRunE:           agentExecDefaults,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellWithTarget(cmd, args[0], args[1], args[2:])
		},
	}
	addAgentExecSessionFlags(cmd)
	return cmd
}

func agentExecDefaults(cmd *cobra.Command, _ []string) error {
	return applyAgentFlagDefaults(cmd, map[string]string{
		flagNoInteractive: "true",
	})
}

func addAgentExecSessionFlags(cmd *cobra.Command) {
	cmd.Flags().String("user", "", "User to run the command as; always honoured, but only an interactive shell defaults it to devuser")
	cmd.Flags().BoolP("no-tty", "T", false, "Disable pseudo-TTY allocation (use when piping output to a file, e.g. a DB dump); implied when stdin is not a terminal")
	cmd.Flags().BoolP("workdir", "w", false, "Run in the project's workspace mount instead of the container default; only the devcontainer has that directory, so a database container needs it left off")
	addInteractiveFlag(cmd)

	_ = cmd.RegisterFlagCompletionFunc("user", func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if cmd.Name() == "via" && len(args) >= 2 {
			var users []string
			_ = service.WithHostOverride(args[0], func() error {
				users = service.InspectService{Report: console}.ListUsers(args[1])
				return nil
			})
			return users, cobra.ShellCompDirectiveNoFileComp
		}
		containerName, err := resolveContainer(cmd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return service.InspectService{Report: console}.ListUsers(containerName), cobra.ShellCompDirectiveNoFileComp
	})
}

func addAgentExecFlags(cmd *cobra.Command) {
	addAgentExecSessionFlags(cmd)
	addContainerFlag(cmd)
}

func agentExecViaArgs(_ *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("agent exec via requires <host> <container> -- command args...")
	}
	if len(args) < 3 {
		return fmt.Errorf("agent exec via requires a command, e.g. 'agent exec via me@host mycontainer -- go test ./...'")
	}
	return nil
}

// agentExecArgs requires the command 'shell' treats as optional.
func agentExecArgs(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent exec requires a command, e.g. 'agent exec -w -- go test ./...'")
	}
	return nil
}

func newAgentForwardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forward <port-mapping>",
		Short: "Forward a port between this machine and the running container over SSH",
		Long: `devcontainer-cli agent forward — forward a TCP port over SSH between this
machine and a running container, with prompting turned off and ephemeral
mode on by default.

Ephemeral mode skips ~/.ssh/config and ~/.ssh/devcontainer-cli.config
entirely: it authorizes the shared public key in devuser's authorized_keys,
reads the container's current IP on the compose network, and runs ssh -N
directly against it. Pass --ephemeral=false to opt into a managed SSH alias
instead.

Forward mappings (default) listen on this machine and reach the container;
reverse mappings ('reverse:11434' or '--reverse 11434') let a process inside
the container reach a port on this machine. The tunnel runs in the
foreground until SIGINT/SIGTERM, so background it when driving it from an
agent loop.`,
		Example: `  # Forward container port 3000 to 127.0.0.1:3000
  devcontainer-cli agent forward 3000

  # Forward Postgres on the project network to 127.0.0.1:5432
  devcontainer-cli agent forward 5432 --service postgres

  # Forward a port from a container on a remote Docker host
  devcontainer-cli agent forward via me@docker-host mycontainer 3000

  # Let the container reach Ollama running on this machine
  devcontainer-cli agent forward reverse:11434`,
		Args:         agentForwardArgs,
		SilenceUsage: true,
		PreRunE:      agentForwardDefaults,
		RunE:         runPortForward,
	}
	addForwardFlags(cmd)
	cmd.AddCommand(newAgentForwardViaCommand())
	return cmd
}

func newAgentForwardViaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "via <host> <container> <port-mapping>",
		Short: "Forward a port between this machine and a container on a remote Docker host",
		Long: `devcontainer-cli agent forward via — forward a TCP port over SSH between this
machine and a container running on a remote Docker host reached through an
existing SSH connection to <host>, with prompting off and ephemeral mode on by default.`,
		Example: `  # Forward remote container port 3000 to 127.0.0.1:3000
  devcontainer-cli agent forward via me@docker-host mycontainer 3000

  # Forward Postgres on the remote compose network
  devcontainer-cli agent forward via me@docker-host mycontainer 5432 --service postgres`,
		Args:              agentForwardViaArgs,
		ValidArgsFunction: completeViaArgs,
		SilenceUsage:      true,
		PreRunE:           agentForwardDefaults,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPortForwardWithTarget(cmd, args[0], args[1], args[2])
		},
	}
	addForwardSessionFlags(cmd)
	return cmd
}

func agentForwardDefaults(cmd *cobra.Command, _ []string) error {
	return applyAgentFlagDefaults(cmd, map[string]string{
		flagNoInteractive: "true",
		"ephemeral":       "true",
	})
}

func agentForwardArgs(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent forward requires a port mapping, e.g. 'agent forward 3000'")
	}
	if len(args) > 1 {
		return fmt.Errorf("accepts at most 1 arg(s), received %d", len(args))
	}
	return nil
}

func agentForwardViaArgs(_ *cobra.Command, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("agent forward via requires <host> <container> <port-mapping>, e.g. 'agent forward via me@host mycontainer 3000'")
	}
	if len(args) > 3 {
		return fmt.Errorf("accepts at most 3 arg(s), received %d", len(args))
	}
	if strings.TrimSpace(args[0]) == "" {
		return fmt.Errorf("remote host cannot be empty")
	}
	if strings.TrimSpace(args[1]) == "" {
		return fmt.Errorf("target container name cannot be empty")
	}
	if strings.TrimSpace(args[2]) == "" {
		return fmt.Errorf("port mapping cannot be empty")
	}
	return nil
}

// applyAgentFlagDefaults flips agent-safe defaults on flags the user did not
// explicitly set on the command line, so `agent ... --no-interactive=false`
// still works when a human runs it directly.
func applyAgentFlagDefaults(cmd *cobra.Command, defaults map[string]string) error {
	for name, val := range defaults {
		if cmd.Flags().Lookup(name) == nil || cmd.Flags().Changed(name) {
			continue
		}
		if err := cmd.Flags().Set(name, val); err != nil {
			return err
		}
	}
	return nil
}

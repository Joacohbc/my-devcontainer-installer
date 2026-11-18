package commands

import "github.com/spf13/cobra"

func init() {
	register(newWorkspaceCommand())
	register(newContainerCommand())
	register(newAccessCommand())
	register(newInspectCommand())
}

func newWorkspaceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage project workspace lifecycle and containers",
		Long: `devcontainer-cli workspace — manage the complete lifecycle of project devcontainers.

Provides subcommands to initialize (init, auto-init), up, down, destroy, compose
passthrough, and import or export configurations for the current project workspace.`,
		Example: `  # Create a workspace configuration with the interactive wizard
  devcontainer-cli workspace init

  # Detect the stack and initialize without prompts
  devcontainer-cli workspace auto-init

  # Bring up containers
  devcontainer-cli workspace up

  # Stop and remove containers
  devcontainer-cli workspace down

  # Run docker compose passthrough
  devcontainer-cli workspace compose ps`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newInitCommand(),
		newAutoInitCommand(),
		newUpCommand(),
		newDownCommand(),
		newDestroyCommand(),
		newComposeCommand(),
		newConfigExportCommand(),
		newConfigImportCommand(),
	)

	return cmd
}

func newContainerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "container",
		Short: "Run, start, stop, restart, and destroy containers",
		Long: `devcontainer-cli container — spin up throwaway containers and control container
lifecycle state (start, stop, restart, destroy).

Use 'container run' for a one-off container without project files, or use
'start', 'stop', 'restart', and 'destroy' against a specific container (by name
or --container). Without a container target, 'start', 'stop', and 'restart'
operate on the current project workspace's compose stack.`,
		Example: `  # Spin up a throwaway Python container
  devcontainer-cli container run --profile python

  # Start or stop a specific container
  devcontainer-cli container start dc-python
  devcontainer-cli container stop dc-python

  # Restart the current workspace's containers
  devcontainer-cli container restart

  # Destroy a standalone container
  devcontainer-cli container destroy dc-python --yes`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newRunCommand(),
		newStartCommand(),
		newStopCommand(),
		newRestartCommand(),
		newContainerDestroyCommand(),
	)

	return cmd
}

func newAccessCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "Connect to and interact with running devcontainers",
		Long: `devcontainer-cli access — connect to and interact with running devcontainers.

Provides interactive shells, SSH connectivity, port forwarding, routing, network
linking, file copy, and password management for active containers.`,
		Example: `  # Open a shell in the container
  devcontainer-cli access shell

  # Connect over SSH
  devcontainer-cli access ssh

  # Forward a port
  devcontainer-cli access forward 3000

  # Copy files into or out of container
  devcontainer-cli access copy ./file.txt devcontainer:/tmp/`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newShellCommand(),
		newSshCommand(),
		newPortForwardCommand(),
		newRouteCommand(),
		newNetworkCommand(),
		newCopyCommand(),
		newPasswordCommand(),
	)

	return cmd
}

func newInspectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect container status, logs, files, and context",
		Long: `devcontainer-cli inspect — observability and inspection commands for devcontainers.

Check container status, inspect container metadata and network configuration,
tail logs, examine container context, and list container files.`,
		Example: `  # Check project container status
  devcontainer-cli inspect status

  # View container metadata
  devcontainer-cli inspect info

  # Tail container logs
  devcontainer-cli inspect logs -f

  # List files in container working directory
  devcontainer-cli inspect ls`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newStatusCommand(),
		newInfoCommand(),
		newLogsCommand(),
		newContextCommand(),
		newLsCommand(),
	)

	return cmd
}

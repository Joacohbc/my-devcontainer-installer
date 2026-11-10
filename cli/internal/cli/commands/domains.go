package commands

import "github.com/spf13/cobra"

func init() {
	register(newWorkspaceCommand())
	register(newAccessCommand())
	register(newInspectCommand())
}

func newWorkspaceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage project workspace lifecycle and containers",
		Long: `devcontainer-cli workspace — manage the complete lifecycle of project devcontainers.

Provides subcommands to initialize, start, stop, restart, destroy, inspect compose
passthrough, and import or export configurations for the current project workspace.`,
		Example: `  # Initialize a workspace configuration
  devcontainer-cli workspace init

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
		newUpCommand(),
		newDownCommand(),
		newStartCommand(),
		newStopCommand(),
		newRestartCommand(),
		newDestroyCommand(),
		newRunCommand(),
		newComposeCommand(),
		newConfigExportCommand(),
		newConfigImportCommand(),
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

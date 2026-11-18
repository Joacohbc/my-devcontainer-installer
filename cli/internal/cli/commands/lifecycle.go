package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func newRestartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart [container]",
		Short: "Restart containers for the current project or a named container",
		Long: `devcontainer-cli container restart — restart existing containers without
recreating them.

Pass a container name (or --container) to restart that single container via
'docker restart'. With no container target, wraps
'docker compose -f .dc_<workspace>/build/docker-compose.yml restart' for the
current project.

The containers must already exist (run 'workspace up' or 'container run'
first). Configuration changes in the compose file are NOT applied — use
'workspace up' for that.`,
		Example: `  # Restart the current project's containers
  devcontainer-cli container restart

  # Restart a specific container by name or flag
  devcontainer-cli container restart dc-python
  devcontainer-cli container restart --container myproject-postgres`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeContainerArg(completeContainers),
		SilenceUsage:      true,
		RunE:              runRestart,
	}
	// restart applies to running or stopped containers, so complete any managed one.
	addContainerFlag(cmd)
	return cmd
}

func runRestart(cmd *cobra.Command, args []string) error {
	svc := service.LifecycleService{Report: ui.Console{}}
	return runContainerOrProjectLifecycle(cmd, args, "restart", svc.RestartContainer)
}

func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [container]",
		Short: "Start stopped containers for the current project or a named container",
		Long: `devcontainer-cli container start — start existing, stopped containers.

Pass a container name (or --container) to start that single container via
'docker start'. With no container target, wraps
'docker compose -f .dc_<workspace>/build/docker-compose.yml start' for the
current project.

Unlike 'workspace up' it never creates or recreates containers — it only resumes
ones that are stopped.`,
		Example: `  # Start the current project's stopped containers
  devcontainer-cli container start

  # Start a specific stopped container by name or flag
  devcontainer-cli container start dc-python
  devcontainer-cli container start --container myproject-postgres`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeContainerArg(completeStoppedContainers),
		SilenceUsage:      true,
		RunE:              runStart,
	}
	// start targets a stopped container, so complete only the stopped ones.
	addContainerFlagFiltered(cmd, completeStoppedContainers)
	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	svc := service.LifecycleService{Report: ui.Console{}}
	return runContainerOrProjectLifecycle(cmd, args, "start", svc.StartContainer)
}

func newStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [container]",
		Short: "Stop running containers (without removing them)",
		Long: `devcontainer-cli container stop — stop running containers, leaving containers,
volumes and networks in place so they can be resumed with 'container start'.

Pass a container name (or --container) to stop that single container via
'docker stop'. With no container target, wraps
'docker compose -f .dc_<workspace>/build/docker-compose.yml stop' for the
current project.

Use 'workspace down' instead when you want to remove the project's containers,
or 'container destroy' to remove an individual container.`,
		Example: `  # Stop the current project's running containers
  devcontainer-cli container stop

  # Stop a specific running container by name or flag
  devcontainer-cli container stop dc-python
  devcontainer-cli container stop --container myproject-postgres`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeContainerArg(completeRunningContainers),
		SilenceUsage:      true,
		RunE:              runStop,
	}
	// stop targets a running container, so complete only the running ones.
	addContainerFlagFiltered(cmd, completeRunningContainers)
	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	svc := service.LifecycleService{Report: ui.Console{}}
	return runContainerOrProjectLifecycle(cmd, args, "stop", svc.StopContainer)
}

func runContainerOrProjectLifecycle(cmd *cobra.Command, args []string, verb string, containerOp func(string) error) error {
	containerName, err := resolveLifecycleContainer(cmd, args)
	if err != nil {
		return err
	}
	if containerName != "" {
		if err := containerOp(containerName); err != nil {
			return err
		}
		ui.Done()
		return nil
	}
	return runLifecycle(verb)
}

func resolveLifecycleContainer(cmd *cobra.Command, args []string) (string, error) {
	var pos string
	if len(args) > 0 {
		pos = strings.TrimSpace(args[0])
		if pos == "" {
			return "", fmt.Errorf("container name cannot be empty")
		}
	}
	if !cmd.Flags().Changed("container") {
		return pos, nil
	}
	rawFlag, _ := cmd.Flags().GetString("container")
	flagName := strings.TrimSpace(rawFlag)
	if flagName == "" {
		return "", fmt.Errorf("container name cannot be empty")
	}
	if pos != "" && pos != flagName {
		return "", fmt.Errorf("cannot specify both a positional container %q and --container %q", pos, flagName)
	}
	return flagName, nil
}

func runLifecycle(verb string) error {
	cwd, err := currentDir()
	if err != nil {
		return err
	}
	composeFile, err := resolveProjectComposeFile(cwd)
	if err != nil {
		return err
	}
	console := ui.Console{}
	svc := service.LifecycleService{Report: console}
	if err := svc.Compose(composeFile, verb); err != nil {
		return err
	}
	console.Done()
	return nil
}

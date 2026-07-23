package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	register(newRestartCommand())
	register(newStartCommand())
	register(newStopCommand())
}

func newRestartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the project's already-created containers",
		Long: `devcontainer-cli restart — restart the existing containers of the current
project without recreating them.

Wraps 'docker compose -f .dc_<workspace>/build/docker-compose.yml restart'. The
containers must already exist (run 'up' first); this only stops and starts them
again, keeping the same containers, volumes and network. Configuration changes
in the compose file are NOT applied — use 'up' for that. Pass --container to
restart a single container instead of the whole project.`,
		Example: `  # Restart the whole project
  devcontainer-cli restart

  # Restart just one container
  devcontainer-cli restart --container myproject-postgres`,
		SilenceUsage: true,
		RunE:         runRestart,
	}
	// restart applies to running or stopped containers, so complete any managed one.
	addContainerFlag(cmd)
	return cmd
}

func runRestart(cmd *cobra.Command, _ []string) error {
	svc := service.LifecycleService{Report: ui.Console{}}

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd)
		if err != nil {
			return err
		}
		if err := svc.RestartContainer(containerName); err != nil {
			return err
		}
		ui.Done()
		return nil
	}

	return runLifecycle("restart")
}

func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the project's stopped containers",
		Long: `devcontainer-cli start — start the existing, stopped containers of the current
project (they must already have been created with 'up').

Wraps 'docker compose -f .dc_<workspace>/build/docker-compose.yml start'. Unlike
'up' it never creates or recreates containers — it only resumes ones that are
stopped. Pass --container to start a single container instead of the whole
project.`,
		Example: `  # Start the whole project
  devcontainer-cli start

  # Start just one container
  devcontainer-cli start --container myproject-postgres`,
		SilenceUsage: true,
		RunE:         runStart,
	}
	// start targets a stopped container, so complete only the stopped ones.
	addContainerFlagFiltered(cmd, completeStoppedContainers)
	return cmd
}

func runStart(cmd *cobra.Command, _ []string) error {
	svc := service.LifecycleService{Report: ui.Console{}}

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd)
		if err != nil {
			return err
		}
		if err := svc.StartContainer(containerName); err != nil {
			return err
		}
		ui.Done()
		return nil
	}

	return runLifecycle("start")
}

func newStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the project's running containers (without removing them)",
		Long: `devcontainer-cli stop — stop the running containers of the current project,
leaving the containers, volumes and network in place so they can be resumed with
'start'.

Wraps 'docker compose -f .dc_<workspace>/build/docker-compose.yml stop'. Use
'down' instead when you want to remove the containers, not just stop them. Pass
--container to stop a single container instead of the whole project.`,
		Example: `  # Stop the whole project
  devcontainer-cli stop

  # Stop just one container
  devcontainer-cli stop --container myproject-postgres`,
		SilenceUsage: true,
		RunE:         runStop,
	}
	// stop targets a running container, so complete only the running ones.
	addContainerFlagFiltered(cmd, completeRunningContainers)
	return cmd
}

func runStop(cmd *cobra.Command, _ []string) error {
	svc := service.LifecycleService{Report: ui.Console{}}

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd)
		if err != nil {
			return err
		}
		if err := svc.StopContainer(containerName); err != nil {
			return err
		}
		ui.Done()
		return nil
	}

	return runLifecycle("stop")
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

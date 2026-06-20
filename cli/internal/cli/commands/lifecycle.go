package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	register(newLifecycleCommand("restart"))
	register(newStartCommand())
	register(newStopCommand())
}

func newLifecycleCommand(verb string) *cobra.Command {
	return &cobra.Command{
		Use:   verb,
		Short: "Restart the project's already-created containers",
		Long: `devcontainer-cli restart — restart the existing containers of the current
project without recreating them.

Wraps 'docker compose -f .dc_<workspace>/build/docker-compose.yml restart'. The
containers must already exist (run 'up' first); this only stops and starts them
again, keeping the same containers, volumes and network. Configuration changes
in the compose file are NOT applied — use 'up' for that.`,
		Example:      "  devcontainer-cli restart",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLifecycle(verb)
		},
	}
}

func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the project's stopped containers",
		Long: `devcontainer-cli start — start the existing, stopped containers of the current
project (they must already have been created with 'up').

Wraps 'docker compose -f .dc_<workspace>/build/docker-compose.yml start'. Unlike
'up' it never creates or recreates containers — it only resumes ones that are
stopped.

Flags:
  --workspace   Target a workspace other than the current directory's.
  --container   Start a single container by name via 'docker start' instead of
                the whole project (tab-completes stopped containers).`,
		Example: `  # Start the whole project
  devcontainer-cli start

  # Start just one container
  devcontainer-cli start --container myproject-postgres`,
		SilenceUsage: true,
		RunE:         runStart,
	}
	addWorkspaceFlag(cmd)
	// start targets a stopped container, so complete only the stopped ones.
	addContainerFlagFiltered(cmd, completeStoppedContainers)
	return cmd
}

func runStart(cmd *cobra.Command, _ []string) error {
	wsFlag := workspaceFlag(cmd)
	svc := service.LifecycleService{Report: ui.Console{}}

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd, wsFlag)
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
'down' instead when you want to remove the containers, not just stop them.

Flags:
  --workspace   Target a workspace other than the current directory's.
  --container   Stop a single container by name via 'docker stop' instead of the
                whole project (tab-completes running containers).`,
		Example: `  # Stop the whole project
  devcontainer-cli stop

  # Stop just one container
  devcontainer-cli stop --container myproject-postgres`,
		SilenceUsage: true,
		RunE:         runStop,
	}
	addWorkspaceFlag(cmd)
	// stop targets a running container, so complete only the running ones.
	addContainerFlagFiltered(cmd, completeRunningContainers)
	return cmd
}

func runStop(cmd *cobra.Command, _ []string) error {
	wsFlag := workspaceFlag(cmd)
	svc := service.LifecycleService{Report: ui.Console{}}

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd, wsFlag)
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

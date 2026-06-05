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
		Short: "Run docker compose " + verb + " for the current project",
		Long: "devcontainer-cli " + verb + " — executes 'docker compose " + verb + "' for the current project.\n\n" +
			"Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml " + verb,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLifecycle(verb)
		},
	}
}

func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start existing project containers",
		Long: "devcontainer-cli start — starts existing project containers without recreating them.\n\n" +
			"Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml start\n\n" +
			"With --container: starts a single container via docker start.",
		SilenceUsage: true,
		RunE:         runStart,
	}
	addWorkspaceFlag(cmd)
	addContainerFlag(cmd)
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
		Short: "Stop running project containers",
		Long: "devcontainer-cli stop — stops running project containers without removing them.\n\n" +
			"Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml stop\n\n" +
			"With --container: stops a single container via docker stop.",
		SilenceUsage: true,
		RunE:         runStop,
	}
	addWorkspaceFlag(cmd)
	addContainerFlag(cmd)
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

package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func newUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create and start the project's containers",
		Long: `devcontainer-cli workspace up — create (or recreate) and start the containers for the
current project in the background.

Wraps 'docker compose -f .dc_<workspace>/build/docker-compose.yml up -d', so it
creates missing containers, applies any compose changes and leaves everything
running detached. Generate the project first (run 'devcontainer-cli agent create') so the
compose file exists.`,
		Example: `  # Start the current project's containers
  devcontainer-cli workspace up

  # Rebuild images first, then start
  devcontainer-cli workspace up --build`,
		SilenceUsage: true,
		RunE:         runUp,
	}
	addBuildFlag(cmd, false, "Build images before starting (docker compose up -d --build); use after changing the Dockerfile or modules")
	return cmd
}

func runUp(cmd *cobra.Command, _ []string) error {
	svc := service.LifecycleService{Report: ui.Console{}}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	composeFile, err := resolveProjectComposeFile(cwd)
	if err != nil {
		return err
	}

	workspace := resolveWorkspace(cwd)

	warnIfBuildStale(cwd, workspace)
	build, _ := cmd.Flags().GetBool("build")
	if err := svc.Up(composeFile, workspace, build); err != nil {
		return err
	}
	ui.Done()
	return nil
}

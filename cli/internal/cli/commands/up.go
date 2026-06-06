package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newUpCommand()) }

func newUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create and start project containers",
		Long: `devcontainer-cli up — builds, recreates, starts, and attaches to containers for a service.

Use this command to apply configuration changes, rebuild images after modifying Dockerfiles, or simply to ensure your entire project environment is running and up-to-date.

Scope: Active workspace

Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml up -d

Examples:
  devcontainer-cli up
  devcontainer-cli up --build
  devcontainer-cli up -w my-workspace`,
		SilenceUsage: true,
		RunE:         runUp,
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().Bool("build", false, "Build images before starting containers (docker compose up -d --build)")
	return cmd
}

func runUp(cmd *cobra.Command, _ []string) error {
	wsFlag := workspaceFlag(cmd)
	svc := service.LifecycleService{Report: ui.Console{}}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	composeFile, err := resolveProjectComposeFileWithWorkspace(cwd, wsFlag)
	if err != nil {
		return err
	}

	workspace := resolveWorkspace(cwd, wsFlag)

	build, _ := cmd.Flags().GetBool("build")
	if err := svc.Up(composeFile, workspace, build); err != nil {
		return err
	}
	ui.Done()
	return nil
}

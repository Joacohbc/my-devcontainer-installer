package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newUpCommand()) }

func newUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create and start project containers",
		Long: `devcontainer-cli up — build, recreate, start, and attach to containers for a service

Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml up -d`,
		SilenceUsage: true,
		RunE:         runUp,
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().Bool("build", false, "Build images before starting containers (docker compose up -d --build)")
	cmd.Flags().Bool("no-forward", false, "Skip starting the configured background port-forwards")
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

	noForward, _ := cmd.Flags().GetBool("no-forward")
	if !noForward {
		startConfiguredForwards(cwd, workspace)
	}

	ui.Done()
	return nil
}

// startConfiguredForwards spawns the project's declared background port-forwards
// after the stack is up. Failures are reported but do not fail `up`.
func startConfiguredForwards(cwd, workspace string) {
	cfg, err := domain.LoadConfig(cwd)
	if err != nil || cfg == nil || len(cfg.PortForwards) == 0 {
		return
	}
	console := ui.Console{}
	if _, err := (service.PortForwardService{Report: console}).StartConfigured(cwd, workspace, cfg.PortForwards); err != nil {
		console.Warn("Port-forwarding setup failed: %v", err)
	}
}

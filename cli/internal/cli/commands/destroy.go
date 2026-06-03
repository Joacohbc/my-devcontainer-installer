package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newDestroyCommand()) }

func newDestroyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "down -v + delete .dc_<workspace>/ and config (irreversible)",
		Long: `devcontainer-cli destroy — tear down everything for the current project

Runs 'docker compose down -v' (containers + volumes), then deletes the generated
.dc_<workspace>/ directory and devcontainer.config.json. This is irreversible.`,
		SilenceUsage: true,
		RunE:         runDestroy,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runDestroy(cmd *cobra.Command, _ []string) error {
	cwd, err := currentDir()
	if err != nil {
		return err
	}
	cfg, _ := domain.LoadConfig(cwd)
	workspace := domain.ResolveWorkspace(cwd, cfg)
	paths := project.ProjectPaths(cwd, workspace)
	cfgPath := domain.ConfigPath(cwd)

	console := ui.Console{}
	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("destroy is irreversible; pass --yes to confirm in non-interactive mode")
		}
		proceed, err := console.ConfirmDefault(
			fmt.Sprintf("Destroy '%s'? Removes containers, volumes, .dc_%s/ and devcontainer.config.json.", workspace, workspace),
			false,
		)
		if err != nil {
			return err
		}
		if !proceed {
			console.Cancelled()
			return nil
		}
	}

	if _, ferr := (service.PortForwardService{Report: console}).StopAllForwards(cwd, workspace); ferr != nil {
		console.Warn("Failed to stop port-forwards: %v", ferr)
	}

	svc := service.DestroyService{Report: console}
	return svc.Run(service.DestroyTarget{
		Workspace:   workspace,
		ComposeFile: paths.ComposeFile,
		ProjectDir:  paths.ProjectDir,
		ConfigPath:  cfgPath,
		ProjectKey:  cwd,
	})
}

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
		Short: "Tear down the project completely and delete its generated files",
		Long: `devcontainer-cli destroy — completely remove the current project. This is the
nuclear option and cannot be undone.

It runs 'docker compose down -v' (removing containers, the network AND the named
volumes, so all data is deleted), then deletes the generated .dc_<workspace>/
directory and the devcontainer.config.json. After this the project is gone and
would have to be regenerated from scratch. Use 'down' instead if you only want
to stop containers but keep your files and data.

Because it is irreversible, with --no-interactive you must also pass --yes.`,
		Example: `  # Interactive: asks for confirmation first
  devcontainer-cli destroy

  # Unattended teardown in a script
  devcontainer-cli destroy --yes`,
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

	svc := service.DestroyService{Report: console}
	return svc.Run(service.DestroyTarget{
		Workspace:   workspace,
		ComposeFile: paths.ComposeFile,
		ProjectDir:  paths.ProjectDir,
		ConfigPath:  cfgPath,
		ProjectKey:  cwd,
	})
}

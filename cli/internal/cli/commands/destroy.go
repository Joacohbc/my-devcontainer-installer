package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
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

It also prunes the SSH state 'ssh' wrote for the target: its managed
managed Host block and the container host keys pinned for the address that
block dialed, unless another Host block still dials it. Your own
~/.ssh/known_hosts is never touched.

Pass --container to instead tear down a single loose container that isn't part
of a workspace project (e.g. one set up via 'ssh --container'): it stops and
removes just that container and prunes the same
SSH state — there is no compose stack, project directory, or config file to
remove in this mode.

Because it is irreversible, with --no-interactive you must also pass --yes.`,
		Example: `  # Interactive: asks for confirmation first
  devcontainer-cli destroy

  # Unattended teardown in a script
  devcontainer-cli destroy --yes

  # Tear down a single loose container and its managed SSH config
  devcontainer-cli destroy --container dc-ssh`,
		SilenceUsage: true,
		RunE:         runDestroy,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	addContainerFlag(cmd)
	return cmd
}

func runDestroy(cmd *cobra.Command, _ []string) error {
	console := ui.Console{}

	if cmd.Flags().Changed("container") {
		container, _ := cmd.Flags().GetString("container")
		return runDestroyContainer(cmd, console, container)
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	target, _ := destroyTargetFor(cwd)

	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("destroy is irreversible; pass --yes to confirm in non-interactive mode")
		}
		proceed, err := console.ConfirmDefault(
			fmt.Sprintf("Destroy '%s'? Removes containers, volumes, .dc_%s/ and devcontainer.config.json.", target.Workspace, target.Workspace),
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
	return svc.Run(target)
}

// destroyTargetFor resolves everything a destroy removes for the project in
// cwd, alongside its persisted config (nil when the project has none yet — the
// workspace then falls back to the directory name). 'agent clean' reuses it so
// both commands can never disagree about what a project owns.
func destroyTargetFor(cwd string) (service.DestroyTarget, *types.DevcontainerConfig) {
	cfg, _ := domain.LoadConfig(cwd)
	workspace := domain.ResolveWorkspace(cwd, cfg)
	paths := project.ProjectPaths(cwd, workspace)
	return service.DestroyTarget{
		Workspace:   workspace,
		ComposeFile: paths.ComposeFile,
		ProjectDir:  paths.ProjectDir,
		ConfigPath:  domain.ConfigPath(cwd),
		ProjectKey:  cwd,
	}, cfg
}

// runDestroyContainer handles the --container path: no workspace, compose file,
// or config to remove — just stop+remove the named container and prune its
// managed SSH host block.
func runDestroyContainer(cmd *cobra.Command, console ui.Console, container string) error {
	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("destroy is irreversible; pass --yes to confirm in non-interactive mode")
		}
		proceed, err := console.ConfirmDefault(
			fmt.Sprintf("Destroy container '%s'? Stops and removes it, and prunes its managed SSH config.", container),
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
	return svc.RunContainer(container)
}

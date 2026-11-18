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

func newDestroyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Tear down the project completely and delete its generated files",
		Long: `devcontainer-cli workspace destroy — completely remove the current project. This is the
nuclear option and cannot be undone.

It runs 'docker compose down -v' (removing containers, the network AND the named
volumes, so all data is deleted), then deletes the generated .dc_<workspace>/
directory and the devcontainer.config.json. After this the project is gone and
would have to be regenerated from scratch. Use 'workspace down' instead if you only want
to stop containers but keep your files and data.

The project image is removed too unless another project shares it: images are
named by content, so identical projects use one image until the last one goes.

It also prunes the SSH state 'access ssh' wrote for the target: its managed
Host block and the container host keys pinned for the address that block dialed,
unless another Host block still dials it. Your own ~/.ssh/known_hosts is never
touched.

To tear down a single loose container outside a project (e.g. one started with
'container run'), use 'devcontainer-cli container destroy <container>' instead.

Because it is irreversible, with --no-interactive you must also pass --yes.`,
		Example: `  # Interactive: asks for confirmation first
  devcontainer-cli workspace destroy

  # Unattended teardown in a script
  devcontainer-cli workspace destroy --yes`,
		SilenceUsage: true,
		RunE:         runDestroy,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func newContainerDestroyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy [container]",
		Short: "Stop and remove a standalone or individual container",
		Long: `devcontainer-cli container destroy — stop and remove a single container (such as
one started with 'container run') and prune its managed SSH host block and
pinned host keys.

To tear down an entire project workspace (compose stack, volumes, .dc_<ws>/,
and devcontainer.config.json), use 'devcontainer-cli workspace destroy' instead.

Because it is irreversible, with --no-interactive you must also pass --yes.`,
		Example: `  # Interactive: asks for confirmation first
  devcontainer-cli container destroy dc-python

  # Target with --container and skip confirmation
  devcontainer-cli container destroy --container dc-base --yes`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeContainerArg(completeContainers),
		SilenceUsage:      true,
		RunE:              runContainerDestroy,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	addContainerFlag(cmd)
	return cmd
}

func runContainerDestroy(cmd *cobra.Command, args []string) error {
	container, err := resolveLifecycleContainer(cmd, args)
	if err != nil {
		return err
	}
	if container == "" {
		return fmt.Errorf("container name is required: pass <container> or --container <name> (or use 'devcontainer-cli workspace destroy' to tear down a project)")
	}
	return runDestroyContainer(cmd, ui.Console{}, container)
}

func runDestroy(cmd *cobra.Command, _ []string) error {
	console := ui.Console{}

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

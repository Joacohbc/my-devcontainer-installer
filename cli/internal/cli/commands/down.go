package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newDownCommand()) }

func newDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop and remove the project's containers",
		Long: `devcontainer-cli down — stop and remove the containers (and network) for the
current project, leaving the generated files and named volumes intact.

Wraps 'docker compose -f .dc_<workspace>/build/docker-compose.yml down'. Named
volumes hold your data (databases, the shared config) and are kept by default;
pass -v to delete them too. To also remove the generated .dc_<workspace>/
directory and config, use 'destroy' instead.

Flags:
  -v, --volumes   Also remove the project's named volumes (down -v). This
                  deletes their data and is irreversible.
  -y, --yes       Skip the "also remove volumes?" prompt. It only suppresses the
                  prompt — it never opts into deleting volumes on its own; you
                  still need -v for that.
      --no-interactive  Never prompt; without -v, volumes are kept.`,
		Example: `  # Stop and remove containers, keep data volumes
  devcontainer-cli down

  # Also delete the named volumes (destroys data)
  devcontainer-cli down -v

  # Non-interactive teardown in a script (keeps volumes)
  devcontainer-cli down --yes`,
		SilenceUsage: true,
		RunE:         runDown,
	}
	cmd.Flags().BoolP("volumes", "v", false, "Also remove named volumes (docker compose down -v)")
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

// resolveRemoveVolumes decides whether `down` should pass -v. Volumes are only
// removed when -v/--volumes is explicit; --yes merely skips the prompt (it does
// not opt into data deletion). The confirm callback is consulted only in
// interactive mode without an explicit answer.
func resolveRemoveVolumes(volumesFlag, yes, interactive bool, confirm func() (bool, error)) (bool, error) {
	if volumesFlag {
		return true, nil
	}
	if yes || !interactive {
		return false, nil
	}
	return confirm()
}

func runDown(cmd *cobra.Command, _ []string) error {
	console := ui.Console{}
	svc := service.LifecycleService{Report: console}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	cfg, _ := domain.LoadConfig(cwd)
	workspace := domain.ResolveWorkspace(cwd, cfg)
	composeFile, err := resolveProjectComposeFile(cwd)
	if err != nil {
		return err
	}

	volumesFlag, _ := cmd.Flags().GetBool("volumes")
	removeVolumes, err := resolveRemoveVolumes(volumesFlag, yesFlag(cmd), interactiveFlag(cmd), func() (bool, error) {
		return console.ConfirmDefault("Also remove named volumes for '"+workspace+"'? This deletes their data.", false)
	})
	if err != nil {
		return err
	}

	if err := svc.Down(composeFile, workspace, removeVolumes); err != nil {
		return err
	}
	console.Done()
	return nil
}

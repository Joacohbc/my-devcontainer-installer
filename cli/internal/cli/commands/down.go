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
		Short: "Stop and remove project containers",
		Long: `devcontainer-cli down — stops and removes the running containers for the current project.

This is the standard way to stop working on a project. It shuts down the environment cleanly and frees up system resources (CPU/RAM) without deleting your configuration files or persistent data (unless you explicitly use flags).

Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml down
Add -v/--volumes to also remove named volumes (deletes data). --yes only
skips the volume prompt; it never deletes volumes on its own.`,
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

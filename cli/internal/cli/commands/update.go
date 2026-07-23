package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newUpdateCommand()) }

func newUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update (pull or rebuild) a project's container images",
		Long: `devcontainer-cli update — refresh the container images for a devcontainer.

What it does depends on the project's build mode: 'remote' projects pull the
newest image from the registry, while 'local-cached' projects rebuild the image
from the Dockerfile when its contents changed. By default it updates the project
in the current directory and records the new image.

Note: this updates container IMAGES, not the CLI. To update the CLI binary
itself, run 'devcontainer-cli upgrade-cli'.`,
		Example: `  # Update the current project's images
  devcontainer-cli update

  # Update every recorded project
  devcontainer-cli update --all

  # Force a fresh pull / rebuild
  devcontainer-cli update --pull
  devcontainer-cli update --rebuild`,
		SilenceUsage: true,
		RunE:         runUpdateImages,
	}
	cmd.Flags().Bool("all", false, "Update images for every project tracked in images.json")
	cmd.Flags().Bool("pull", false, "Always pull (no-op for local-cached without rebuild)")
	cmd.Flags().Bool("rebuild", false, "Always rebuild (no-op for remote)")
	addContainerFlag(cmd)
	return cmd
}

func updateService() service.UpdateService {
	return service.UpdateService{Report: console}
}

func runUpdateImages(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")
	pull, _ := cmd.Flags().GetBool("pull")
	rebuild, _ := cmd.Flags().GetBool("rebuild")

	if cmd.Flags().Changed("container") {
		return updateContainerImage(cmd)
	}

	svc := updateService()
	if all {
		updated, skipped, failed := svc.UpdateAll(pull, rebuild)
		console.Info("--- %d updated, %d skipped, %d failed", updated, skipped, failed)
		if failed > 0 {
			return fmt.Errorf("%d project(s) failed to update", failed)
		}
		return nil
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	config, _ := domain.LoadConfig(cwd)
	if config == nil {
		return fmt.Errorf("no devcontainer.config.json found in %s. Run 'devcontainer-cli' to generate one first, or pass --all to update every recorded project", cwd)
	}
	image, ok := svc.UpdateOne(cwd, config, pull, rebuild)
	if !ok {
		return fmt.Errorf("update failed")
	}
	domain.RecordProject(cwd, config, image)
	return nil
}

func updateContainerImage(cmd *cobra.Command) error {
	containerName, err := resolveContainer(cmd)
	if err != nil {
		return err
	}
	_, err = updateService().UpdateContainer(containerName)
	return err
}

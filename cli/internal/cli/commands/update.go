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
		Short: "Update container images for this project or all projects",
		Long: `devcontainer-cli update — pulls the latest container images for the project.

Essential for keeping your base tools, databases, and dependencies secure and up-to-date with upstream changes without needing to manually run docker pull commands.

Scope: Active project (default) or All projects

  devcontainer-cli update              Update images for the project in the current dir
  devcontainer-cli update --all        Update images for every recorded project
  devcontainer-cli update --pull       Force pull for remote images
  devcontainer-cli update --rebuild    Force rebuild for local-cached images

Note: To update the CLI binary itself, run 'devcontainer-cli upgrade-cli'.`,
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
	containerName, err := resolveContainer(cmd, "")
	if err != nil {
		return err
	}
	_, err = updateService().UpdateContainer(containerName)
	return err
}

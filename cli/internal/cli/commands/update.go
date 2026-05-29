package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/spf13/cobra"
)

func init() { register(newUpdateCommand()) }

func newUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update container images for this project / all projects",
		Long: `devcontainer-cli update — update container images

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

type updateResult struct {
	ok    bool
	image string
}

func updateOne(projectDir string, config *types.DevcontainerConfig, pull, rebuild bool) updateResult {
	ui.Log(fmt.Sprintf("Updating '%s' (mode=%s) at %s", config.Workspace, config.Mode, projectDir))

	if config.Mode == types.BuildModeRemote {
		if config.Remote == nil {
			ui.Warn("Remote config missing — skipping.")
			return updateResult{ok: false, image: config.Image}
		}
		reg := domain.ResolveRegistry("", config.Remote.Registry)
		image := domain.ResolveRemoteImage(config.Remote.Variant, reg)
		status, err := docker.DockerInherit([]string{"pull", image})
		if err != nil || status != 0 {
			return updateResult{ok: false, image: image}
		}
		ui.Ok("Pulled " + image)
		return updateResult{ok: true, image: image}
	}

	paths := project.ProjectPaths(projectDir, config.Workspace)
	if _, err := os.Stat(paths.ComposeFile); err != nil {
		ui.Warn(fmt.Sprintf("No compose file at %s — skipping.", paths.ComposeFile))
		return updateResult{ok: false, image: config.Image}
	}
	composeArgs := []string{"build"}
	if !rebuild || pull {
		composeArgs = append(composeArgs, "--pull")
	}
	status, err := docker.DockerCompose(paths.ComposeFile, composeArgs, &docker.ComposeOptions{CWD: projectDir})
	if err != nil || status != 0 {
		return updateResult{ok: false, image: config.Image}
	}
	ui.Ok("Rebuilt " + config.Image)
	return updateResult{ok: true, image: config.Image}
}

func updateAll(pull, rebuild bool) error {
	entries := domain.ListEntries()
	if len(entries) == 0 {
		ui.Warn("No projects recorded yet. Generate at least one project first.")
		return nil
	}
	okCount, skipCount, failCount := 0, 0, 0
	for _, e := range entries {
		if _, err := os.Stat(e.ProjectDir); err != nil {
			ui.Warn(fmt.Sprintf("Project directory missing: %s — removing from catalog.", e.ProjectDir))
			domain.RemoveEntry(e.ProjectDir)
			skipCount++
			continue
		}
		cfg, _ := domain.LoadConfig(e.ProjectDir)
		if cfg == nil {
			ui.Warn(fmt.Sprintf("No devcontainer.config.json at %s — skipping.", e.ProjectDir))
			skipCount++
			continue
		}
		r := updateOne(e.ProjectDir, cfg, pull, rebuild)
		if r.ok {
			domain.RecordProject(e.ProjectDir, cfg, r.image)
			okCount++
		} else {
			failCount++
		}
	}
	fmt.Printf(ui.Subtle("\n--- %d updated, %d skipped, %d failed\n"), okCount, skipCount, failCount)
	if failCount > 0 {
		return fmt.Errorf("%d project(s) failed to update", failCount)
	}
	return nil
}

func runUpdateImages(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")
	pull, _ := cmd.Flags().GetBool("pull")
	rebuild, _ := cmd.Flags().GetBool("rebuild")

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd, "")
		if err != nil {
			return err
		}
		status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", "{{.Config.Image}}", containerName})
		if err != nil || status != 0 {
			return fmt.Errorf("failed to inspect container '%s'", containerName)
		}
		image := strings.TrimSpace(stdout)
		if image == "" {
			return fmt.Errorf("could not resolve image for container '%s'", containerName)
		}
		ui.Log(fmt.Sprintf("Pulling updated image '%s' for container '%s'...", image, containerName))
		status, err = docker.DockerInherit([]string{"pull", image})
		if err != nil || status != 0 {
			return fmt.Errorf("failed to pull image '%s'", image)
		}
		ui.Ok("Pulled " + image)
		return nil
	}

	if all {
		return updateAll(pull, rebuild)
	}
	cwd, err := currentDir()
	if err != nil {
		return err
	}
	config, _ := domain.LoadConfig(cwd)
	if config == nil {
		return fmt.Errorf("no devcontainer.config.json found in %s. Run 'devcontainer-cli' to generate one first, or pass --all to update every recorded project", cwd)
	}
	r := updateOne(cwd, config, pull, rebuild)
	if r.ok {
		domain.RecordProject(cwd, config, r.image)
		return nil
	}
	return fmt.Errorf("update failed")
}

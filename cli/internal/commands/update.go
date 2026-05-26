package commands

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/core"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/ui"
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
	return cmd
}

type updateResult struct {
	ok    bool
	image string
}

func updateOne(projectDir string, config *core.DevcontainerConfig, pull, rebuild bool) updateResult {
	ui.Log(fmt.Sprintf("Updating '%s' (mode=%s) at %s", config.Workspace, config.Mode, projectDir))

	if config.Mode == core.BuildModeRemote {
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

func updateAll(pull, rebuild bool) {
	entries := domain.ListEntries()
	if len(entries) == 0 {
		ui.Warn("No projects recorded yet. Generate at least one project first.")
		return
	}
	okCount, skipCount, failCount := 0, 0, 0
	for _, e := range entries {
		if _, err := os.Stat(e.ProjectDir); err != nil {
			ui.Warn(fmt.Sprintf("Project directory missing: %s — removing from registry.", e.ProjectDir))
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
	color.New(color.FgWhite).Printf("\n--- %d updated, %d skipped, %d failed\n", okCount, skipCount, failCount)
}

func runUpdateImages(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")
	pull, _ := cmd.Flags().GetBool("pull")
	rebuild, _ := cmd.Flags().GetBool("rebuild")

	if all {
		updateAll(pull, rebuild)
		return nil
	}
	cwd, _ := os.Getwd()
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

package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/spf13/cobra"
)

func init() { register(newCleanupTipsCommand()) }

func newCleanupTipsCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "cleanup-tips",
		Short:        "Show docker cleanup commands for this project",
		Long:         "devcontainer-cli cleanup-tips — show docker cleanup commands for this project",
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			cwd, err := currentDir()
			if err != nil {
				return err
			}
			cfg, _ := domain.LoadConfig(cwd)
			if cfg == nil {
				cfg = domain.DefaultConfig(cwd)
			}
			if cfg.Workspace == "" {
				cfg.Workspace = domain.ResolveWorkspace(cwd, cfg)
			}
			printCleanupInstructions(cfg)
			return nil
		},
	}
}

func printCleanupInstructions(config *types.DevcontainerConfig) {
	projectID := types.ProjectID(config)
	bar := ui.Subtle(strings.Repeat("─", 64))
	prompt := ui.Subtle("   $ ")
	allFilter := fmt.Sprintf(`--filter "label=%s=true"`, types.LabelManaged)
	projFilter := fmt.Sprintf(`--filter "label=%s=%s"`, types.LabelProject, projectID)

	lines := []string{
		ui.StyleHeader.Render("Cleanup / update — managed by label"),
		bar,
		ui.Subtle("Every image, container, volume and network created by this CLI is tagged with:"),
		ui.Subtle(fmt.Sprintf("  %s=true", types.LabelManaged)),
		ui.Subtle(fmt.Sprintf("  %s=%s", types.LabelProject, projectID)),
		"",
		ui.Bold("List resources of THIS project:"),
		prompt + "docker ps -a " + projFilter,
		prompt + "docker images " + projFilter,
		prompt + "docker volume ls " + projFilter,
		"",
		ui.Bold("List resources of ALL projects managed by this CLI:"),
		prompt + "docker ps -a " + allFilter,
		prompt + "docker images " + allFilter,
		"",
		ui.Bold("Stop + remove THIS project (containers, network, volumes):"),
		prompt + "docker compose down -v",
		"",
		ui.Bold("Purge dangling/unused images of THIS project:"),
		prompt + "docker image prune -a " + projFilter + " -f",
		"",
		ui.Bold("Purge ALL CLI-managed images (every project):"),
		prompt + "docker image prune -a " + allFilter + " -f",
		"",
		ui.Bold("Update (rebuild without cache + recreate):"),
		prompt + "docker compose build --no-cache",
		prompt + "docker compose up -d --force-recreate",
		bar,
		"",
	}
	fmt.Println(strings.Join(lines, "\n"))
}

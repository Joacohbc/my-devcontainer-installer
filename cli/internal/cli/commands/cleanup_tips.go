package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/spf13/cobra"
)

func init() { register(newCleanupTipsCommand()) }

func newCleanupTipsCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "cleanup-tips",
		Short:        "Show docker cleanup commands for this project",
		Long:         "devcontainer-cli cleanup-tips — shows suggested docker cleanup commands for the current project resources.",
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
	bar := console.Subtle(strings.Repeat("─", 64))
	prompt := console.Subtle("   $ ")
	allFilter := fmt.Sprintf(`--filter "label=%s=true"`, types.LabelManaged)
	projFilter := fmt.Sprintf(`--filter "label=%s=%s"`, types.LabelProject, projectID)

	lines := []string{
		console.HeaderS("Cleanup / update — managed by label"),
		bar,
		console.Subtle("Every image, container, volume and network created by this CLI is tagged with:"),
		console.Subtle(fmt.Sprintf("  %s=true", types.LabelManaged)),
		console.Subtle(fmt.Sprintf("  %s=%s", types.LabelProject, projectID)),
		"",
		console.Bold("List resources of THIS project:"),
		prompt + "docker ps -a " + projFilter,
		prompt + "docker images " + projFilter,
		prompt + "docker volume ls " + projFilter,
		"",
		console.Bold("List resources of ALL projects managed by this CLI:"),
		prompt + "docker ps -a " + allFilter,
		prompt + "docker images " + allFilter,
		"",
		console.Bold("Stop + remove THIS project (containers, network, volumes):"),
		prompt + "docker compose down -v",
		"",
		console.Bold("Purge dangling/unused images of THIS project:"),
		prompt + "docker image prune -a " + projFilter + " -f",
		"",
		console.Bold("Purge ALL CLI-managed images (every project):"),
		prompt + "docker image prune -a " + allFilter + " -f",
		"",
		console.Bold("Update (rebuild without cache + recreate):"),
		prompt + "docker compose build --no-cache",
		prompt + "docker compose up -d --force-recreate",
		bar,
		"",
	}
	console.Print(strings.Join(lines, "\n"))
}

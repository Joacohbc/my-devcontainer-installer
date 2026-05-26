package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/infra/project"
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
			cwd, _ := os.Getwd()
			cfg, _ := domain.LoadConfig(cwd)
			if cfg == nil {
				cfg = domain.DefaultConfig(cwd)
			}
			if cfg.Workspace == "" {
				cfg.Workspace = project.ResolveWorkspace(cwd, cfg)
			}
			printCleanupInstructions(cfg)
			return nil
		},
	}
}

func printCleanupInstructions(config *core.DevcontainerConfig) {
	projectID := core.ProjectID(config)
	bold := color.New(color.Bold)
	gray := color.New(color.FgWhite)
	cyanBold := color.New(color.FgCyan, color.Bold)
	bar := gray.Sprint(strings.Repeat("─", 64))
	prompt := gray.Sprint("   $ ")
	allFilter := fmt.Sprintf(`--filter "label=%s=true"`, core.LabelManaged)
	projFilter := fmt.Sprintf(`--filter "label=%s=%s"`, core.LabelProject, projectID)

	lines := []string{
		cyanBold.Sprint("Cleanup / update — managed by label"),
		bar,
		gray.Sprint("Every image, container, volume and network created by this CLI is tagged with:"),
		gray.Sprintf("  %s=true", core.LabelManaged),
		gray.Sprintf("  %s=%s", core.LabelProject, projectID),
		"",
		bold.Sprint("List resources of THIS project:"),
		prompt + "docker ps -a " + projFilter,
		prompt + "docker images " + projFilter,
		prompt + "docker volume ls " + projFilter,
		"",
		bold.Sprint("List resources of ALL projects managed by this CLI:"),
		prompt + "docker ps -a " + allFilter,
		prompt + "docker images " + allFilter,
		"",
		bold.Sprint("Stop + remove THIS project (containers, network, volumes):"),
		prompt + "docker compose down -v",
		"",
		bold.Sprint("Purge dangling/unused images of THIS project:"),
		prompt + "docker image prune -a " + projFilter + " -f",
		"",
		bold.Sprint("Purge ALL CLI-managed images (every project):"),
		prompt + "docker image prune -a " + allFilter + " -f",
		"",
		bold.Sprint("Update (rebuild without cache + recreate):"),
		prompt + "docker compose build --no-cache",
		prompt + "docker compose up -d --force-recreate",
		bar,
		"",
	}
	fmt.Println(strings.Join(lines, "\n"))
}

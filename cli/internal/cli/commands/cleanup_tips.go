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
	console.Print(strings.Join(cleanupInstructionLines(config), "\n"))
}

// cleanupInstructionLines builds the cleanup/update cheat-sheet. Each task lists
// the native devcontainer-cli command first and, where useful, the raw docker
// command it runs under the hood (prefixed "└ docker:") for users who prefer to
// drive docker directly.
func cleanupInstructionLines(config *types.DevcontainerConfig) []string {
	projectID := types.ProjectID(config)
	bar := console.Subtle(strings.Repeat("─", 64))
	cli := func(c string) string { return "   $ " + c }
	docker := func(c string) string { return console.Subtle("     └ docker: " + c) }
	allFilter := fmt.Sprintf(`--filter "label=%s=true"`, types.LabelManaged)
	projFilter := fmt.Sprintf(`--filter "label=%s=%s"`, types.LabelProject, projectID)

	return []string{
		console.HeaderS("Cleanup / update — managed by label"),
		bar,
		console.Subtle("Every image, container, volume and network created by this CLI is tagged with:"),
		console.Subtle(fmt.Sprintf("  %s=true", types.LabelManaged)),
		console.Subtle(fmt.Sprintf("  %s=%s", types.LabelProject, projectID)),
		console.Subtle("Each task below shows the native command and the docker it wraps."),
		"",
		console.Bold("List resources of THIS project:"),
		cli("devcontainer-cli ls"),
		docker("docker ps -a " + projFilter),
		docker("docker images " + projFilter),
		docker("docker volume ls " + projFilter),
		"",
		console.Bold("List resources of ALL projects managed by this CLI:"),
		docker("docker ps -a " + allFilter),
		docker("docker images " + allFilter),
		"",
		console.Bold("Stop + remove THIS project (containers, network, volumes):"),
		cli("devcontainer-cli down -v"),
		cli("devcontainer-cli destroy        # also deletes .dc_<ws>/ + config"),
		docker("docker compose down -v"),
		"",
		console.Bold("Remove managed containers / images:"),
		cli("devcontainer-cli remove-container [name...] [--all]"),
		cli("devcontainer-cli remove-image [ref...] [--all]"),
		"",
		console.Bold("Purge unused CLI-managed images / networks / volumes:"),
		cli("devcontainer-cli prune [images|network|volume] [--all]"),
		docker("docker image prune -a " + projFilter + " -f"),
		docker("docker image prune -a " + allFilter + " -f"),
		"",
		console.Bold("Update (rebuild without cache + recreate):"),
		cli("devcontainer-cli update [--all]"),
		docker("docker compose build --no-cache"),
		docker("docker compose up -d --force-recreate"),
		bar,
		"",
	}
}

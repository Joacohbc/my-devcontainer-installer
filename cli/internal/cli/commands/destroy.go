package commands

import (
	"fmt"
	"os"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/prompt"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/spf13/cobra"
)

func init() { register(newDestroyCommand()) }

func newDestroyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "down -v + delete .dc_<workspace>/ and config (irreversible)",
		Long: `devcontainer-cli destroy — tear down everything for the current project

Runs 'docker compose down -v' (containers + volumes), then deletes the generated
.dc_<workspace>/ directory and devcontainer.config.json. This is irreversible.`,
		SilenceUsage: true,
		RunE:         runDestroy,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runDestroy(cmd *cobra.Command, _ []string) error {
	cwd, err := currentDir()
	if err != nil {
		return err
	}
	cfg, _ := domain.LoadConfig(cwd)
	workspace := domain.ResolveWorkspace(cwd, cfg)
	paths := project.ProjectPaths(cwd, workspace)
	cfgPath := domain.ConfigPath(cwd)

	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("destroy is irreversible; pass --yes to confirm in non-interactive mode")
		}
		proceed, err := prompt.Confirm(
			fmt.Sprintf("Destroy '%s'? Removes containers, volumes, .dc_%s/ and devcontainer.config.json.", workspace, workspace),
			false,
		)
		if err != nil {
			return err
		}
		if !proceed {
			ui.Cancelled()
			return nil
		}
	}

	if _, err := os.Stat(paths.ComposeFile); err == nil {
		ui.Yellow("\nBringing down '%s' (with volumes)...", workspace)
		if err := docker.DockerComposeOrThrow(paths.ComposeFile, []string{"down", "-v"}, nil); err != nil {
			return err
		}
	} else {
		fmt.Printf(ui.Subtle("No compose file at %s; skipping 'docker compose down'.\n"), paths.ComposeFile)
	}

	if _, err := os.Stat(paths.ProjectDir); err == nil {
		if err := os.RemoveAll(paths.ProjectDir); err == nil {
			fmt.Printf(ui.Subtle("Removed %s\n"), paths.ProjectDir)
		}
	}
	if _, err := os.Stat(cfgPath); err == nil {
		if err := os.Remove(cfgPath); err == nil {
			fmt.Printf(ui.Subtle("Removed %s\n"), cfgPath)
		}
	}
	domain.RemoveEntry(cwd)

	fmt.Print(ui.StyleSuccess.Bold(true).Render("\nDestroyed.") + "\n\n")
	return nil
}

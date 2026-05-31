package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func newConfigImportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "import <file.yml>",
		Short:        "Import a YAML config and write devcontainer.config.json",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE:         runConfigImport,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("force", false, "Overwrite existing devcontainer.config.json without prompt")
	return cmd
}

func runConfigImport(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	console := ui.Console{}
	svc := service.ConfigService{Report: console}
	cfg, err := svc.ParseConfigYAML(data)
	if err != nil {
		return err
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool("force")
	if !force && existingConfig(cwd) {
		if yesFlag(cmd) || !interactiveFlag(cmd) {
			if !yesFlag(cmd) {
				return fmt.Errorf("devcontainer.config.json exists. Pass --force or --yes")
			}
		} else {
			ok, err := console.ConfirmDefault("devcontainer.config.json exists. Overwrite?", false)
			if err != nil {
				return err
			}
			if !ok {
				console.Cancelled()
				return nil
			}
		}
	}
	return svc.SaveProjectConfig(cwd, cfg)
}

func existingConfig(cwd string) bool {
	_, err := os.Stat(filepath.Join(cwd, types.ConfigFile))
	return err == nil
}

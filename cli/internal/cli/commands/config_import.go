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
		Use:   "import <file.yml>",
		Short: "Import a YAML config into devcontainer.config.json",
		Long: `devcontainer-cli config import — read a YAML config file (such as one produced
by 'config export') and write it to the project's devcontainer.config.json.

This recreates a project's configuration from a shared/checked-in file. Run
'devcontainer-cli agent create' afterwards to (re)generate the Dockerfile and
compose from it.
If a config already exists you are asked before overwriting (use --force or --yes
to skip the prompt).`,
		Example: `  # Recreate the project config from a YAML file
  devcontainer-cli config import devcontainer.yml

  # Overwrite an existing config without prompting
  devcontainer-cli config import devcontainer.yml --force`,
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

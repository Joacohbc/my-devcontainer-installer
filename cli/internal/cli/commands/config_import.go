package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/prompt"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
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
	var cfg types.DevcontainerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid yaml: %w", err)
	}
	if !domain.IsValidDockerName(cfg.Workspace) {
		return fmt.Errorf("invalid workspace name: %s", cfg.Workspace)
	}
	if cfg.Image != "" && !domain.IsValidImageName(cfg.Image) {
		return fmt.Errorf("invalid image name: %s", cfg.Image)
	}
	if cfg.Compose.Subnet != "" && !domain.IsValidCidr(cfg.Compose.Subnet) {
		return fmt.Errorf("invalid CIDR: %s", cfg.Compose.Subnet)
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
			ok, err := prompt.Confirm("devcontainer.config.json exists. Overwrite?", false)
			if err != nil {
				return err
			}
			if !ok {
				ui.Cancelled()
				return nil
			}
		}
	}
	return domain.SaveConfig(&cfg, cwd)
}

func existingConfig(cwd string) bool {
	_, err := os.Stat(filepath.Join(cwd, types.ConfigFile))
	return err == nil
}

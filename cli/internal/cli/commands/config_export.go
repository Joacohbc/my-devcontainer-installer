package commands

import (
	"os"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func newConfigExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the current project's config as YAML",
		Long: `devcontainer-cli config export — render the current project's
devcontainer.config.json as human-friendly YAML.

Useful for reviewing the config, checking it into version control, or sharing a
reproducible setup that someone else can recreate with 'config import'. Writes to
stdout by default, or to a file with -o.`,
		Example: `  # Print the config as YAML
  devcontainer-cli config export

  # Save it to a file
  devcontainer-cli config export -o devcontainer.yml`,
		SilenceUsage: true,
		RunE:         runConfigExport,
	}
	cmd.Flags().StringP("output", "o", "", "Output file (default stdout)")
	return cmd
}

func runConfigExport(cmd *cobra.Command, _ []string) error {
	cwd, err := currentDir()
	if err != nil {
		return err
	}
	data, err := service.ConfigService{Report: ui.Console{}}.ExportConfigYAML(cwd)
	if err != nil {
		return err
	}

	out, _ := cmd.Flags().GetString("output")
	if out == "" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(out, data, 0644)
}

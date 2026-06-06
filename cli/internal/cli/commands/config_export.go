package commands

import (
	"os"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func newConfigExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "export",
		Short:        "Export the current project's configuration to YAML",
		Long:         "devcontainer-cli export — converts devcontainer.config.json to a YAML format and outputs it to stdout.\n\nUseful for sharing, version-controlling, or migrating configurations across different environments or machines in a more human-readable format.\n\nScope: Active workspace\n\nExamples:\n  devcontainer-cli export -o config.yml",
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

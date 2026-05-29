package commands

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/spf13/cobra"
)

func newConfigExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "export",
		Short:        "Export devcontainer.config.json as YAML",
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
	cfg, err := domain.LoadConfig(cwd)
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("no devcontainer.config.json found in %s", cwd)
	}

	data, err := yaml.Marshal(cfg)
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

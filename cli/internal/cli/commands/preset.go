package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/spf13/cobra"
)

func init() { register(newPresetCommand()) }

func newPresetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preset",
		Short: "Manage presets",
	}
	cmd.AddCommand(newPresetListCommand())
	return cmd
}

func newPresetListCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List builtin and user-defined presets",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, p := range catalog.All(presetsDir()) {
				fmt.Printf("%-20s %-10s %s\n", p.ID, p.Source, p.Label)
				if len(p.Modules) > 0 {
					fmt.Printf("  modules:  %s\n", strings.Join(p.Modules, ", "))
				}
				if len(p.Services) > 0 {
					fmt.Printf("  services: %s\n", strings.Join(p.Services, ", "))
				}
			}
			return nil
		},
	}
}

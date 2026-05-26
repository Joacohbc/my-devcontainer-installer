package commands

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/domain"
	"github.com/spf13/cobra"
)

func init() { register(newConfigCommand()) }

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read or write global CLI config (e.g. 'config registry <url>')",
		Long: "devcontainer-cli config — read/write global CLI config\n\n" +
			"Config file: " + domain.GlobalConfigPath(),
		SilenceUsage: true,
	}
	cmd.AddCommand(newConfigRegistryCommand())
	return cmd
}

func newConfigRegistryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry [url]",
		Short: "Get/set the default image registry",
		Long: `devcontainer-cli config registry — get/set the default image registry

Usage:
  devcontainer-cli config registry                 # print current value
  devcontainer-cli config registry ghcr.io/myorg/  # set value
  devcontainer-cli config registry --unset         # remove key`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE:         runConfigRegistry,
	}
	cmd.Flags().Bool("unset", false, "Remove the registry key (revert to default)")
	return cmd
}

func runConfigRegistry(cmd *cobra.Command, args []string) error {
	unset, _ := cmd.Flags().GetBool("unset")

	if !unset && len(args) == 0 {
		cfg := domain.LoadGlobalConfig()
		effective := cfg.Registry
		if effective == "" {
			effective = domain.DefaultRegistry
		}
		fmt.Println(effective)
		if cfg.Registry == "" {
			color.New(color.FgWhite).Println("(default — not yet customized)")
		}
		return nil
	}

	if unset {
		cfg := domain.LoadGlobalConfig()
		cfg.Registry = ""
		if err := domain.SaveGlobalConfig(cfg); err != nil {
			return err
		}
		color.Green("✓ Unset registry (will use default: %s).", domain.DefaultRegistry)
		return nil
	}

	cfg := domain.LoadGlobalConfig()
	cfg.Registry = args[0]
	if err := domain.SaveGlobalConfig(cfg); err != nil {
		return err
	}
	color.Green("✓ Set registry = %s (in %s)", cfg.Registry, domain.GlobalConfigPath())
	return nil
}

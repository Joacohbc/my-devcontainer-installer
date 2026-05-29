package commands

import (
	"fmt"
	"strconv"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
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
	cmd.AddCommand(newConfigStringDefaultCommand("db-user", "Default DB user"))
	cmd.AddCommand(newConfigStringDefaultCommand("db-password", "Default DB password"))
	cmd.AddCommand(newConfigIntDefaultCommand("ssh-port", "Default host SSH port for windows mode"))
	cmd.AddCommand(newConfigExportCommand())
	cmd.AddCommand(newConfigImportCommand())
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

func newConfigStringDefaultCommand(name, description string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          name + " [value]",
		Short:        "Get/set the default " + description,
		Long:         fmt.Sprintf("devcontainer-cli config %s — get/set the default %s\n\nUsage:\n  devcontainer-cli config %s                 # print current value\n  devcontainer-cli config %s <value>         # set value\n  devcontainer-cli config %s --unset         # remove key", name, description, name, name, name),
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			unset, _ := cmd.Flags().GetBool("unset")
			cfg := domain.LoadGlobalConfig()
			if cfg.Defaults == nil {
				cfg.Defaults = &domain.Defaults{}
			}

			var valPtr *string
			var fallback string
			if name == "db-user" {
				valPtr = &cfg.Defaults.DBUser
				fallback = "devuser"
			} else if name == "db-password" {
				valPtr = &cfg.Defaults.DBPassword
				fallback = "devpass"
			}

			if !unset && len(args) == 0 {
				effective := *valPtr
				if effective == "" {
					effective = fallback
				}
				fmt.Println(effective)
				if *valPtr == "" {
					color.New(color.FgWhite).Println("(default — not yet customized)")
				}
				return nil
			}

			if unset {
				*valPtr = ""
				if err := domain.SaveGlobalConfig(cfg); err != nil {
					return err
				}
				color.Green("✓ Unset %s (will use default: %s).", name, fallback)
				return nil
			}

			*valPtr = args[0]
			if err := domain.SaveGlobalConfig(cfg); err != nil {
				return err
			}
			color.Green("✓ Set %s = %s (in %s)", name, *valPtr, domain.GlobalConfigPath())
			return nil
		},
	}
	cmd.Flags().Bool("unset", false, fmt.Sprintf("Remove the %s key (revert to default)", name))
	return cmd
}

func newConfigIntDefaultCommand(name, description string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          name + " [value]",
		Short:        "Get/set the default " + description,
		Long:         fmt.Sprintf("devcontainer-cli config %s — get/set the default %s\n\nUsage:\n  devcontainer-cli config %s                 # print current value\n  devcontainer-cli config %s <value>         # set value\n  devcontainer-cli config %s --unset         # remove key", name, description, name, name, name),
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			unset, _ := cmd.Flags().GetBool("unset")
			cfg := domain.LoadGlobalConfig()
			if cfg.Defaults == nil {
				cfg.Defaults = &domain.Defaults{}
			}

			var valPtr *int
			var fallback int
			if name == "ssh-port" {
				valPtr = &cfg.Defaults.SSHHostPort
				fallback = 2222
			}

			if !unset && len(args) == 0 {
				effective := *valPtr
				if effective == 0 {
					effective = fallback
				}
				fmt.Println(effective)
				if *valPtr == 0 {
					color.New(color.FgWhite).Println("(default — not yet customized)")
				}
				return nil
			}

			if unset {
				*valPtr = 0
				if err := domain.SaveGlobalConfig(cfg); err != nil {
					return err
				}
				color.Green("✓ Unset %s (will use default: %d).", name, fallback)
				return nil
			}

			port, err := strconv.Atoi(args[0])
			if err != nil || port < 1 || port > 65535 {
				return fmt.Errorf("invalid port: %s (must be between 1 and 65535)", args[0])
			}

			*valPtr = port
			if err := domain.SaveGlobalConfig(cfg); err != nil {
				return err
			}
			color.Green("✓ Set %s = %d (in %s)", name, *valPtr, domain.GlobalConfigPath())
			return nil
		},
	}
	cmd.Flags().Bool("unset", false, fmt.Sprintf("Remove the %s key (revert to default)", name))
	return cmd
}

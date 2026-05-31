package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
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
	cmd.AddCommand(newConfigKeyCommand("registry", "image registry"))
	cmd.AddCommand(newConfigKeyCommand("db-user", "DB user"))
	cmd.AddCommand(newConfigKeyCommand("db-password", "DB password"))
	cmd.AddCommand(newConfigKeyCommand("ssh-port", "host SSH port for windows mode"))
	cmd.AddCommand(newConfigExportCommand())
	cmd.AddCommand(newConfigImportCommand())
	return cmd
}

func newConfigKeyCommand(key, description string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   key + " [value]",
		Short: "Get/set the default " + description,
		Long: fmt.Sprintf("devcontainer-cli config %s — get/set the default %s\n\nUsage:\n"+
			"  devcontainer-cli config %s                 # print current value\n"+
			"  devcontainer-cli config %s <value>         # set value\n"+
			"  devcontainer-cli config %s --unset         # remove key", key, description, key, key, key),
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigKey(cmd, key, args)
		},
	}
	cmd.Flags().Bool("unset", false, fmt.Sprintf("Remove the %s key (revert to default)", key))
	return cmd
}

func runConfigKey(cmd *cobra.Command, key string, args []string) error {
	unset, _ := cmd.Flags().GetBool("unset")
	svc := service.ConfigService{Report: ui.Console{}}

	switch {
	case unset:
		return svc.UnsetGlobalDefault(key)
	case len(args) == 0:
		value, customized, err := svc.GetGlobalDefault(key)
		if err != nil {
			return err
		}
		fmt.Println(value)
		if !customized {
			fmt.Println(ui.Subtle("(default — not yet customized)"))
		}
		return nil
	default:
		return svc.SetGlobalDefault(key, args[0])
	}
}

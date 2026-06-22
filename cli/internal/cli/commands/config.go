package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newConfigCommand()) }

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read or write the global CLI config and manage presets",
		Long: `devcontainer-cli config — read and write the global, machine-wide CLI defaults
and manage reusable module presets.

These defaults are applied to every project so you don't repeat them on each
generate (e.g. a default registry, DB credentials, the shared SSH key). Run a
key subcommand with no value to print the current value, with a value to set it,
or with --unset to revert to the built-in default.

Subcommands:
  registry            Default container registry prefix for remote images.
  db-user             Default database user for DB services.
  db-password         Default database password for DB services.
  ssh-key             Path to the shared managed SSH key (and key utilities).
  preset              List/create/copy/remove reusable module-bundle presets.
  export / import     Export the project config to YAML / import it back.

Config file: ` + domain.GlobalConfigPath(),
		Example: `  # Show or set the default registry
  devcontainer-cli config registry
  devcontainer-cli config registry ghcr.io/myuser

  # Manage presets and export the current project config
  devcontainer-cli config preset list
  devcontainer-cli config export -o devcontainer.yml`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newConfigKeyCommand("registry", "image registry"))
	cmd.AddCommand(newConfigKeyCommand("db-user", "DB user"))
	cmd.AddCommand(newConfigKeyCommand("db-password", "DB password"))
	cmd.AddCommand(newConfigSSHKeyCommand())
	cmd.AddCommand(newConfigExportCommand())
	cmd.AddCommand(newConfigImportCommand())
	cmd.AddCommand(newPresetCommand())
	return cmd
}

func newConfigKeyCommand(key, description string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   key + " [value]",
		Short: "Get, set or unset the default " + description,
		Long: fmt.Sprintf("devcontainer-cli config %s — get, set or unset the global default %s,\n"+
			"applied to every project that doesn't override it.\n\n"+
			"With no argument it prints the current value (and whether it has been\n"+
			"customized); with a value it stores it; with --unset it reverts to the\n"+
			"built-in default.", key, description),
		Example: fmt.Sprintf("  devcontainer-cli config %s            # print current value\n"+
			"  devcontainer-cli config %s <value>    # set value\n"+
			"  devcontainer-cli config %s --unset    # revert to default", key, key, key),
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigKey(cmd, key, args)
		},
	}
	cmd.Flags().Bool("unset", false, fmt.Sprintf("Remove the %s key (revert to default)", key))
	return cmd
}

func newConfigSSHKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh-key [path]",
		Short: "Manage the single shared SSH key reused by every devcontainer",
		Long: `devcontainer-cli config ssh-key — manage the one shared SSH key that 'setup-ssh'
installs into every devcontainer.

With no argument it prints the configured key path and whether the key exists.
Pass a path to point the CLI at a different key, or use the flags to revert to
the default, generate the key, or print its public/private contents.`,
		Example: `  devcontainer-cli config ssh-key             # print path + existence
  devcontainer-cli config ssh-key ~/.ssh/id_ed25519   # set a custom key
  devcontainer-cli config ssh-key --generate  # create the managed key
  devcontainer-cli config ssh-key --public    # print the public key`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE:         runConfigSSHKey,
	}
	cmd.Flags().Bool("unset", false, "Remove the ssh-key path (revert to default)")
	cmd.Flags().Bool("generate", false, "Generate the managed key now if it does not exist")
	cmd.Flags().Bool("public", false, "Print the public key contents")
	cmd.Flags().Bool("private", false, "Print the private key contents")
	return cmd
}

func runConfigSSHKey(cmd *cobra.Command, args []string) error {
	unset, _ := cmd.Flags().GetBool("unset")
	generate, _ := cmd.Flags().GetBool("generate")
	public, _ := cmd.Flags().GetBool("public")
	private, _ := cmd.Flags().GetBool("private")
	svc := service.ConfigService{Report: console}

	switch {
	case unset:
		return svc.UnsetGlobalDefault("ssh-key")
	case len(args) == 1:
		return svc.SetGlobalDefault("ssh-key", args[0])
	}

	keyPath, _, err := svc.GetGlobalDefault("ssh-key")
	if err != nil {
		return err
	}
	ssh := service.SshService{Report: console}

	if generate {
		created, gerr := ssh.EnsureKey(keyPath)
		if gerr != nil {
			return gerr
		}
		if created {
			console.Ok(fmt.Sprintf("Generated managed key at %s", keyPath))
		} else {
			console.Ok(fmt.Sprintf("Key already exists: %s", keyPath))
		}
	}

	if public {
		pub, perr := ssh.PublicKey(keyPath)
		if perr != nil {
			return fmt.Errorf("could not read public key (generate it with --generate): %w", perr)
		}
		console.Print(strings.TrimRight(string(pub), "\n") + "\n")
		return nil
	}

	if private {
		priv, perr := ssh.PrivateKey(keyPath)
		if perr != nil {
			return fmt.Errorf("could not read private key (generate it with --generate): %w", perr)
		}
		console.Print(strings.TrimRight(string(priv), "\n") + "\n")
		return nil
	}

	if generate {
		return nil
	}

	console.Info("%s", keyPath)
	if fileExists(keyPath) && fileExists(keyPath+".pub") {
		console.Ok("Key exists.")
	} else {
		console.Warn("Key not created yet (run: config ssh-key --generate).")
	}
	return nil
}

func runConfigKey(cmd *cobra.Command, key string, args []string) error {
	unset, _ := cmd.Flags().GetBool("unset")
	svc := service.ConfigService{Report: console}

	switch {
	case unset:
		return svc.UnsetGlobalDefault(key)
	case len(args) == 0:
		value, customized, err := svc.GetGlobalDefault(key)
		if err != nil {
			return err
		}
		console.Info("%s", value)
		if !customized {
			console.Warn("(default — not yet customized)")
		}
		return nil
	default:
		return svc.SetGlobalDefault(key, args[0])
	}
}

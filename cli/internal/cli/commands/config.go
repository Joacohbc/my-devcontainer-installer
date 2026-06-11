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
		Short: "Read or write global CLI config (e.g. 'config registry <url>')",
		Long: "devcontainer-cli config — read/write global CLI config\n\n" +
			"Config file: " + domain.GlobalConfigPath(),
		SilenceUsage: true,
	}
	cmd.AddCommand(newConfigKeyCommand("registry", "image registry"))
	cmd.AddCommand(newConfigKeyCommand("db-user", "DB user"))
	cmd.AddCommand(newConfigKeyCommand("db-password", "DB password"))
	cmd.AddCommand(newConfigSSHKeyCommand())
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

func newConfigSSHKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh-key [path]",
		Short: "Get/set the shared managed SSH key path",
		Long: "devcontainer-cli config ssh-key — manage the single shared SSH key reused by every devcontainer\n\nUsage:\n" +
			"  devcontainer-cli config ssh-key            # print path and whether the key exists\n" +
			"  devcontainer-cli config ssh-key <path>     # set the key path\n" +
			"  devcontainer-cli config ssh-key --unset    # revert to the default managed path\n" +
			"  devcontainer-cli config ssh-key --generate # generate the managed key now if missing\n" +
			"  devcontainer-cli config ssh-key --public   # print the public key contents\n" +
			"  devcontainer-cli config ssh-key --private  # print the private key contents",
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

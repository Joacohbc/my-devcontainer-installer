package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/spf13/cobra"
)

// addYesFlag registers the -y/--yes flag (skip confirmation prompts).
func addYesFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompts")
}

// addInteractiveFlag ensures --no-interactive is available. In normal CLI execution,
// --no-interactive is registered as a persistent flag on the root command. For standalone
// subcommand unit tests that do not construct a root command, it registers the flag locally.
func addInteractiveFlag(cmd *cobra.Command) {
	if cmd.Flag(flagNoInteractive) == nil {
		cmd.Flags().Bool(flagNoInteractive, false, "Non-interactive mode (fail instead of prompting)")
	}
}

// addBuildFlag registers the canonical boolean --build flag.
func addBuildFlag(cmd *cobra.Command, defaultVal bool, usage string) {
	cmd.Flags().Bool(flagBuild, defaultVal, usage)
}

// addForwardFlags registers the shared flags for 'port-forward' and 'agent forward'.
func addForwardFlags(cmd *cobra.Command) {
	cmd.Flags().String("alias", "", "SSH host alias to use (bypasses auto-discovery)")
	cmd.Flags().String("service", "", "Compose service to map port to (default: localhost)")
	cmd.Flags().Bool("reverse", false, "Reverse every mapping without an explicit prefix: the container listens and reaches a service on this machine (a single mapping can still opt in with 'reverse:')")
	cmd.Flags().Bool("ephemeral", false, "Forward directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks")
	addContainerFlag(cmd)
	cmd.Flags().String("key", "", "Private key path (default: the shared managed key under the CLI config dir)")
	cmd.Flags().String("user", sshdefaults.User, "SSH user inside the container")

	_ = cmd.RegisterFlagCompletionFunc("alias", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listSshHosts(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("service", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		cwd, err := currentDir()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return listComposeServices(defaultComposeFile(cwd)), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.MarkFlagFilename("key")
}

func yesFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("yes")
	return v
}

func interactiveFlag(cmd *cobra.Command) bool {
	_ = cmd.InheritedFlags()
	noi, _ := cmd.Flags().GetBool(flagNoInteractive)
	return !noi
}

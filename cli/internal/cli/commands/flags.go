package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service/sshdefaults"
	"github.com/spf13/cobra"
)

// flagNoInteractive is the single name for the non-interactive flag across
// every command that prompts. Keeping it a constant lets tests grep or bind
// against it without drift.
const flagNoInteractive = "no-interactive"

func addInteractiveFlag(cmd *cobra.Command) {
	cmd.Flags().Bool(flagNoInteractive, false, "Fail if any value is missing instead of prompting")
}

func addYesFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompts")
}

// addForwardSessionFlags registers the port-forwarding flags shared across
// 'access forward', 'access forward via', 'agent forward', and 'agent forward via'.
func addForwardSessionFlags(cmd *cobra.Command) {
	cmd.Flags().String("service", "", "Compose service to map port to (default: localhost)")
	cmd.Flags().Bool("reverse", false, "Reverse every mapping without an explicit prefix: the container listens and reaches a service on this machine (a single mapping can still opt in with 'reverse:')")
	cmd.Flags().Bool("ephemeral", true, "Forward directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (--ephemeral=false opts into ~/.ssh/devcontainer-cli.config)")
	cmd.Flags().String("key", "", "Private key path (default: the shared managed key under the CLI config dir)")
	cmd.Flags().String("user", sshdefaults.User, "SSH user inside the container")
	addInteractiveFlag(cmd)
	_ = cmd.MarkFlagFilename("key")
}

// addForwardFlags registers the flags runPortForward reads. It is shared with
// the 'agent forward' facade.
func addForwardFlags(cmd *cobra.Command) {
	cmd.Flags().String("alias", "", "SSH host alias to use (bypasses auto-discovery)")
	addContainerFlag(cmd)
	addForwardSessionFlags(cmd)

	_ = cmd.RegisterFlagCompletionFunc("alias", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return getSSHAliases(), cobra.ShellCompDirectiveNoFileComp
	})
}

func addContainerFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("container", "c", "", "Explicit target container name (managed by devcontainer-cli)")
	_ = cmd.RegisterFlagCompletionFunc("container", completeRunningContainers)
}

func interactiveFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool(flagNoInteractive)
	return !v
}

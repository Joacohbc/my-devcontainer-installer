package commands

import "github.com/spf13/cobra"

// addYesFlag registers the -y/--yes flag (skip confirmation prompts).
func addYesFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompts")
}

// addInteractiveFlag registers --no-interactive (disable prompts).
func addInteractiveFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("no-interactive", false, "Non-interactive mode (fail instead of prompting)")
}

func yesFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("yes")
	return v
}

func interactiveFlag(cmd *cobra.Command) bool {
	noi, _ := cmd.Flags().GetBool("no-interactive")
	return !noi
}

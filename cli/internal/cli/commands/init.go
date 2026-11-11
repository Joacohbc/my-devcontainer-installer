package commands

import "github.com/spf13/cobra"

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a devcontainer for the current project with the interactive wizard",
		Long: `devcontainer-cli workspace init — walk the interactive wizard to design the
devcontainer for the current directory: build mode, Dockerfile modules, compose
services, profiles, agent skills, published ports and extra volumes.

Any value given as a flag skips its wizard step. Use '--no-interactive' to fail
instead of prompting. After the build or pull it asks whether to run
'docker compose up -d', and then which session to open: SSH (ephemeral, or
creating the key and registering the host), a shell, or none.

To detect the project stack and generate everything without questions, use
'workspace auto-init'.`,
		Example: `  # Open the wizard in the current directory
  devcontainer-cli workspace init

  # Pre-select modules and let the wizard ask for the rest
  devcontainer-cli workspace init --modules nodejs,golang

  # Build the image right after generating
  devcontainer-cli workspace init --build`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runGenerate,
	}

	addGenerateFlags(cmd)
	_ = cmd.Flags().MarkHidden(flagVersion)
	return cmd
}

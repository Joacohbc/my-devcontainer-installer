package commands

import (
	"github.com/charmbracelet/log"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/logger"
	"github.com/spf13/cobra"
)

// NewRootCommand builds the full command tree. The root command itself runs the
// default "generate" flow when invoked with no subcommand.
func NewRootCommand(v string) *cobra.Command {
	version = v
	root := &cobra.Command{
		Use:   "devcontainer-cli",
		Short: "Generate and manage devcontainer environments",
		Long:  "devcontainer CLI — generates Dockerfile and docker-compose.yml files, and manages devcontainer environments.\n\nThis is the main entry point for the CLI. Running it without subcommands initiates the interactive generation wizard.\n\nScope: Global CLI",
		// Run the generate flow when no subcommand is given.
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runGenerate,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				logger.SetLevel(log.DebugLevel)
				return nil
			}
			raw, _ := cmd.Flags().GetString("log-level")
			lvl, err := logger.ParseLevel(raw)
			if err != nil {
				return err
			}
			logger.SetLevel(lvl)
			return nil
		},
	}
	root.PersistentFlags().Bool("verbose", false, "Enable debug logging (shortcut for --log-level debug)")
	root.PersistentFlags().String("log-level", "warn", "Log level: debug|info|warn|error")

	addGenerateFlags(root)
	for _, c := range subcommands {
		root.AddCommand(c)
	}
	return root
}

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
		Short: "Generate and manage reproducible Docker devcontainers",
		Long: `devcontainer-cli — generate and manage reproducible Docker devcontainers.

Run with no subcommand to (re)generate the Dockerfile, docker-compose.yml and
.env for the current directory and then build/start the stack. Interactively it
walks a wizard: pick a build mode, choose Dockerfile modules (languages and
tools such as nodejs, golang, python), add compose services (databases like
postgres/mongo/redis, a tunnel, …), and set published ports and extra volumes.
Generated files are written under .dc_<workspace>/ next to your project and the
result is recorded so the other subcommands can find it.

The subcommands manage an existing project: lifecycle (up, down, start, stop,
restart, destroy), inspection (status, info, logs, ls, shell), access (setup-ssh,
port-forward, network, password), config (config, sync-config) and cleanup
(prune, remove-container, remove-image). Run 'devcontainer-cli <command> --help'
for the full details of any one.

Build modes:
  local-cached  Build the full Dockerfile/compose pipeline locally and tag the
                image by a content fingerprint so identical setups are reused.
  remote        Skip the Dockerfile and pull a prebuilt ghcr.io image for the
                chosen --variant; database services are still generated.

Run 'devcontainer-cli --help' for the full flag list, or
'devcontainer-cli <command> --help' for any subcommand.`,
		Example: `  # Generate, build and start a devcontainer for the current directory
  devcontainer-cli

  # Non-interactive generation with explicit modules and a database service
  devcontainer-cli --with nodejs,golang --service postgres --no-interactive --force

  # Pull a prebuilt remote image instead of building locally
  devcontainer-cli --mode remote --variant nodejs

  # Start from a saved preset, then publish a port
  devcontainer-cli --preset web --ports 3000:3000

  # Work with the running container, then tear it down
  devcontainer-cli shell
  devcontainer-cli logs -f
  devcontainer-cli down`,
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

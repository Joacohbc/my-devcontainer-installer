package commands

import (
	"github.com/charmbracelet/log"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/logger"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/spf13/cobra"
)

// NewRootCommand builds the full command tree. The root command itself runs the
// default "generate" flow when invoked with no subcommand.
func NewRootCommand(v string) *cobra.Command {
	version = v
	types.CLIVersion = v
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

Commands are structured across 6 canonical resource domains:
  workspace    Manage project workspace lifecycle (init, auto-init, up, down, start, stop, restart, destroy, compose, export, import)
  access       Connect to and interact with running devcontainers (shell, ssh, forward, route, network, copy, password)
  inspect      Inspect container status, logs, files, and context (status, info, logs, context, ls)
  profile      Manage reusable devcontainer profiles (list, info, create, copy, remove)
  image        Manage reusable, named devcontainer images (list, info, create, build, pull, export, remove, update)
  config       Manage global CLI settings, skills, shared volume, and maintenance (registry, git-init, db-user, db-password, ssh-key, ssh-config-file, alias, shared, skill, clean, upgrade-cli)

Standalone utility:
  run          Spin up a container from a prebuilt remote image (no project files)

For AI assistants: the 'agent' facade provides an unattended, never-interactive command set
covering the whole loop (agent cli-info, create, run, ssh, exec, forward, list, destroy).

Build modes:
  custom    Build the full Dockerfile/compose pipeline locally and tag the
            image by a content fingerprint so identical setups are reused.
            Works for every profile, including a [local]-only one like
            'base' or a user-created profile.
  profiles  Skip the Dockerfile and pull a prebuilt ghcr.io image for the
            chosen --profile — a [remote]-tagged profile id (see
            'profile list'), or 'ssh' for the full image; database
            services are still generated.

Run 'devcontainer-cli --help' for the full flag list, or
'devcontainer-cli <command> --help' for any subcommand.`,
		Example: `  # Generate, build and start a devcontainer for the current directory
  devcontainer-cli

  # Non-interactive generation with explicit modules and a database service
  devcontainer-cli --modules nodejs,golang --services postgres --no-interactive --force

  # Pull a prebuilt remote image instead of building locally
  devcontainer-cli --mode profiles --profile nodejs

  # Run a throwaway container from a prebuilt remote image (no project files)
  devcontainer-cli run --profile nodejs

  # Manage workspace lifecycle
  devcontainer-cli workspace up
  devcontainer-cli workspace down

  # Access container
  devcontainer-cli access shell
  devcontainer-cli access ssh
  devcontainer-cli access forward 3000

  # Inspect container
  devcontainer-cli inspect status
  devcontainer-cli inspect logs -f`,
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
	root.PersistentFlags().Bool(flagNoInteractive, false, "Fail if any value is missing instead of prompting")

	addGenerateFlags(root)
	for _, c := range subcommands {
		root.AddCommand(c)
	}

	var initInherited func(*cobra.Command)
	initInherited = func(c *cobra.Command) {
		_ = c.InheritedFlags()
		for _, child := range c.Commands() {
			initInherited(child)
		}
	}
	initInherited(root)

	return root
}

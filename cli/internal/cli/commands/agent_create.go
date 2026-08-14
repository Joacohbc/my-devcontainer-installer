package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// flagNoUp opts out of the 'up' that otherwise closes an agent create.
const flagNoUp = "no-up"

// agentCreateHiddenFlags are the generate flags that make no sense on the agent
// facade. They stay registered (parseGenFlags reads the whole set) but are kept
// out of --help: --version belongs to the root, and the other three only exist
// for prompting or backwards compatibility, neither of which applies here.
var agentCreateHiddenFlags = []string{flagVersion, flagPreset, flagNonInteractive, flagForcePrompt}

func newAgentCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [flags]",
		Short: "Generate, build and start a devcontainer for the current directory",
		Long: `devcontainer-cli agent create — take the current directory from nothing to a
running devcontainer in one command: generate the Dockerfile, docker-compose.yml
and .env, build (or pull) the image, then start the stack.

It is the wizard's job done from flags. Where a bare 'devcontainer-cli' would
open the wizard and ask, this never prompts: modules, services and ports come
from the flags, existing generated files are overwritten, and the build runs
without asking. Re-run it with different flags to change an existing project.

Run 'agent cli-info' first for the module, service, profile and skill ids this
accepts.

The one thing it adds over generation is the start: the containers are running
when it returns, so 'agent exec' works immediately. Pass --no-up to stop after
the build.`,
		Example: `  # A Node project with a database
  devcontainer-cli agent create --with nodejs,pnpm --service postgres

  # Pull a prebuilt image instead of building one
  devcontainer-cli agent create --mode profiles --profile nodejs

  # Start from a profile and publish a port
  devcontainer-cli agent create --profile scraper --ports 3000:3000

  # Generate and build, but leave the stack down
  devcontainer-cli agent create --with golang --no-up`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		PreRunE:      agentCreateDefaults,
		RunE:         runAgentCreate,
	}
	addGenerateFlags(cmd)
	for _, name := range agentCreateHiddenFlags {
		_ = cmd.Flags().MarkHidden(name)
	}
	cmd.Flags().Bool(flagNoUp, false, "Generate and build only; leave the containers stopped")
	return cmd
}

// agentCreateDefaults applies what makes this "create" rather than "generate":
// no prompts, no overwrite question, and a build that just runs.
func agentCreateDefaults(cmd *cobra.Command, _ []string) error {
	return agentDefaults(cmd, map[string]string{
		flagNoInteractive: "true",
		flagForce:         "true",
		flagBuild:         "true",
	})
}

func runAgentCreate(cmd *cobra.Command, args []string) error {
	if err := runGenerate(cmd, args); err != nil {
		return err
	}
	if noUp, _ := cmd.Flags().GetBool(flagNoUp); noUp {
		return nil
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	composeFile, err := resolveProjectComposeFile(cwd)
	if err != nil {
		return err
	}

	// The image was already built (or pulled) by the generate above, so this
	// only has to create and start the containers.
	if err := (service.LifecycleService{Report: console}).Up(composeFile, resolveWorkspace(cwd), false); err != nil {
		return err
	}
	console.Done()
	return nil
}

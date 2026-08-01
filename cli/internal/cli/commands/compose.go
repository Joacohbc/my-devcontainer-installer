package commands

import (
	"fmt"
	"os"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newComposeCommand()) }

func newComposeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "compose [docker compose args...]",
		Aliases: []string{"dc"},
		Short:   "Run any docker compose command against the project's stack",
		Long: `devcontainer-cli compose (alias: dc) — a passthrough to 'docker compose' scoped
to the current project. Every argument is forwarded verbatim after
'-f .dc_<workspace>/build/docker-compose.yml --env-file .dc_<workspace>/build/.env'.

Use it as the escape hatch for anything the curated wrappers (start/stop/restart,
logs, status, copy) don't cover — 'exec', 'ps', 'top', 'config', 'kill', 'run',
'port', … — reaching every service in the stack, database services included,
without retyping the compose file and env-file paths. Because arguments pass
straight through, use flags exactly as docker compose expects them (e.g.
'compose exec -it postgres psql').`,
		Example: `  # Open psql in the postgres service
  devcontainer-cli dc exec -it postgres psql -U devuser

  # Compose-native ps for the project
  devcontainer-cli dc ps

  # Follow logs for two services
  devcontainer-cli compose logs -f devcontainer redis`,
		SilenceUsage:       true,
		DisableFlagParsing: true,
		RunE:               runCompose,
	}
	return cmd
}

func runCompose(cmd *cobra.Command, args []string) error {
	// Flag parsing is disabled so every token reaches docker compose. Surface our
	// own help for a bare invocation or a lone -h/--help; a "<verb> --help" still
	// forwards to docker compose.
	if len(args) == 0 || (len(args) == 1 && (args[0] == "-h" || args[0] == "--help")) {
		return cmd.Help()
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}

	paths := project.ProjectPaths(cwd, resolveWorkspace(cwd))
	if _, statErr := os.Stat(paths.ComposeFile); os.IsNotExist(statErr) {
		return fmt.Errorf("no compose file found at %s. Run 'devcontainer-cli' to generate one first", paths.ComposeFile)
	}

	// Only pass --env-file when the generated .env is present; otherwise let
	// compose fall back to its own autoload so a missing file isn't a hard error.
	envFile := paths.EnvPath
	if _, statErr := os.Stat(envFile); statErr != nil {
		envFile = ""
	}

	svc := service.LifecycleService{Report: ui.Console{}}
	return svc.Passthrough(paths.ComposeFile, envFile, args)
}

package commands

import (
	"sort"
	"strings"

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
		ValidArgsFunction:  completeComposeArgs,
	}
	return cmd
}

// composeVerbs are the compose subcommands offered for the first token; the
// passthrough still forwards anything the user types by hand.
var composeVerbs = []string{
	"exec\tRun a command in a running service",
	"ps\tList the project's containers",
	"logs\tView service output",
	"top\tShow running processes",
	"run\tRun a one-off command",
	"restart\tRestart services",
	"start\tStart services",
	"stop\tStop services",
	"up\tCreate and start services",
	"down\tStop and remove the stack",
	"build\tBuild service images",
	"pull\tPull service images",
	"config\tParse and render the compose file",
	"port\tPrint a service's public port",
	"kill\tForce-stop services",
	"pause\tPause services",
	"unpause\tUnpause services",
	"cp\tCopy files to/from a service",
	"events\tStream container events",
}

// completeComposeArgs completes the first token with compose verbs and later
// tokens with the project's compose service keys — what `docker compose <verb>
// <service>` expects, not container names.
func completeComposeArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		var out []string
		for _, v := range composeVerbs {
			if strings.HasPrefix(v, toComplete) {
				out = append(out, v)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
	return composeServiceNames(toComplete), cobra.ShellCompDirectiveNoFileComp
}

// composeServiceNames returns the project's compose service keys matching
// toComplete, or nil when the project has not been generated yet.
func composeServiceNames(toComplete string) []string {
	cwd, err := currentDir()
	if err != nil {
		return nil
	}
	paths := project.ProjectPaths(cwd, resolveWorkspace(cwd))
	services := service.ReadComposeServices(paths.ComposeFile)
	var out []string
	for name := range services {
		if strings.HasPrefix(name, toComplete) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func runCompose(cmd *cobra.Command, args []string) error {
	// With flag parsing disabled, cobra won't intercept help, so handle it here;
	// "<verb> --help" still forwards to docker compose.
	if len(args) == 0 || (len(args) == 1 && (args[0] == "-h" || args[0] == "--help")) {
		return cmd.Help()
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	composeFile, err := resolveProjectComposeFile(cwd)
	if err != nil {
		return err
	}

	svc := service.LifecycleService{Report: ui.Console{}}
	return svc.Passthrough(composeFile, resolveProjectEnvFile(cwd), args)
}

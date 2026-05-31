package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	for _, verb := range []string{"start", "stop", "restart"} {
		register(newLifecycleCommand(verb))
	}
}

func newLifecycleCommand(verb string) *cobra.Command {
	return &cobra.Command{
		Use:   verb,
		Short: "docker compose " + verb + " for the current project",
		Long: "devcontainer-cli " + verb + " — docker compose " + verb + " for the current project\n\n" +
			"Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml " + verb,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLifecycle(verb)
		},
	}
}

func runLifecycle(verb string) error {
	cwd, err := currentDir()
	if err != nil {
		return err
	}
	composeFile, err := resolveProjectComposeFile(cwd)
	if err != nil {
		return err
	}
	svc := service.LifecycleService{Report: ui.Console{}}
	if err := svc.Compose(composeFile, verb); err != nil {
		return err
	}
	ui.Done()
	return nil
}

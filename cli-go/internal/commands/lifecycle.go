package commands

import (
	"os"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/infra/project"
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
	cwd, _ := os.Getwd()
	composeFile, err := project.ResolveProjectComposeFile(cwd)
	if err != nil {
		return err
	}
	color.Yellow("\nRunning 'docker compose %s'...\n", verb)
	if err := docker.DockerComposeOrThrow(composeFile, []string{verb}, nil); err != nil {
		return err
	}
	color.New(color.FgGreen, color.Bold).Print("\nDone.\n\n")
	return nil
}

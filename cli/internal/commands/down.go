package commands

import (
	"os"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/prompt"
	"github.com/spf13/cobra"
)

func init() { register(newDownCommand()) }

func newDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "docker compose down for the current project (-v to drop volumes)",
		Long: `devcontainer-cli down — stop and remove project containers

Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml down
Add -v/--volumes to also remove named volumes (deletes data).`,
		SilenceUsage: true,
		RunE:         runDown,
	}
	cmd.Flags().BoolP("volumes", "v", false, "Also remove named volumes (docker compose down -v)")
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runDown(cmd *cobra.Command, _ []string) error {
	cwd, _ := os.Getwd()
	cfg, _ := domain.LoadConfig(cwd)
	workspace := project.ResolveWorkspace(cwd, cfg)
	composeFile, err := project.ResolveProjectComposeFile(cwd)
	if err != nil {
		return err
	}

	removeVolumes, _ := cmd.Flags().GetBool("volumes")
	if !removeVolumes && !yesFlag(cmd) && interactiveFlag(cmd) {
		ok, perr := prompt.Confirm("Also remove named volumes for '"+workspace+"'? This deletes their data.", false)
		if perr != nil {
			return perr
		}
		removeVolumes = ok
	}

	args := []string{"down"}
	if removeVolumes {
		args = append(args, "-v")
	}

	suffix := ""
	if removeVolumes {
		suffix = " (with volumes)"
	}
	color.Yellow("\nBringing down '%s'%s...\n", workspace, suffix)
	if err := docker.DockerComposeOrThrow(composeFile, args, nil); err != nil {
		return err
	}
	color.New(color.FgGreen, color.Bold).Print("\nDone.\n\n")
	return nil
}

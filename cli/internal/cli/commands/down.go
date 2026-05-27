package commands

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/prompt"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/spf13/cobra"
)

func init() { register(newDownCommand()) }

func newDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Bring down project containers",
		Long: `devcontainer-cli down — stop and remove project containers

Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml down
Add -v/--volumes to also remove named volumes (deletes data).`,
		SilenceUsage: true,
		RunE:         runDown,
	}
	cmd.Flags().BoolP("volumes", "v", false, "Also remove named volumes (docker compose down -v)")
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	addContainerFlag(cmd)
	return cmd
}

func runDown(cmd *cobra.Command, _ []string) error {
	wsFlag, _ := cmd.Flags().GetString("workspace")

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return err
		}
		color.Yellow("\nStopping and removing container '%s'...\n", containerName)
		_, _ = docker.DockerInherit([]string{"stop", containerName})
		status, err := docker.DockerInherit([]string{"rm", containerName})
		if err != nil {
			return err
		}
		if status != 0 {
			return fmt.Errorf("docker rm failed")
		}
		color.New(color.FgGreen, color.Bold).Print("\nDone.\n\n")
		return nil
	}

	cwd, _ := os.Getwd()
	cfg, _ := domain.LoadConfig(cwd)
	workspace := domain.ResolveWorkspace(cwd, cfg)
	composeFile, err := resolveProjectComposeFile(cwd)
	if err != nil {
		return err
	}

	removeVolumes, _ := cmd.Flags().GetBool("volumes")
	if !removeVolumes {
		if yesFlag(cmd) {
			removeVolumes = true
		} else if interactiveFlag(cmd) {
			ok, perr := prompt.Confirm("Also remove named volumes for '"+workspace+"'? This deletes their data.", false)
			if perr != nil {
				return perr
			}
			removeVolumes = ok
		}
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

package commands

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/spf13/cobra"
)

func init() { register(newUpCommand()) }

func newUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create and start project containers",
		Long: `devcontainer-cli up — build, recreate, start, and attach to containers for a service

Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml up -d`,
		SilenceUsage: true,
		RunE:         runUp,
	}
	cmd.Flags().StringP("workspace", "w", "", "Workspace name")
	cmd.Flags().Bool("build", false, "Build images before starting containers (docker compose up -d --build)")
	addContainerFlag(cmd)
	return cmd
}

func runUp(cmd *cobra.Command, _ []string) error {
	wsFlag, _ := cmd.Flags().GetString("workspace")

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return err
		}
		color.Yellow("\nStarting container '%s'...\n", containerName)
		status, err := docker.DockerInherit([]string{"start", containerName})
		if err != nil {
			return err
		}
		if status != 0 {
			return fmt.Errorf("docker start failed")
		}
		color.New(color.FgGreen, color.Bold).Print("\nDone.\n\n")
		return nil
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	composeFile, err := resolveProjectComposeFileWithWorkspace(cwd, wsFlag)
	if err != nil {
		return err
	}

	var workspace string
	if wsFlag != "" {
		workspace = wsFlag
	} else {
		cfg, _ := domain.LoadConfig(cwd)
		workspace = domain.ResolveWorkspace(cwd, cfg)
	}

	build, _ := cmd.Flags().GetBool("build")

	args := []string{"up", "-d"}
	if build {
		args = append(args, "--build")
	}

	color.Yellow("\nBringing up '%s'...\n", workspace)
	if err := docker.DockerComposeOrThrow(composeFile, args, nil); err != nil {
		return err
	}
	color.New(color.FgGreen, color.Bold).Print("\nDone.\n\n")
	return nil
}

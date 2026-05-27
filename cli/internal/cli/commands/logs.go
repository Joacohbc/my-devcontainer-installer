package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/spf13/cobra"
)

func init() { register(newLogsCommand()) }

func newLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [service]",
		Short: "View logs from project containers",
		Long:  `devcontainer-cli logs — wrapper around docker compose logs to view container logs`,
		RunE:  runLogs,
	}
	cmd.Flags().StringP("workspace", "w", "", "Workspace name")
	cmd.Flags().BoolP("follow", "f", false, "Follow log output")
	cmd.Flags().String("tail", "all", "Number of lines to show from the end of the logs")
	addContainerFlag(cmd)
	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	wsFlag, _ := cmd.Flags().GetString("workspace")
	follow, _ := cmd.Flags().GetBool("follow")
	tail, _ := cmd.Flags().GetString("tail")

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return err
		}
		dockerArgs := []string{"logs"}
		if follow {
			dockerArgs = append(dockerArgs, "-f")
		}
		if tail != "" {
			dockerArgs = append(dockerArgs, "--tail", tail)
		}
		dockerArgs = append(dockerArgs, containerName)

		status, err := docker.DockerInherit(dockerArgs)
		if err != nil {
			return err
		}
		if status != 0 {
			return fmt.Errorf("docker logs failed with exit code %d", status)
		}
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

	composeArgs := []string{"logs"}
	if follow {
		composeArgs = append(composeArgs, "--follow")
	}
	if tail != "" {
		composeArgs = append(composeArgs, "--tail", tail)
	}
	if len(args) > 0 {
		composeArgs = append(composeArgs, args...)
	}

	status, err := docker.DockerCompose(composeFile, composeArgs, nil)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("docker compose logs failed with exit code %d", status)
	}
	return nil
}

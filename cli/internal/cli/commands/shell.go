package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/spf13/cobra"
)

func init() { register(newShellCommand()) }

func newShellCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell [flags] [-- command args...]",
		Short: "Open an interactive shell in the devcontainer",
		Long:  `devcontainer-cli shell — shortcut for docker exec -it <container> <shell>`,
		RunE:  runShell,
	}
	cmd.Flags().StringP("workspace", "w", "", "Workspace name")
	cmd.Flags().String("user", "", "User to run the command as (e.g. root)")
	addContainerFlag(cmd)
	return cmd
}

func runShell(cmd *cobra.Command, args []string) error {
	wsFlag, _ := cmd.Flags().GetString("workspace")
	userFlag, _ := cmd.Flags().GetString("user")

	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return err
	}

	// Check if container is running
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", "{{.State.Status}}", containerName})
	state := strings.TrimSpace(stdout)
	if err != nil || status != 0 || state != "running" {
		return fmt.Errorf("container '%s' is not running. Run 'devcontainer-cli' or 'devcontainer-cli start' first", containerName)
	}

	execArgs := []string{"exec", "-it"}
	if userFlag != "" {
		execArgs = append(execArgs, "-u", userFlag)
	}
	execArgs = append(execArgs, containerName)

	if len(args) > 0 {
		execArgs = append(execArgs, args...)
	} else {
		execArgs = append(execArgs, "sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi")
	}

	exitCode, err := docker.DockerInherit(execArgs)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("shell session exited with code %d", exitCode)
	}
	return nil
}

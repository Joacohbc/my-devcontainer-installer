package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/spf13/cobra"
)

func init() { register(newLsCommand()) }

func newLsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls [container_path]",
		Short: "List files and directories inside the devcontainer",
		Long: `devcontainer-cli ls — wrapper for ls command inside the devcontainer

Supports real-time dynamic completion for paths inside the container.`,
		SilenceUsage:      true,
		ValidArgsFunction: runLsCompletion,
		RunE:              runLs,
	}
	cmd.Flags().StringP("workspace", "w", "", "Workspace name")
	cmd.Flags().BoolP("all", "a", false, "Show hidden files (ls -a)")
	cmd.Flags().BoolP("long", "l", false, "Use a long listing format (ls -l)")
	addContainerFlag(cmd)
	return cmd
}

func runLs(cmd *cobra.Command, args []string) error {
	containerPath := "."
	if len(args) > 0 {
		containerPath = args[0]
	}

	wsFlag, _ := cmd.Flags().GetString("workspace")
	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return err
	}

	// Verify container is running
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", "{{.State.Status}}", containerName})
	state := strings.TrimSpace(stdout)
	if err != nil || status != 0 || state != "running" {
		return fmt.Errorf("container '%s' is not running. Run 'devcontainer-cli' or 'devcontainer-cli start' first", containerName)
	}

	all, _ := cmd.Flags().GetBool("all")
	long, _ := cmd.Flags().GetBool("long")

	execArgs := []string{"exec", "-it", containerName, "ls"}

	if long {
		execArgs = append(execArgs, "-l")
	}
	if all {
		execArgs = append(execArgs, "-a")
	}
	// Add standard coloring flag to ls inside container
	execArgs = append(execArgs, "--color=auto")

	execArgs = append(execArgs, containerPath)

	exitCode, err := docker.DockerInherit(execArgs)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("ls failed with exit code %d", exitCode)
	}
	return nil
}

func runLsCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		wsFlag, _ := cmd.Flags().GetString("workspace")
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		dir := "."
		prefix := ""
		lastSlash := strings.LastIndex(toComplete, "/")
		if lastSlash != -1 {
			dir = toComplete[:lastSlash]
			if dir == "" {
				dir = "/"
			}
			prefix = toComplete[lastSlash+1:]
		} else {
			prefix = toComplete
		}

		status, stdout, _, err := docker.DockerCapture([]string{"exec", containerName, "ls", "-1", "-p", dir})
		if err != nil || status != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var suggestions []string
		for _, entry := range strings.Split(stdout, "\n") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if strings.HasPrefix(entry, prefix) {
				var fullPath string
				if lastSlash != -1 {
					if dir == "/" {
						fullPath = "/" + entry
					} else {
						fullPath = dir + "/" + entry
					}
				} else {
					fullPath = entry
				}
				suggestions = append(suggestions, fullPath)
			}
		}
		return suggestions, cobra.ShellCompDirectiveNoSpace
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

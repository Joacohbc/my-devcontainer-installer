package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/spf13/cobra"
)

func init() { register(newCopyCommand()) }

func newCopyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "copy [src_local_path] [dest_container_path]",
		Aliases: []string{"cp"},
		Short:   "Copy a file or directory from the host to the container",
		Long: `devcontainer-cli copy — copy a local file or directory into the running devcontainer

Supports real-time dynamic completion for paths inside the container.`,
		Args:              cobra.ExactArgs(2),
		SilenceUsage:      true,
		ValidArgsFunction: runCopyCompletion,
		RunE:              runCopy,
	}
	cmd.Flags().StringP("workspace", "w", "", "Workspace name")
	addContainerFlag(cmd)
	return cmd
}

func runCopy(cmd *cobra.Command, args []string) error {
	localPath := args[0]
	containerPath := args[1]

	wsFlag, _ := cmd.Flags().GetString("workspace")

	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return err
	}

	// Verify local path exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return fmt.Errorf("local path '%s' does not exist", localPath)
	}

	// Verify container is running
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", "{{.State.Status}}", containerName})
	state := strings.TrimSpace(stdout)
	if err != nil || status != 0 || state != "running" {
		return fmt.Errorf("container '%s' is not running. Run 'devcontainer-cli' or 'devcontainer-cli start' first", containerName)
	}

	ui.Yellow("\nCopying '%s' to '%s' in container '%s'...", localPath, containerPath, containerName)

	exitCode, err := docker.DockerInherit([]string{"cp", localPath, containerName + ":" + containerPath})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("copy failed with exit code %d", exitCode)
	}

	fmt.Print(ui.StyleSuccess.Bold(true).Render("\nSuccessfully copied.") + "\n\n")
	return nil
}

func runCopyCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		// Local file path auto-completion
		return nil, cobra.ShellCompDirectiveDefault
	}
	if len(args) == 1 {
		// Container destination path auto-completion via dynamic ls
		wsFlag, _ := cmd.Flags().GetString("workspace")
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Split toComplete into dir and prefix
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

		// Exec 'ls -1 -p' in container to fetch subfiles and folders with trailing slashes
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

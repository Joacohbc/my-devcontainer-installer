package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/pick"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// resolveWorkspace returns wsFlag when set, otherwise the workspace derived from
// the project config (falling back to the sanitized directory name). It is the
// single source of the flag-or-config workspace rule shared across commands.
func resolveWorkspace(cwd, wsFlag string) string {
	if wsFlag != "" {
		return wsFlag
	}
	cfg, _ := domain.LoadConfig(cwd)
	return domain.ResolveWorkspace(cwd, cfg)
}

// relativeComposeFile returns the project-relative compose path for a workspace.
func relativeComposeFile(workspace string) string {
	return fmt.Sprintf(".dc_%s/build/docker-compose.yml", workspace)
}

// defaultComposeFile is the relative compose path for the workspace inferred from
// cwd, used for flag defaults and completion (no existence check).
func defaultComposeFile(cwd string) string {
	return relativeComposeFile(resolveWorkspace(cwd, ""))
}

func resolveProjectComposeFile(cwd string) (string, error) {
	return resolveProjectComposeFileWithWorkspace(cwd, "")
}

func resolveProjectComposeFileWithWorkspace(cwd string, wsFlag string) (string, error) {
	paths := project.ProjectPaths(cwd, resolveWorkspace(cwd, wsFlag))
	if _, statErr := os.Stat(paths.ComposeFile); os.IsNotExist(statErr) {
		return "", fmt.Errorf("no compose file found at %s. Run 'devcontainer-cli' to generate one first", paths.ComposeFile)
	}
	return paths.ComposeFile, nil
}

func resolveDevcontainerContainer(cwd string, wsFlag string) (string, error) {
	workspace := resolveWorkspace(cwd, wsFlag)
	paths := project.ProjectPaths(cwd, workspace)
	services := service.ReadComposeServices(paths.ComposeFile)
	if services != nil {
		if picked, ok := service.PickDevcontainerService(services); ok {
			return picked.Container, nil
		}
	}
	return workspace + "-" + sshdefaults.ServiceName, nil
}

func resolveContainer(cmd *cobra.Command, wsFlag string) (string, error) {
	if cmd.Flags().Changed("container") {
		containerFlag, _ := cmd.Flags().GetString("container")
		if containerFlag != "" {
			return containerFlag, nil
		}
	}
	cwd, err := currentDir()
	if err != nil {
		return "", err
	}
	return resolveDevcontainerContainer(cwd, wsFlag)
}

func addContainerFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("container", "c", "", "Explicit target container name (managed by devcontainer-cli)")
	_ = cmd.RegisterFlagCompletionFunc("container", completeContainers)
}

func completeContainers(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var suggestions []string
	for _, c := range pick.ListManaged() {
		if strings.HasPrefix(c.Name, toComplete) {
			suggestions = append(suggestions, c.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

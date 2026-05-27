package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/pick"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/spf13/cobra"
)

func resolveProjectComposeFile(cwd string) (string, error) {
	return resolveProjectComposeFileWithWorkspace(cwd, "")
}

func resolveProjectComposeFileWithWorkspace(cwd string, wsFlag string) (string, error) {
	var workspace string
	if wsFlag != "" {
		workspace = wsFlag
	} else {
		cfg, err := domain.LoadConfig(cwd)
		if err != nil {
			return "", err
		}
		workspace = domain.ResolveWorkspace(cwd, cfg)
	}
	paths := project.ProjectPaths(cwd, workspace)
	if _, statErr := os.Stat(paths.ComposeFile); os.IsNotExist(statErr) {
		return "", fmt.Errorf("no compose file found at %s. Run 'devcontainer-cli' to generate one first", paths.ComposeFile)
	}
	return paths.ComposeFile, nil
}

func resolveDevcontainerContainer(cwd string, wsFlag string) (string, error) {
	var workspace string
	if wsFlag != "" {
		workspace = wsFlag
	} else {
		cfg, _ := domain.LoadConfig(cwd)
		workspace = domain.ResolveWorkspace(cwd, cfg)
	}
	paths := project.ProjectPaths(cwd, workspace)
	services := readComposeServices(paths.ComposeFile)
	if services != nil {
		if picked := pickDevcontainerService(services); picked != nil {
			return picked.container, nil
		}
	}
	return workspace + "-devcontainer-ssh", nil
}

func resolveContainer(cmd *cobra.Command, wsFlag string) (string, error) {
	if cmd.Flags().Changed("container") {
		containerFlag, _ := cmd.Flags().GetString("container")
		if containerFlag != "" {
			return containerFlag, nil
		}
	}
	cwd, _ := os.Getwd()
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

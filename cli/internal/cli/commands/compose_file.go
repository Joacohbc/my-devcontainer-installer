package commands

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/pick"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// resolveWorkspace returns the workspace for the project in cwd, derived from its
// config (falling back to the sanitized directory name). Operational commands
// always act on the current directory's project — there is no workspace flag to
// target another one.
func resolveWorkspace(cwd string) string {
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
	return relativeComposeFile(resolveWorkspace(cwd))
}

func resolveProjectComposeFile(cwd string) (string, error) {
	paths := project.ProjectPaths(cwd, resolveWorkspace(cwd))
	if _, statErr := os.Stat(paths.ComposeFile); os.IsNotExist(statErr) {
		return "", fmt.Errorf("no compose file found at %s. Run 'devcontainer-cli' to generate one first", paths.ComposeFile)
	}
	return paths.ComposeFile, nil
}

func resolveDevcontainerContainer(cwd string) (string, error) {
	workspace := resolveWorkspace(cwd)
	paths := project.ProjectPaths(cwd, workspace)
	services := service.ReadComposeServices(paths.ComposeFile)
	if services != nil {
		if picked, ok := service.PickDevcontainerService(services); ok {
			return picked.Container, nil
		}
	}
	return workspace + "-" + sshdefaults.ServiceName, nil
}

func resolveContainer(cmd *cobra.Command) (string, error) {
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
	return resolveDevcontainerContainer(cwd)
}

// addContainerFlag registers --container with the generic (any-state) managed
// container completion.
func addContainerFlag(cmd *cobra.Command) {
	addContainerFlagFiltered(cmd, completeContainers)
}

// containerCompletionFunc is the signature cobra expects for flag completion.
type containerCompletionFunc func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

// addContainerFlagFiltered registers --container with a state-specific
// completion (e.g. start completes stopped containers, stop completes running
// ones).
func addContainerFlagFiltered(cmd *cobra.Command, complete containerCompletionFunc) {
	cmd.Flags().StringP("container", "c", "", "Explicit target container name (managed by devcontainer-cli)")
	_ = cmd.RegisterFlagCompletionFunc("container", complete)
}

// filterContainerNames returns the names of containers matching toComplete and,
// when keep is non-nil, for which keep returns true.
func filterContainerNames(containers []pick.Container, toComplete string, keep func(pick.Container) bool) []string {
	var suggestions []string
	for _, c := range containers {
		if !strings.HasPrefix(c.Name, toComplete) {
			continue
		}
		if keep != nil && !keep(c) {
			continue
		}
		suggestions = append(suggestions, c.Name)
	}
	return suggestions
}

// managedContainerNames suggests managed container names matching toComplete,
// keeping only those for which keep returns true. A nil keep accepts all.
func managedContainerNames(toComplete string, keep func(pick.Container) bool) ([]string, cobra.ShellCompDirective) {
	return filterContainerNames(pick.ListManaged(), toComplete, keep), cobra.ShellCompDirectiveNoFileComp
}

// containerRunning reports whether a container is in the running state, used as
// the keep predicate for state-specific completion.
func containerRunning(c pick.Container) bool { return c.State == "running" }

// completeManagedContainerArgs completes positional managed container names for
// remove-container, skipping names already present on the command line.
func completeManagedContainerArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var out []string
	for _, name := range filterContainerNames(pick.ListManaged(), toComplete, nil) {
		if !slices.Contains(args, name) {
			out = append(out, name)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeManagedImageArgs completes positional managed image references for
// remove-image, skipping refs already present on the command line.
func completeManagedImageArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	images, _ := service.PruneService{Report: console}.SelectImages(true)
	var out []string
	for _, img := range images {
		if strings.HasPrefix(img.Ref, toComplete) && !slices.Contains(args, img.Ref) {
			out = append(out, img.Ref)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeContainers completes managed containers in any state.
func completeContainers(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return managedContainerNames(toComplete, nil)
}

// completeRunningContainers completes managed containers that are running
// (used by stop).
func completeRunningContainers(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return managedContainerNames(toComplete, containerRunning)
}

// completeStoppedContainers completes managed containers that are not running
// (used by start).
func completeStoppedContainers(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return managedContainerNames(toComplete, func(c pick.Container) bool { return !containerRunning(c) })
}

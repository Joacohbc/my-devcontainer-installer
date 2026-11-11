package commands

import (
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// listSshHosts returns all non-wildcard Host aliases defined across the CLI's
// managed SSH config and the user's own ~/.ssh/config.
func listSshHosts() []string {
	return service.SshService{Report: ui.Console{}}.ConfigHostAliasesFromDisk()
}

// completeContainerPath suggests paths inside the container for shell completion,
// splitting toComplete into a directory to list and a prefix to filter by.
func completeContainerPath(containerName, toComplete string) ([]string, cobra.ShellCompDirective) {
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

	entries, err := service.InspectService{Report: ui.Console{}}.ListDir(containerName, dir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var suggestions []string
	for _, entry := range entries {
		if !strings.HasPrefix(entry, prefix) {
			continue
		}
		fullPath := entry
		if lastSlash != -1 {
			if dir == "/" {
				fullPath = "/" + entry
			} else {
				fullPath = dir + "/" + entry
			}
		}
		suggestions = append(suggestions, fullPath)
	}
	return suggestions, cobra.ShellCompDirectiveNoSpace
}

// listComposeServices retrieves defined service names from the specified compose file.
func listComposeServices(composeFile string) []string {
	services := service.ReadComposeServices(composeFile)
	if services == nil {
		return nil
	}
	var names []string
	for name := range services {
		names = append(names, name)
	}
	return names
}

// staticCompletion returns a completion function that always offers the given
// fixed items (with no file completion).
func staticCompletion(items ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return items, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeCSV autocompletes comma-separated list values, omitting already selected ones.
func completeCSV(toComplete string, allValues []string) []string {
	parts := strings.Split(toComplete, ",")
	prefix := ""
	if len(parts) > 1 {
		prefix = strings.Join(parts[:len(parts)-1], ",") + ","
	}
	lastPart := strings.TrimSpace(parts[len(parts)-1])

	selected := make(map[string]bool)
	for _, p := range parts[:len(parts)-1] {
		selected[strings.TrimSpace(p)] = true
	}

	var suggestions []string
	for _, val := range allValues {
		if !selected[val] && strings.HasPrefix(val, lastPart) {
			suggestions = append(suggestions, prefix+val)
		}
	}
	return suggestions
}

// imageNames lists the registered image definition names, optionally only those
// of the given sources (build or remote).
func imageNames(sources ...domain.ImageSource) []string {
	return service.ImageService{}.Names(sources...)
}

func withPrefix(items []string, prefix string) []string {
	var out []string
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return out
}

// completeImageNames completes a flag value with image definition names.
func completeImageNames(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return withPrefix(imageNames(), toComplete), cobra.ShellCompDirectiveNoFileComp
}

// completeImageArg completes the single <name> argument of an image subcommand
// with the definitions of the given sources (all when none are given): 'build'
// offers only build definitions and 'pull' only remote ones, so TAB never
// suggests a name the command would reject.
func completeImageArg(sources ...domain.ImageSource) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return withPrefix(imageNames(sources...), toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// completeImageExportArgs completes 'image export <name> <dir>': an image name,
// then a host directory.
func completeImageExportArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return completeImageArg()(cmd, args, toComplete)
	case 1:
		return nil, cobra.ShellCompDirectiveFilterDirs
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

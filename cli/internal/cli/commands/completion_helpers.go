package commands

import (
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
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

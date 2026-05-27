package commands

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// listSshHosts parses ~/.ssh/config and returns all defined non-wildcard Host aliases.
func listSshHosts() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	configPath := filepath.Join(home, ".ssh", "config")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var hosts []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		// Use the existing aliasOfHostLine package helper
		lineHosts := aliasOfHostLine(line)
		for _, h := range lineHosts {
			h = strings.TrimSpace(h)
			if h != "" && !strings.Contains(h, "*") && !strings.Contains(h, "?") && !seen[h] {
				seen[h] = true
				hosts = append(hosts, h)
			}
		}
	}
	return hosts
}

// listContainers queries all container names from the running Docker daemon.
func listContainers() []string {
	if !docker.IsDockerAvailable() {
		return nil
	}
	status, stdout, _, err := docker.DockerCapture([]string{"ps", "-a", "--format", "{{.Names}}"})
	if err != nil || status != 0 {
		return nil
	}
	var names []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	return names
}

// listComposeServices retrieves defined service names from the specified compose file.
func listComposeServices(composeFile string) []string {
	services := readComposeServices(composeFile)
	if services == nil {
		return nil
	}
	var names []string
	for name := range services {
		names = append(names, name)
	}
	return names
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

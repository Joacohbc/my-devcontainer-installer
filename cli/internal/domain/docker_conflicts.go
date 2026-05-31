package domain

import (
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

type Conflict struct {
	Kind  string
	Name  string
	Owner string
}

type CaptureFunc func(args []string) (status int, stdout, stderr string)

func FindConflicts(config *types.DevcontainerConfig, dockerAvailable bool, capture CaptureFunc) []Conflict {
	if !dockerAvailable {
		return nil
	}
	containers, network, _, err := PlannedComposeNames(config)
	if err != nil {
		return nil
	}
	project := types.ProjectID(config)

	var conflicts []Conflict

	existingContainers := listExistingDockerResources("container", capture)
	for _, name := range containers {
		owner, found := existingContainers[name]
		if !found {
			continue
		}
		if owner == project {
			continue
		}
		conflicts = append(conflicts, Conflict{Kind: "container", Name: name, Owner: orFallback(owner, "(no label)")})
	}

	existingNetworks := listExistingDockerResources("network", capture)
	if owner, found := existingNetworks[network]; found && owner != project {
		conflicts = append(conflicts, Conflict{Kind: "network", Name: network, Owner: orFallback(owner, "(no label)")})
	}

	return conflicts
}

func listExistingDockerResources(kind string, capture CaptureFunc) map[string]string {
	labelKey := types.LabelProject
	var args []string
	if kind == "container" {
		args = []string{"container", "ls", "-a", "--format", `{{.Names}}\t{{.Label "` + labelKey + `"}}`}
	} else {
		args = []string{"network", "ls", "--format", `{{.Name}}\t{{.Label "` + labelKey + `"}}`}
	}
	_, stdout, _ := capture(args)
	resources := make(map[string]string)
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			resources[parts[0]] = parts[1]
		} else {
			resources[parts[0]] = ""
		}
	}
	return resources
}

func orFallback(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

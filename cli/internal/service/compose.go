package service

import (
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

// ComposeService is the subset of a docker-compose service definition the CLI
// needs to resolve a target container.
type ComposeService struct {
	ContainerName string `yaml:"container_name"`
}

// ComposeTarget pairs a compose service key with its resolved container name.
type ComposeTarget struct {
	Service   string
	Container string
}

type composeDoc struct {
	Services map[string]ComposeService `yaml:"services"`
}

// ReadComposeServices parses composeFile and returns its services, or nil if the
// file is missing, malformed, or defines no services.
func ReadComposeServices(composeFile string) map[string]ComposeService {
	data, err := os.ReadFile(composeFile)
	if err != nil {
		return nil
	}
	var doc composeDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	if len(doc.Services) == 0 {
		return nil
	}
	return doc.Services
}

// ContainerOf returns the explicit container_name of svc, falling back to its
// compose service key when none is set.
func ContainerOf(svc ComposeService, key string) string {
	if svc.ContainerName != "" {
		return svc.ContainerName
	}
	return key
}

// PickDevcontainerService finds the devcontainer-ssh service among services:
// first by an exact service/container match, then by a unique "devcontainer"
// substring match. It returns false when no unambiguous candidate exists.
func PickDevcontainerService(services map[string]ComposeService) (ComposeTarget, bool) {
	for key, svc := range services {
		if key == sshdefaults.ServiceName || ContainerOf(svc, key) == sshdefaults.ServiceName {
			return ComposeTarget{Service: key, Container: ContainerOf(svc, key)}, true
		}
	}
	var candidates []ComposeTarget
	for key, svc := range services {
		container := ContainerOf(svc, key)
		if strings.Contains(key, "devcontainer") || strings.Contains(container, "devcontainer") {
			candidates = append(candidates, ComposeTarget{Service: key, Container: container})
		}
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	return ComposeTarget{}, false
}

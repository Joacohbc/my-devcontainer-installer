package service

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// NetworkService attaches and detaches arbitrary containers to/from the docker
// network of a devcontainer workspace. The workspace network is the managed
// bridge network docker compose created for the project; this service resolves
// its real name by label and runs `docker network connect`/`disconnect`.
//
// Any container can be wired onto the network — it does not need to be managed
// by this CLI, nor does it need to be (re)created from the compose file.
type NetworkService struct {
	Report Reporter
}

// EnsureDocker reports an error if the docker daemon is unavailable.
func (s NetworkService) EnsureDocker() error { return docker.EnsureDocker() }

// ResolveNetwork returns the real docker network name created for workspace.
// The network is the one carrying both the CLI managed label and docker
// compose's project label for the workspace; an error is returned when none
// exists (the workspace has not been brought up yet).
func (s NetworkService) ResolveNetwork(workspace string) (string, error) {
	for _, projectLabel := range composeProjectLabels {
		names, err := s.managedNetworksOfProject(projectLabel, workspace)
		if err != nil {
			return "", err
		}
		if len(names) > 0 {
			return pickWorkspaceNetwork(workspace, names), nil
		}
	}
	return "", fmt.Errorf("no managed network found for workspace %q; bring it up first with 'devcontainer-cli up'", workspace)
}

func (s NetworkService) managedNetworksOfProject(projectLabel, workspace string) ([]string, error) {
	status, stdout, _, err := docker.DockerCapture([]string{
		"network", "ls",
		"--filter", managedFilter,
		"--filter", "label=" + projectLabel + "=" + workspace,
		"--format", "{{.Name}}",
	})
	if err != nil {
		return nil, err
	}
	if status != 0 {
		return nil, fmt.Errorf("%s network ls failed", docker.CurrentEngine().Name)
	}
	return splitNonEmptyLines(stdout), nil
}

// pickWorkspaceNetwork prefers the conventional "<workspace>-network" compose
// key when more than one managed network claims the project.
func pickWorkspaceNetwork(workspace string, names []string) string {
	want := workspace + "-network"
	for _, name := range names {
		if strings.HasSuffix(name, want) {
			return name
		}
	}
	return names[0]
}

// Connect attaches each container to the workspace network, registering the
// given network aliases (extra DNS names) on each. It returns the number
// connected and the number that failed.
func (s NetworkService) Connect(workspace string, containers, aliases []string) (connected, failed int, err error) {
	return s.apply("connect", workspace, containers, aliases)
}

// Disconnect detaches each container from the workspace network. It returns the
// number disconnected and the number that failed.
func (s NetworkService) Disconnect(workspace string, containers []string) (disconnected, failed int, err error) {
	return s.apply("disconnect", workspace, containers, nil)
}

// apply resolves the workspace network and runs `docker network <verb>` for each
// container, reporting per-container outcomes and tallying success/failure. For
// "connect", aliases are passed as --alias flags (ignored by "disconnect").
func (s NetworkService) apply(verb, workspace string, containers, aliases []string) (ok, failed int, err error) {
	network, err := s.ResolveNetwork(workspace)
	if err != nil {
		return 0, 0, err
	}
	s.Report.Warn("\nRunning 'docker network %s' on '%s'...", verb, network)
	for _, c := range containers {
		args := []string{"network", verb}
		for _, a := range aliases {
			args = append(args, "--alias", a)
		}
		args = append(args, network, c)
		status, derr := docker.DockerInherit(args)
		if derr == nil && status == 0 {
			s.Report.Success("  ✓ %s", c)
			ok++
			continue
		}
		s.Report.Error("  ✗ %s", c)
		failed++
	}
	return ok, failed, nil
}

// ContainerNames returns every container name on the daemon (running and
// stopped), used to complete arbitrary connect/disconnect targets.
func (s NetworkService) ContainerNames() []string {
	status, stdout, _, err := docker.DockerCapture([]string{"ps", "-a", "--format", "{{.Names}}"})
	if err != nil || status != 0 {
		return nil
	}
	return splitNonEmptyLines(stdout)
}

// splitNonEmptyLines splits s on newlines, trimming each line and dropping the
// empty ones.
func splitNonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if v := strings.TrimSpace(line); v != "" {
			out = append(out, v)
		}
	}
	return out
}

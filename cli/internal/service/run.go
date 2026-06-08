package service

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// RunService spins up a container from a remote image without project files.
// The cli resolves the variant/image and prints the summary and next steps;
// this service owns the docker run/start orchestration.
type RunService struct {
	Report Reporter
}

// QuickRunSpec fully describes a quick-run container to create or resume.
type QuickRunSpec struct {
	Variant       string
	ContainerName string
	Image         string
	Volumes       []string // docker volume specs: name:/path or /host:/container
	Ports         []string // docker port specs: hostport:containerport
	// ExposeAll publishes ports on all interfaces (0.0.0.0). When false (the
	// default), specs without an explicit host IP are bound to 127.0.0.1 so the
	// port is not reachable from the local network.
	ExposeAll bool
}

// Run reuses the container if it already exists (starting it when stopped),
// otherwise creates it from the image. It reports progress and returns once the
// container is running.
func (s RunService) Run(spec QuickRunSpec) error {
	switch s.containerState(spec.ContainerName) {
	case StateRunning:
		s.Report.Success("✓ Container '%s' is already running.", spec.ContainerName)
		return nil
	case StateExited, StateCreated, StatePaused:
		s.Report.Warn("↻ Starting existing container '%s'...", spec.ContainerName)
		status, err := docker.DockerInherit([]string{"start", spec.ContainerName})
		if err != nil {
			return err
		}
		if status != 0 {
			return fmt.Errorf("docker start failed")
		}
		return nil
	}

	args := []string{
		"run", "-d",
		"--name", spec.ContainerName,
		"--restart", "unless-stopped",
		"--label", types.LabelManaged + "=true",
		"--label", types.LabelQuickRun + "=" + spec.Variant,
	}
	for _, v := range spec.Volumes {
		args = append(args, "-v", v)
	}
	for _, p := range spec.Ports {
		if !spec.ExposeAll {
			p = bindLoopback(p)
		}
		args = append(args, "-p", p)
	}
	args = append(args, spec.Image, "sleep", "infinity")

	s.Report.Warn("Pulling and starting container...")
	status, err := docker.DockerInherit(args)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("docker run failed")
	}
	return nil
}

// bindLoopback prefixes a docker `-p` port spec with 127.0.0.1 so the published
// port is only reachable from the local machine, never the LAN. A spec that
// already carries an explicit host IP (two or more colons, e.g.
// "0.0.0.0:8080:80" or "127.0.0.1::80") is returned unchanged. The optional
// protocol suffix ("/tcp", "/udp") adds no colon, so counting colons is safe.
//
//	"8080:80"     -> "127.0.0.1:8080:80"   (host:container)
//	"80"          -> "127.0.0.1::80"       (container only, ephemeral host port)
//	"0.0.0.0:..." -> unchanged             (user-supplied IP is respected)
func bindLoopback(spec string) string {
	switch strings.Count(spec, ":") {
	case 0:
		return "127.0.0.1::" + spec
	case 1:
		return "127.0.0.1:" + spec
	default:
		return spec
	}
}

func (s RunService) containerState(name string) ContainerState {
	inspectSvc := InspectService{Report: s.Report}
	state, err := inspectSvc.ContainerState(name)
	if err != nil {
		return ""
	}
	return state
}

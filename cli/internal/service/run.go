package service

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
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
	// SharedConfig mounts the global shared tool-config volume
	// (devcontainer-shared-config) so logins/sessions persist across containers.
	SharedConfig bool
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
	if spec.SharedConfig {
		// Best-effort: ensure the volume carries the managed labels. Even if this
		// fails, `docker run -v` will create it (unlabeled) so the mount still works.
		if err := EnsureSharedConfigVolume(s.Report); err != nil {
			s.Report.Warn("Could not ensure shared-config volume: %v", err)
		}
		args = append(args, "-v", types.SharedConfigMount())
	}
	for _, v := range spec.Volumes {
		args = append(args, "-v", v)
	}
	for _, p := range spec.Ports {
		if !spec.ExposeAll {
			p = domain.BindLoopback(p)
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

// CopyAIScripts copies the named AI/dev tool installer scripts into the running
// container's devuser home, reusing InspectService.CopyAsset (which materializes
// the embedded script, docker cp's it and leaves it owned by devuser and
// executable). Names are the copyable asset ids (see assets.CopyableNames).
func (s RunService) CopyAIScripts(container string, names []string) error {
	if len(names) == 0 {
		return nil
	}
	inspectSvc := InspectService{Report: s.Report}
	for _, name := range names {
		s.Report.Info("Copying %s...", name)
		if err := inspectSvc.CopyAsset(container, name, ""); err != nil {
			return fmt.Errorf("copy asset %q: %w", name, err)
		}
	}
	s.Report.Success("✓ Copied %d AI tool script(s) into '%s'.", len(names), container)
	return nil
}

func (s RunService) containerState(name string) ContainerState {
	inspectSvc := InspectService{Report: s.Report}
	state, err := inspectSvc.ContainerState(name)
	if err != nil {
		return ""
	}
	return state
}

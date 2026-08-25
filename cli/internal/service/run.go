package service

import (
	"fmt"
	"strings"

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
// otherwise creates it from the image. If a container with the same name
// already exists but was not created by 'run' (no quick-run label), it
// refuses to touch it and returns an error instead of hijacking or renaming
// an unrelated container. It reports progress and returns once the container
// is running.
func (s RunService) Run(spec QuickRunSpec) error {
	lookup := s.lookupQuickRunContainer(spec.ContainerName)
	if lookup.Exists {
		if !lookup.QuickRun {
			return nameConflictErr(spec.ContainerName)
		}
		switch lookup.State {
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
	}

	args := []string{
		"run", "-d",
		"--name", spec.ContainerName,
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

// ValidateContainerName reports an error if name is already used by a
// container that 'run' cannot safely reuse (i.e. one not created by a
// previous 'run' invocation). A free name, or one belonging to an existing
// quick-run container, returns nil. Intended for interactive prompt
// validation, so the user is asked again rather than failing after the image
// has already been pulled.
func (s RunService) ValidateContainerName(name string) error {
	lookup := s.lookupQuickRunContainer(name)
	if lookup.Exists && !lookup.QuickRun {
		return nameConflictErr(name)
	}
	return nil
}

func nameConflictErr(name string) error {
	return fmt.Errorf("container name '%s' is already in use by an existing container that was not created by 'run'; pick a different name (or remove/rename the existing container) and try again", name)
}

// quickRunLookup describes what's known about an existing container name:
// whether it exists, its execution state, and whether it carries the
// quick-run label (i.e. was created by a previous 'run' invocation and is
// therefore safe to reuse/restart).
type quickRunLookup struct {
	Exists   bool
	State    ContainerState
	QuickRun bool
}

// lookupQuickRunContainer inspects name in a single docker call, returning
// both its state and whether it carries the quick-run label. A non-existent
// container yields a zero-value (Exists: false) result.
func (s RunService) lookupQuickRunContainer(name string) quickRunLookup {
	format := `{{.State.Status}}|{{index .Config.Labels "` + types.LabelQuickRun + `"}}`
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", format, name})
	if err != nil || status != 0 {
		return quickRunLookup{}
	}
	state, label, found := strings.Cut(strings.TrimSpace(stdout), "|")
	if state == "" {
		return quickRunLookup{}
	}
	return quickRunLookup{Exists: true, State: ContainerState(state), QuickRun: found && label != ""}
}

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
	Volume        string
	Port          int
}

// Run reuses the container if it already exists (starting it when stopped),
// otherwise creates it from the image. It reports progress and returns once the
// container is running.
func (s RunService) Run(spec QuickRunSpec) error {
	switch s.containerState(spec.ContainerName) {
	case "running":
		s.Report.Success("✓ Container '%s' is already running.", spec.ContainerName)
		return nil
	case "exited", "created", "paused":
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
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"--label", types.LabelManaged + "=true",
		"--label", types.LabelQuickRun + "=" + spec.Variant,
	}
	if spec.Volume != "" {
		_, _ = docker.DockerInherit([]string{"volume", "create", spec.Volume})
		args = append(args, "-v", spec.Volume+":/workspace")
	}
	if spec.Port != 0 {
		args = append(args, "-p", fmt.Sprintf("%d:22", spec.Port))
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

func (s RunService) containerState(name string) string {
	_, stdout, _, _ := docker.DockerCapture([]string{"inspect", "-f", "{{.State.Status}}", name})
	return strings.TrimSpace(strings.ReplaceAll(stdout, " ", ""))
}

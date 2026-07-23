package service

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// LifecycleService runs docker compose lifecycle verbs and container-targeted
// start/stop/remove operations for a project. The cli resolves the compose
// file, workspace and flags; this service owns the docker calls and progress.
type LifecycleService struct {
	Report Reporter
}

// Compose runs "docker compose <verb>" (start/stop/restart) for the project.
func (s LifecycleService) Compose(composeFile, verb string) error {
	s.Report.Warn("\nRunning 'docker compose %s'...", verb)
	return docker.DockerComposeOrThrow(composeFile, []string{verb}, nil)
}

// Up brings the stack up detached, optionally building images first.
func (s LifecycleService) Up(composeFile, workspace string, build bool) error {
	args := []string{"up", "-d"}
	if build {
		args = append(args, "--build")
	}
	s.Report.Warn("\nBringing up '%s'...", workspace)
	return docker.DockerComposeOrThrow(composeFile, args, nil)
}

// Down brings the stack down, optionally removing named volumes (deletes data).
func (s LifecycleService) Down(composeFile, workspace string, removeVolumes bool) error {
	args := []string{"down"}
	suffix := ""
	if removeVolumes {
		args = append(args, "-v")
		suffix = " (with volumes)"
	}
	s.Report.Warn("\nBringing down '%s'%s...", workspace, suffix)
	return docker.DockerComposeOrThrow(composeFile, args, nil)
}

// StartContainer starts a single named container.
func (s LifecycleService) StartContainer(name string) error {
	s.Report.Warn("\nStarting container '%s'...", name)
	status, err := docker.DockerInherit([]string{"start", name})
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("docker start failed")
	}
	return nil
}

// StopContainer stops a single named container.
func (s LifecycleService) StopContainer(name string) error {
	s.Report.Warn("\nStopping container '%s'...", name)
	status, err := docker.DockerInherit([]string{"stop", name})
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("docker stop failed")
	}
	return nil
}

// RestartContainer restarts a single named container.
func (s LifecycleService) RestartContainer(name string) error {
	s.Report.Warn("\nRestarting container '%s'...", name)
	status, err := docker.DockerInherit([]string{"restart", name})
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("docker restart failed")
	}
	return nil
}

// RemoveContainer stops then removes a single named container.
func (s LifecycleService) RemoveContainer(name string) error {
	s.Report.Warn("\nStopping and removing container '%s'...", name)
	_, _ = docker.DockerInherit([]string{"stop", name})
	status, err := docker.DockerInherit([]string{"rm", name})
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("docker rm failed")
	}
	return nil
}

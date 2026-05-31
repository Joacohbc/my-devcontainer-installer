package service

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// dockerCapture adapts the infra docker capture (which also returns an error)
// to the domain.CaptureFunc signature used by docker-querying domain helpers.
func dockerCapture(args []string) (int, string, string) {
	status, stdout, stderr, err := docker.DockerCapture(args)
	if err != nil {
		return 1, "", err.Error()
	}
	return status, stdout, stderr
}

// captureFunc returns the shared CaptureFunc closure for domain calls.
func captureFunc() domain.CaptureFunc {
	return dockerCapture
}

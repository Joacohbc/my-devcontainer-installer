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

// WithHostOverride runs fn with every docker call it makes (through any
// service) redirected to the daemon reachable through host — an existing
// "user@host" or ssh-config alias, via docker.SetHostOverride. This is what
// lets --via reach a container that lives on a different Docker host: 'ssh
// --via'/'setup-ssh --via' redirect the key install and host-key read/pin,
// 'shell --via' redirects the docker exec directly (no ssh-into-container
// step of its own). host == "" runs fn unchanged. Exported at package level,
// not as a method on one service, because cli/commands must never import
// infra/docker directly (see the layering rule in CLAUDE.md) and this same
// plumbing is shared by more than one service.
func WithHostOverride(host string, fn func() error) error {
	if host == "" {
		return fn()
	}
	docker.SetHostOverride(host)
	defer docker.ResetHostOverride()
	return fn()
}

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
// "user@host" or ssh-config alias, via docker.SetHostOverride — or unchanged
// when host == "". Exported at package level rather than as a method on one
// service because cli/commands must never import infra/docker directly (see
// the layering rule in AGENTS.md), and this plumbing is shared by more than
// one service.
func WithHostOverride(host string, fn func() error) error {
	if host == "" {
		return fn()
	}
	docker.SetHostOverride(host)
	defer docker.ResetHostOverride()
	return fn()
}

// WithRemoteCompletion runs fn inside a shell-completion scope against host
// (or the local daemon when host == ""), enabling short timeouts, disk caching,
// and non-interactive SSH flags in infra/docker.
func WithRemoteCompletion(host string, fn func() error) error {
	return docker.WithRemoteCompletion(host, fn)
}

const maxPrewarmItems = 4

func prewarmUpTo(items []string, fn func(string)) {
	count := 0
	for _, item := range items {
		if item == "" {
			continue
		}
		fn(item)
		count++
		if count >= maxPrewarmItems {
			return
		}
	}
}

// PrewarmRemoteContainers triggers a non-blocking background cache refresh
// for up to maxPrewarmItems SSH hosts whose container list cache is missing or stale.
func PrewarmRemoteContainers(hosts ...string) {
	args := []string{"ps", "-a", "--format", "{{json .}}"}
	prewarmUpTo(hosts, func(h string) {
		docker.PrewarmRemoteCompletion(h, args)
	})
}

// PrewarmRemoteContainerUsers triggers a non-blocking background cache refresh
// for /etc/passwd user listings on up to maxPrewarmItems containers on host
// when their cache entry is missing or stale.
func PrewarmRemoteContainerUsers(host string, containers ...string) {
	if host == "" {
		return
	}
	prewarmUpTo(containers, func(name string) {
		docker.PrewarmRemoteCompletion(host, listUsersDockerArgs(name))
	})
}

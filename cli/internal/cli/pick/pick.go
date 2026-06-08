// Package pick lists Docker containers managed by the CLI. The interactive
// selection over these lists lives in the commands layer (pickContainer), so the
// chosen format is uniform across commands.
package pick

import (
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

type Container = service.Container

// ContainerWorkspace returns the workspace prefix of a devcontainer-ssh
// container name, or "" if the name does not carry the service suffix.
func ContainerWorkspace(containerName string) string {
	suffix := "-" + sshdefaults.ServiceName
	if strings.HasSuffix(containerName, suffix) {
		return strings.TrimSuffix(containerName, suffix)
	}
	return ""
}

// ListManaged returns every container carrying the CLI managed label (running
// and stopped). Docker errors yield an empty list.
func ListManaged() []Container {
	svc := service.InspectService{Report: ui.Console{}}
	return svc.ListManaged()
}

// ListAll returns every container regardless of label (running and stopped),
// flagging which are CLI-managed devcontainers.
func ListAll() []Container {
	svc := service.InspectService{Report: ui.Console{}}
	return svc.ListAll()
}

func StatusLabel(c Container) string {
	text := c.Status
	if text == "" {
		text = c.State
	}
	switch c.State {
	case "running":
		return ui.GreenS("%s", text)
	case "exited":
		return ui.RedS("%s", text)
	default:
		return ui.YellowS("%s", text)
	}
}

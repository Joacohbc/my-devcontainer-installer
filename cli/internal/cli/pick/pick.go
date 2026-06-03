// Package pick lists Docker containers managed by the CLI and lets
// the user pick one interactively. Used by setup-ssh and port-forward.
package pick

import (
	"fmt"
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

// ListManaged returns all containers carrying the CLI managed label. Docker
// errors yield an empty list.
func ListManaged() []Container {
	svc := service.InspectService{Report: ui.Console{}}
	return svc.ListManaged()
}

// ListAll returns running containers regardless of label, flagging which are
// CLI-managed devcontainers.
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

type PickOptions struct {
	Interactive bool
	AssumeYes   bool
}

// PickManaged returns the single managed container, or prompts to choose one.
func PickManaged(message string, opts PickOptions) (Container, error) {
	containers := ListManaged()
	if len(containers) == 0 {
		return Container{}, fmt.Errorf("no devcontainer-cli managed containers found. Run 'devcontainer-cli' first to create one")
	}
	if len(containers) == 1 || opts.AssumeYes {
		c := containers[0]
		fmt.Println(ui.CyanS("Using container: %s  %s  %s", c.Name, ui.Subtle(c.Image), StatusLabel(c)))
		return c, nil
	}
	if !opts.Interactive {
		return Container{}, fmt.Errorf("multiple CLI-managed containers found. Specify a container explicitly or run interactively")
	}
	choices := make([]service.Option, len(containers))
	for i, c := range containers {
		choices[i] = service.Option{Value: c.Name, Label: fmt.Sprintf("%s  %s  %s", c.Name, ui.Subtle(c.Image), StatusLabel(c))}
	}
	chosen, err := ui.Select(message, choices, choices[0])
	if err != nil {
		return Container{}, err
	}
	for _, c := range containers {
		if c.Name == chosen.Value {
			return c, nil
		}
	}
	return containers[0], nil
}

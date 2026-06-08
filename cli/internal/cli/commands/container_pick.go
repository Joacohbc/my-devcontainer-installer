package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/pick"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

// pickContainerOptions controls how pickContainer resolves a choice when the
// list has more than one entry.
type pickContainerOptions struct {
	Interactive bool
	AssumeYes   bool
}

// workspaceTag renders the workspace/standalone column for a container, using a
// subtle dash for unmanaged containers that have neither.
func workspaceTag(c pick.Container) string {
	if c.Workspace == "" {
		return console.Subtle("-")
	}
	return c.Workspace
}

// containerChoiceLabel is the single, uniform one-line rendering of a container
// used everywhere a container is listed or selected: name, workspace/standalone,
// image and status.
func containerChoiceLabel(c pick.Container) string {
	return fmt.Sprintf("%s  %s  %s  %s", c.Name, workspaceTag(c), console.Subtle(c.Image), pick.StatusLabel(c))
}

// pickContainer renders a uniform selection prompt over containers and returns
// the chosen one. With a single container (or AssumeYes) it auto-selects and
// reports the choice; in non-interactive mode with several candidates it errors.
func pickContainer(containers []pick.Container, message string, opts pickContainerOptions) (pick.Container, error) {
	if len(containers) == 0 {
		return pick.Container{}, fmt.Errorf("no containers found")
	}
	if len(containers) == 1 || opts.AssumeYes {
		c := containers[0]
		console.Info("Using container: %s", containerChoiceLabel(c))
		return c, nil
	}
	if !opts.Interactive {
		return pick.Container{}, fmt.Errorf("multiple containers found. Specify one explicitly or run interactively")
	}
	choices := make([]service.Option, len(containers))
	for i, c := range containers {
		choices[i] = service.Option{Value: c.Name, Label: containerChoiceLabel(c)}
	}
	chosen, err := console.Select(message, choices, choices[0])
	if err != nil {
		return pick.Container{}, err
	}
	for _, c := range containers {
		if c.Name == chosen.Value {
			return c, nil
		}
	}
	return containers[0], nil
}

// pickManagedContainer lists CLI-managed containers and prompts to choose one,
// erroring when none exist.
func pickManagedContainer(message string, opts pickContainerOptions) (pick.Container, error) {
	containers := pick.ListManaged()
	if len(containers) == 0 {
		return pick.Container{}, fmt.Errorf("no devcontainer-cli managed containers found. Run 'devcontainer-cli' first to create one")
	}
	return pickContainer(containers, message, opts)
}

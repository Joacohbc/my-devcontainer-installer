// Package containerpicker lists Docker containers managed by the CLI and lets
// the user pick one interactively. Used by setup-ssh and port-forward.
package containerpicker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/core"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/prompt"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

type Container struct {
	Name    string
	Image   string
	Status  string
	State   string
	Managed bool
}

// ContainerWorkspace returns the workspace prefix of a devcontainer-ssh
// container name, or "" if the name does not carry the service suffix.
func ContainerWorkspace(containerName string) string {
	suffix := "-" + sshdefaults.ServiceName
	if strings.HasSuffix(containerName, suffix) {
		return strings.TrimSuffix(containerName, suffix)
	}
	return ""
}

type dockerPSLine struct {
	Names  string `json:"Names"`
	Image  string `json:"Image"`
	Status string `json:"Status"`
	State  string `json:"State"`
	Labels string `json:"Labels"`
}

func parsePSLines(stdout string) []dockerPSLine {
	var out []dockerPSLine
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var obj dockerPSLine
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if obj.Names == "" {
			continue
		}
		out = append(out, obj)
	}
	return out
}

// ListManaged returns all containers carrying the CLI managed label. Docker
// errors yield an empty list.
func ListManaged() []Container {
	status, stdout, _, err := docker.DockerCapture([]string{
		"ps", "-a", "--filter", "label=" + core.LabelManaged + "=true", "--format", "{{json .}}",
	})
	if err != nil || status != 0 || strings.TrimSpace(stdout) == "" {
		return nil
	}
	var out []Container
	for _, l := range parsePSLines(stdout) {
		out = append(out, Container{Name: l.Names, Image: l.Image, Status: l.Status, State: l.State, Managed: true})
	}
	return out
}

// ListAll returns running containers regardless of label, flagging which are
// CLI-managed devcontainers.
func ListAll() []Container {
	status, stdout, _, err := docker.DockerCapture([]string{"ps", "--format", "{{json .}}"})
	if err != nil || status != 0 || strings.TrimSpace(stdout) == "" {
		return nil
	}
	managedLabel := core.LabelManaged + "=true"
	var out []Container
	for _, l := range parsePSLines(stdout) {
		managed := false
		for _, lab := range strings.Split(l.Labels, ",") {
			if strings.TrimSpace(lab) == managedLabel {
				managed = true
				break
			}
		}
		out = append(out, Container{Name: l.Names, Image: l.Image, Status: l.Status, State: l.State, Managed: managed})
	}
	return out
}

func StatusLabel(c Container) string {
	text := c.Status
	if text == "" {
		text = c.State
	}
	switch c.State {
	case "running":
		return color.GreenString(text)
	case "exited":
		return color.RedString(text)
	default:
		return color.YellowString(text)
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
		fmt.Println(color.CyanString("Using container: %s  %s  %s", c.Name, color.WhiteString(c.Image), StatusLabel(c)))
		return c, nil
	}
	if !opts.Interactive {
		return Container{}, fmt.Errorf("multiple CLI-managed containers found. Specify a container explicitly or run interactively")
	}
	choices := make([]prompt.Choice, len(containers))
	for i, c := range containers {
		choices[i] = prompt.Choice{Value: c.Name, Label: fmt.Sprintf("%s  %s  %s", c.Name, color.WhiteString(c.Image), StatusLabel(c))}
	}
	chosen, err := prompt.Select(message, choices, choices[0].Value)
	if err != nil {
		return Container{}, err
	}
	for _, c := range containers {
		if c.Name == chosen {
			return c, nil
		}
	}
	return containers[0], nil
}

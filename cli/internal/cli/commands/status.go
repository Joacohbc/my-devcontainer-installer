package commands

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/pick"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newStatusCommand()) }

func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show a status table for the project's containers",
		Long: `devcontainer-cli status — print a compact table of the current project's
containers with their service, name, state, published ports and image (similar
to 'docker compose ps'). Services that exist in the compose file but have no
container yet are shown as "not created".

For an exhaustive per-container report use 'info' instead. --container narrows to
one container; --all lists every managed container across all workspaces.`,
		Example: `  # Status of the current project
  devcontainer-cli status

  # Every managed container on the machine
  devcontainer-cli status --all`,
		SilenceUsage: true,
		RunE:         runStatus,
	}
	addContainerFlag(cmd)
	cmd.Flags().Bool("all", false, "Show all CLI-managed containers across all workspaces")
	return cmd
}

func runStatus(cmd *cobra.Command, _ []string) error {
	svc := service.InspectService{Report: console}
	if err := svc.EnsureDocker(); err != nil {
		return err
	}

	containerMap := make(map[string]pick.Container)
	for _, c := range pick.ListAll() {
		containerMap[c.Name] = c
	}

	if cmd.Flags().Changed("container") {
		return statusForContainer(cmd, svc, containerMap)
	}
	if allFlag, _ := cmd.Flags().GetBool("all"); allFlag {
		return statusForAllManaged()
	}
	return statusForProject(containerMap)
}

func newStatusTable(headers ...string) *table.Table {
	return table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorSubtle)).
		Headers(headers...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return ui.StyleBold.Foreground(ui.ColorPrimary)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})
}

func statusForContainer(cmd *cobra.Command, svc service.InspectService, containerMap map[string]pick.Container) error {
	containerName, err := resolveContainer(cmd)
	if err != nil {
		return err
	}

	t := newStatusTable("CONTAINER NAME", "STATUS", "PORTS", "IMAGE")
	if c, found := containerMap[containerName]; found {
		t.Row(c.Name, pick.StatusLabel(c), c.Ports, c.Image)
	} else if state, serr := svc.ContainerState(containerName); serr == nil {
		t.Row(containerName, console.ErrorS("%s", state), "", svc.ContainerImage(containerName))
	} else {
		t.Row(containerName, console.ErrorS("not found"), "", "")
	}

	console.NewLine()
	console.Print(t.String())
	console.NewLine()
	return nil
}

func statusForAllManaged() error {
	managed := pick.ListManaged()

	console.NewLine()
	console.Header("All managed containers")
	console.NewLine()

	if len(managed) == 0 {
		console.Warn("No managed containers found.")
		console.NewLine()
		return nil
	}

	t := newStatusTable("CONTAINER NAME", "WORKSPACE", "STATUS", "PORTS", "IMAGE")
	for _, c := range managed {
		t.Row(c.Name, workspaceTag(c), pick.StatusLabel(c), c.Ports, c.Image)
	}
	console.Print(t.String())
	console.NewLine()
	return nil
}

func statusForProject(containerMap map[string]pick.Container) error {
	cwd, err := currentDir()
	if err != nil {
		return err
	}
	workspace := resolveWorkspace(cwd)

	paths := project.ProjectPaths(cwd, workspace)
	services := service.ReadComposeServices(paths.ComposeFile)
	if services == nil {
		return fmt.Errorf("no active project compose file found at %s. Run 'devcontainer-cli' to generate one first, or target a specific container via --container", paths.ComposeFile)
	}

	console.NewLine()
	console.Header("Workspace: %s", workspace)
	console.NewLine()

	t := newStatusTable("SERVICE", "CONTAINER NAME", "STATUS", "PORTS", "IMAGE")
	for serviceKey, svc := range services {
		expectedName := statusContainerName(workspace, svc.ContainerName, serviceKey)

		statusText := console.ErrorS("not created")
		portsText := ""
		imageText := ""
		if c, found := containerMap[expectedName]; found {
			statusText = pick.StatusLabel(c)
			portsText = c.Ports
			imageText = c.Image
		}

		t.Row(serviceKey, expectedName, statusText, portsText, imageText)
	}

	console.Print(t.String())
	console.NewLine()
	return nil
}

func statusContainerName(workspace, base, serviceKey string) string {
	if base == "" {
		base = serviceKey
	}
	if base == workspace || strings.HasPrefix(base, workspace+"-") {
		return base
	}
	return workspace + "-" + base
}

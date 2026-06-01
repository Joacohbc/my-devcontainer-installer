package commands

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/pick"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newStatusCommand()) }

func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Show container status for the active project",
		Long:         `devcontainer-cli status — list containers, states, and port mappings for the active workspace`,
		SilenceUsage: true,
		RunE:         runStatus,
	}
	addWorkspaceFlag(cmd)
	addContainerFlag(cmd)
	cmd.Flags().Bool("all", false, "Show all CLI-managed containers across all workspaces")
	return cmd
}

func runStatus(cmd *cobra.Command, _ []string) error {
	svc := service.InspectService{Report: ui.Console{}}
	if err := svc.EnsureDocker(); err != nil {
		return err
	}

	wsFlag := workspaceFlag(cmd)

	// Get all containers from Docker (running and stopped)
	allContainers := pick.ListAll()
	containerMap := make(map[string]pick.Container)
	for _, c := range allContainers {
		containerMap[c.Name] = c
	}

	// If explicit container flag is provided, show status for just that container
	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return err
		}

		t := table.New().
			Border(lipgloss.NormalBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorSubtle)).
			Headers("CONTAINER NAME", "STATUS", "PORTS", "IMAGE").
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == 0 {
					return ui.StyleBold.Foreground(ui.ColorPrimary)
				}
				return lipgloss.NewStyle().Padding(0, 1)
			})

		if c, found := containerMap[containerName]; found {
			t.Row(c.Name, pick.StatusLabel(c), c.Ports, c.Image)
		} else if state, serr := svc.ContainerState(containerName); serr == nil {
			t.Row(containerName, ui.RedS("%s", state), "", svc.ContainerImage(containerName))
		} else {
			t.Row(containerName, ui.RedS("not found"), "", "")
		}

		fmt.Println()
		fmt.Println(t)
		fmt.Println()
		return nil
	}

	// --all: show every CLI-managed container across all workspaces
	if allFlag, _ := cmd.Flags().GetBool("all"); allFlag {
		managed := pick.ListManaged()

		fmt.Println()
		ui.Header("All managed containers")
		fmt.Println()

		t := table.New().
			Border(lipgloss.NormalBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorSubtle)).
			Headers("CONTAINER NAME", "STATUS", "PORTS", "IMAGE").
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == 0 {
					return ui.StyleBold.Foreground(ui.ColorPrimary)
				}
				return lipgloss.NewStyle().Padding(0, 1)
			})

		if len(managed) == 0 {
			fmt.Println(ui.YellowS("No managed containers found."))
			fmt.Println()
			return nil
		}
		for _, c := range managed {
			t.Row(c.Name, pick.StatusLabel(c), c.Ports, c.Image)
		}
		fmt.Println(t)
		fmt.Println()
		return nil
	}

	// Default project mode (like docker compose ps)
	cwd, err := currentDir()
	if err != nil {
		return err
	}
	var workspace string
	if wsFlag != "" {
		workspace = wsFlag
	} else {
		cfg, _ := domain.LoadConfig(cwd)
		workspace = domain.ResolveWorkspace(cwd, cfg)
	}

	paths := project.ProjectPaths(cwd, workspace)
	services := readComposeServices(paths.ComposeFile)
	if services == nil {
		return fmt.Errorf("no active project compose file found at %s. Run 'devcontainer-cli' to generate one first, or target a specific container via --container", paths.ComposeFile)
	}

	fmt.Println()
	ui.Header(fmt.Sprintf("Workspace: %s", workspace))
	fmt.Println()

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorSubtle)).
		Headers("SERVICE", "CONTAINER NAME", "STATUS", "PORTS", "IMAGE").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return ui.StyleBold.Foreground(ui.ColorPrimary)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	// Helper to prefix container name per workspace
	prefixContainerLocal := func(workspace, base string) string {
		if base == workspace || strings.HasPrefix(base, workspace+"-") {
			return base
		}
		return workspace + "-" + base
	}

	for serviceKey, svc := range services {
		baseContainer := svc.ContainerName
		if baseContainer == "" {
			baseContainer = serviceKey
		}
		expectedName := prefixContainerLocal(workspace, baseContainer)

		statusText := ui.RedS("not created")
		portsText := ""
		imageText := ""

		if c, found := containerMap[expectedName]; found {
			statusText = pick.StatusLabel(c)
			portsText = c.Ports
			imageText = c.Image
		}

		t.Row(serviceKey, expectedName, statusText, portsText, imageText)
	}

	fmt.Println(t)
	fmt.Println()
	return nil
}

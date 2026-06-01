package commands

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newInfoCommand()) }

func newInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "info",
		Short:        "Show name, image and IP(s) for project containers",
		Long:         `devcontainer-cli info — display container name, image and network IPs for the active workspace`,
		SilenceUsage: true,
		RunE:         runInfo,
	}
	addWorkspaceFlag(cmd)
	addContainerFlag(cmd)
	return cmd
}

func runInfo(cmd *cobra.Command, _ []string) error {
	svc := service.InspectService{Report: ui.Console{}}
	if err := svc.EnsureDocker(); err != nil {
		return err
	}

	wsFlag := workspaceFlag(cmd)

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return err
		}
		return showSingleContainerInfo(svc, containerName)
	}

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

	t := newInfoTable()

	prefixContainer := func(base string) string {
		if base == workspace || strings.HasPrefix(base, workspace+"-") {
			return base
		}
		return workspace + "-" + base
	}

	for serviceKey, svcDef := range services {
		base := svcDef.ContainerName
		if base == "" {
			base = serviceKey
		}
		name := prefixContainer(base)
		t.Row(name, svc.ContainerImage(name), containerIPsText(svc, name))
	}

	fmt.Println(t)
	fmt.Println()
	return nil
}

func showSingleContainerInfo(svc service.InspectService, name string) error {
	t := newInfoTable()
	t.Row(name, svc.ContainerImage(name), containerIPsText(svc, name))
	fmt.Println()
	fmt.Println(t)
	fmt.Println()
	return nil
}

func newInfoTable() *table.Table {
	return table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorSubtle)).
		Headers("CONTAINER NAME", "IMAGE", "IPs").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return ui.StyleBold.Foreground(ui.ColorPrimary)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})
}

func containerIPsText(svc service.InspectService, name string) string {
	ips, err := svc.ContainerNetworkIPs(name)
	if err != nil || len(ips) == 0 {
		return ui.YellowS("-")
	}
	return strings.Join(ips, "\n")
}

package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/pick"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
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
	cmd.Flags().StringP("workspace", "w", "", "Workspace name")
	addContainerFlag(cmd)
	return cmd
}

func runStatus(cmd *cobra.Command, _ []string) error {
	if err := docker.EnsureDocker(); err != nil {
		return err
	}

	wsFlag, _ := cmd.Flags().GetString("workspace")

	// Get all containers from Docker (running and stopped)
	allContainers := pick.ListAll()
	containerMap := make(map[string]pick.Container)
	for _, c := range allContainers {
		containerMap[c.Name] = c
	}

	bold := color.New(color.Bold)

	// If explicit container flag is provided, show status for just that container
	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return err
		}

		bold.Printf("\n%-30s %-20s %-30s %-30s\n", "CONTAINER NAME", "STATUS", "PORTS", "IMAGE")
		fmt.Println(strings.Repeat("─", 115))

		if c, found := containerMap[containerName]; found {
			fmt.Printf("%-30s %-20s %-30s %-30s\n\n", c.Name, pick.StatusLabel(c), c.Ports, c.Image)
		} else {
			// fallback: check if we can inspect it
			status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", "{{.State.Status}}", containerName})
			if err == nil && status == 0 {
				state := strings.TrimSpace(stdout)
				_, imgOut, _, _ := docker.DockerCapture([]string{"inspect", "-f", "{{.Config.Image}}", containerName})
				image := strings.TrimSpace(imgOut)
				fmt.Printf("%-30s %-20s %-30s %-30s\n\n", containerName, color.RedString(state), "", image)
			} else {
				fmt.Printf("%-30s %-20s %-30s %-30s\n\n", containerName, color.RedString("not found"), "", "")
			}
		}
		return nil
	}

	// Default project mode (like docker compose ps)
	cwd, _ := os.Getwd()
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

	bold.Printf("\nWorkspace: %s\n\n", workspace)
	bold.Printf("%-20s %-30s %-20s %-30s %-30s\n", "SERVICE", "CONTAINER NAME", "STATUS", "PORTS", "IMAGE")
	fmt.Println(strings.Repeat("─", 135))

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

		statusText := color.RedString("not created")
		portsText := ""
		imageText := ""

		if c, found := containerMap[expectedName]; found {
			statusText = pick.StatusLabel(c)
			portsText = c.Ports
			imageText = c.Image
		}

		fmt.Printf("%-20s %-30s %-20s %-30s %-30s\n", serviceKey, expectedName, statusText, portsText, imageText)
	}

	fmt.Println()
	return nil
}

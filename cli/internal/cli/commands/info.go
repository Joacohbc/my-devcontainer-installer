package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newInfoCommand()) }

func newInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show detailed info for project containers",
		Long: `devcontainer-cli info — display name, image, status, timestamps, ports, volumes and IPs
for the active workspace, or a specific container via --container.`,
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
		fmt.Println()
		printContainerInfoBlock(svc, containerName)
		fmt.Println()
		return nil
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
		printContainerInfoBlock(svc, prefixContainer(base))
		fmt.Println()
	}

	return nil
}

// printContainerInfoBlock fetches and renders all details for one container.
func printContainerInfoBlock(svc service.InspectService, name string) {
	const keyWidth = 9
	// total prefix width: "  " + padded-key + " : "
	const contWidth = 2 + keyWidth + 3

	keyStr := func(k string) string {
		return ui.Subtle(fmt.Sprintf("  %-*s : ", keyWidth, k))
	}
	cont := strings.Repeat(" ", contWidth)

	fmt.Println(ui.Bold(name))

	info, err := svc.ContainerDetails(name)
	if err != nil {
		fmt.Println(ui.Subtle("  (not found)"))
		return
	}

	fmt.Print(keyStr("Image"))
	fmt.Println(info.Image)

	fmt.Print(keyStr("Status"))
	fmt.Println(infoStatusLabel(info.Status))

	fmt.Print(keyStr("Created"))
	fmt.Println(formatInfoTime(info.Created))

	fmt.Print(keyStr("Started"))
	fmt.Println(formatInfoTime(info.StartedAt))

	printInfoMulti(keyStr("Ports"), cont, info.Ports)
	printInfoMulti(keyStr("Volumes"), cont, info.Volumes)
	printInfoMulti(keyStr("IPs"), cont, info.IPs)
}

// printInfoMulti prints a key followed by multiple values, continuing on new
// lines aligned with the first value. Prints "-" when vals is empty.
func printInfoMulti(keyStr, cont string, vals []string) {
	if len(vals) == 0 {
		fmt.Print(keyStr)
		fmt.Println(ui.Subtle("-"))
		return
	}
	for i, v := range vals {
		if i == 0 {
			fmt.Print(keyStr)
		} else {
			fmt.Print(cont)
		}
		fmt.Println(v)
	}
}

func formatInfoTime(t time.Time) string {
	if t.IsZero() || t.Year() <= 1 {
		return ui.Subtle("-")
	}
	return t.Local().Format("02-01-2006 15:04:05")
}

func infoStatusLabel(status string) string {
	switch status {
	case "running":
		return ui.GreenS("%s", status)
	case "exited":
		return ui.RedS("%s", status)
	case "":
		return ui.YellowS("-")
	default:
		return ui.YellowS("%s", status)
	}
}

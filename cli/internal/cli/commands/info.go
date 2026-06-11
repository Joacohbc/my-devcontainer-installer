package commands

import (
	"fmt"
	"strings"
	"time"

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
	svc := service.InspectService{Report: console}
	if err := svc.EnsureDocker(); err != nil {
		return err
	}

	wsFlag := workspaceFlag(cmd)

	if cmd.Flags().Changed("container") {
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return err
		}
		console.NewLine()
		printContainerInfoBlock(svc, containerName)
		console.NewLine()
		return nil
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}

	workspace := resolveWorkspace(cwd, wsFlag)

	paths := project.ProjectPaths(cwd, workspace)
	services := service.ReadComposeServices(paths.ComposeFile)
	if services == nil {
		return fmt.Errorf("no active project compose file found at %s. Run 'devcontainer-cli' to generate one first, or target a specific container via --container", paths.ComposeFile)
	}

	console.NewLine()
	console.Header("Workspace: %s", workspace)
	console.NewLine()

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
		console.NewLine()
	}

	return nil
}

// printContainerInfoBlock fetches and renders all details for one container.
func printContainerInfoBlock(svc service.InspectService, name string) {
	const keyWidth = 9
	// total prefix width: "  " + padded-key + " : "
	const contWidth = 2 + keyWidth + 3

	keyStr := func(k string) string {
		return console.Subtle(fmt.Sprintf("  %-*s : ", keyWidth, k))
	}
	cont := strings.Repeat(" ", contWidth)

	console.Header("%s", name)

	info, err := svc.ContainerDetails(name)
	if err != nil {
		console.Warn("  (not found)")
		return
	}

	console.Print(keyStr("Image") + info.Image + "\n")
	console.Print(keyStr("Status") + infoStatusLabel(info.Status) + "\n")
	console.Print(keyStr("Created") + formatInfoTime(info.Created) + "\n")
	console.Print(keyStr("Started") + formatInfoTime(info.StartedAt) + "\n")

	var ports []string
	for _, p := range info.Ports {
		ports = append(ports, fmt.Sprintf("%s:%s/%s", p.HostPort, p.ContainerPort, p.Protocol))
	}
	printInfoMulti(keyStr("Ports"), cont, ports)

	var vols []string
	for _, v := range info.Volumes {
		vols = append(vols, v.Source+" → "+v.Destination)
	}
	printInfoMulti(keyStr("Volumes"), cont, vols)

	var ips []string
	for _, ip := range info.IPs {
		line := ip.Network + " " + ip.IP
		if len(ip.Aliases) > 0 {
			line += console.Subtle(" (aliases: " + strings.Join(ip.Aliases, ", ") + ")")
		}
		ips = append(ips, line)
	}
	printInfoMulti(keyStr("IPs"), cont, ips)
}

// printInfoMulti prints a key followed by multiple values, continuing on new
// lines aligned with the first value. Prints "-" when vals is empty.
func printInfoMulti(keyStr, cont string, vals []string) {
	if len(vals) == 0 {
		console.Print(keyStr + console.Subtle("-") + "\n")
		return
	}
	for i, v := range vals {
		if i == 0 {
			console.Print(keyStr + v + "\n")
		} else {
			console.Print(cont + v + "\n")
		}
	}
}

func formatInfoTime(t time.Time) string {
	if t.IsZero() || t.Year() <= 1 {
		return console.Subtle("-")
	}
	return t.Local().Format("02-01-2006 15:04:05")
}

func infoStatusLabel(status service.ContainerState) string {
	switch status {
	case service.StateRunning:
		return console.SuccessS("%s", status)
	case service.StateExited:
		return console.ErrorS("%s", status)
	case "":
		return console.WarnS("-")
	default:
		return console.WarnS("%s", status)
	}
}

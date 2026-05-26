package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/containerpicker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/prompt"
	"github.com/spf13/cobra"
)

func init() { register(newPortForwardCommand()) }

func newPortForwardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port-forward [port_mapping]",
		Short: "Forward host port to a container port using SSH",
		Long: `devcontainer-cli port-forward — forward host ports to container ports using SSH

Run with no port_mapping for an interactive session: pick any running container
(devcontainers are marked '(devcontainer)'), enter one or more ports, and repeat
for other containers. All tunnels are opened in parallel after you confirm.

Examples:
  devcontainer-cli port-forward                 # interactive multi-container picker
  devcontainer-cli port-forward 3000
  devcontainer-cli port-forward 8080:80
  devcontainer-cli port-forward 5432:postgres:5432
  devcontainer-cli port-forward 5432 --service postgres
  devcontainer-cli port-forward 3000 --alias my-custom-host`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE:         runPortForward,
	}
	cmd.Flags().String("alias", "", "SSH host alias to use (bypasses auto-discovery)")
	cmd.Flags().String("service", "", "Compose service to map port to (default: localhost)")
	cmd.Flags().Bool("no-interactive", false, "Disable interactive prompts (fail on missing config)")
	cmd.Flags().Bool("non-interactive", false, "Disable interactive prompts (fail on missing config)")
	return cmd
}

type parsedMapping struct {
	localPort     int
	targetHost    string
	containerPort int
}

type portPair struct {
	localPort     int
	containerPort int
}

type plannedTunnel struct {
	localPort      int
	containerPort  int
	targetHost     string
	alias          string
	containerName  string
	isDevcontainer bool
}

func parsePort(value, label string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 || n > 65535 {
		return 0, fmt.Errorf("invalid %s: %s", label, value)
	}
	return n, nil
}

func parsePortMapping(mapping, defaultService string) (parsedMapping, error) {
	parts := strings.Split(mapping, ":")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	switch len(parts) {
	case 1:
		port, err := parsePort(parts[0], "port")
		if err != nil {
			return parsedMapping{}, err
		}
		host := defaultService
		if host == "" {
			host = "localhost"
		}
		return parsedMapping{localPort: port, targetHost: host, containerPort: port}, nil
	case 2:
		local, err := parsePort(parts[0], "local port")
		if err != nil {
			return parsedMapping{}, err
		}
		container, err := parsePort(parts[1], "container port")
		if err != nil {
			return parsedMapping{}, err
		}
		host := defaultService
		if host == "" {
			host = "localhost"
		}
		return parsedMapping{localPort: local, targetHost: host, containerPort: container}, nil
	case 3:
		local, err := parsePort(parts[0], "local port")
		if err != nil {
			return parsedMapping{}, err
		}
		targetHost := parts[1]
		if targetHost == "" {
			return parsedMapping{}, fmt.Errorf("invalid target host: empty string")
		}
		container, err := parsePort(parts[2], "container port")
		if err != nil {
			return parsedMapping{}, err
		}
		if defaultService != "" && defaultService != targetHost {
			return parsedMapping{}, fmt.Errorf("conflicting target hosts: mapping specifies '%s' but --service is '%s'", targetHost, defaultService)
		}
		return parsedMapping{localPort: local, targetHost: targetHost, containerPort: container}, nil
	default:
		return parsedMapping{}, fmt.Errorf("invalid port mapping format: %s", mapping)
	}
}

func parsePortPair(item string) (portPair, error) {
	parts := strings.Split(item, ":")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	switch len(parts) {
	case 1:
		p, err := parsePort(parts[0], "port")
		if err != nil {
			return portPair{}, err
		}
		return portPair{localPort: p, containerPort: p}, nil
	case 2:
		local, err := parsePort(parts[0], "local port")
		if err != nil {
			return portPair{}, err
		}
		container, err := parsePort(parts[1], "container port")
		if err != nil {
			return portPair{}, err
		}
		return portPair{localPort: local, containerPort: container}, nil
	default:
		return portPair{}, fmt.Errorf("invalid port mapping: %s (expected 'port' or 'local:container')", item)
	}
}

func parsePortsList(value string) ([]portPair, error) {
	var pairs []portPair
	for _, item := range strings.Split(value, ",") {
		if t := strings.TrimSpace(item); t != "" {
			p, err := parsePortPair(t)
			if err != nil {
				return nil, err
			}
			pairs = append(pairs, p)
		}
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("no ports specified")
	}
	return pairs, nil
}

var hostLineRe = regexp.MustCompile(`(?i)^[ \t]*host[ \t]+([^#]+)`)

func parseSSHConfigContent(content string) []string {
	var hosts []string
	seen := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		m := hostLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for _, p := range strings.Fields(strings.TrimSpace(m[1])) {
			if p != "" && !strings.Contains(p, "*") && !strings.Contains(p, "?") && !seen[p] {
				seen[p] = true
				hosts = append(hosts, p)
			}
		}
	}
	return hosts
}

func getSSHAliases() []string {
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".ssh", "config")
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	return parseSSHConfigContent(string(data))
}

func ownSSHAlias(containerName string, aliases []string) string {
	candidate := containerpicker.ContainerWorkspace(containerName)
	if candidate == "" {
		candidate = containerName
	}
	for _, a := range aliases {
		if a == candidate {
			return candidate
		}
	}
	return ""
}

func jumpHostAliases(containers []containerpicker.Container, aliases []string) []string {
	var result []string
	seen := map[string]bool{}
	for _, c := range containers {
		if !c.Managed {
			continue
		}
		if a := ownSSHAlias(c.Name, aliases); a != "" && !seen[a] {
			seen[a] = true
			result = append(result, a)
		}
	}
	return result
}

func pickContainerFromList(containers []containerpicker.Container, message string) (containerpicker.Container, error) {
	choices := make([]prompt.Choice, len(containers))
	for i, c := range containers {
		tag := "              "
		if c.Managed {
			tag = color.GreenString("(devcontainer)")
		}
		choices[i] = prompt.Choice{Value: c.Name, Label: fmt.Sprintf("%s  %s  %s  %s", c.Name, tag, color.WhiteString(c.Image), containerpicker.StatusLabel(c))}
	}
	chosen, err := prompt.Select(message, choices, choices[0].Value)
	if err != nil {
		return containerpicker.Container{}, err
	}
	for _, c := range containers {
		if c.Name == chosen {
			return c, nil
		}
	}
	return containers[0], nil
}

func resolveTarget(container containerpicker.Container, aliases []string, containers []containerpicker.Container, jumpHost *string, interactive bool) (alias, targetHost string, err error) {
	if container.Managed {
		if own := ownSSHAlias(container.Name, aliases); own != "" {
			return own, "localhost", nil
		}
	}
	if *jumpHost == "" {
		candidates := jumpHostAliases(containers, aliases)
		switch {
		case len(candidates) == 1:
			*jumpHost = candidates[0]
			color.Cyan("Using SSH jump host '%s'.", *jumpHost)
		case len(candidates) > 1:
			if !interactive {
				return "", "", fmt.Errorf("multiple SSH jump hosts available; specify --alias")
			}
			choices := make([]prompt.Choice, len(candidates))
			for i, a := range candidates {
				choices[i] = prompt.Choice{Value: a, Label: a}
			}
			sel, serr := prompt.Select("Select the devcontainer SSH host to tunnel through:", choices, candidates[0])
			if serr != nil {
				return "", "", serr
			}
			*jumpHost = sel
		default:
			if !interactive {
				return "", "", fmt.Errorf("no devcontainer SSH alias found to tunnel through '%s'. Run 'setup-ssh' first or specify --alias", container.Name)
			}
			in, ierr := prompt.Input(fmt.Sprintf("No devcontainer SSH alias found to reach '%s'. Enter SSH alias to tunnel through:", container.Name), "", func(v string) error {
				if strings.TrimSpace(v) == "" {
					return fmt.Errorf("SSH alias cannot be empty")
				}
				return nil
			})
			if ierr != nil {
				return "", "", ierr
			}
			*jumpHost = in
		}
	}
	return *jumpHost, container.Name, nil
}

func buildTunnelsInteractive(flagAlias string, interactive bool) ([]plannedTunnel, error) {
	containers := containerpicker.ListAll()
	if len(containers) == 0 {
		return nil, fmt.Errorf("no running containers found. Start a container with 'devcontainer-cli' first")
	}
	aliases := getSSHAliases()
	var tunnels []plannedTunnel
	jumpHost := flagAlias

	for {
		var container containerpicker.Container
		var err error
		if len(containers) == 1 {
			container = containers[0]
			tag := ""
			if container.Managed {
				tag = " (devcontainer)"
			}
			color.Cyan("Using container: %s%s", container.Name, tag)
		} else {
			container, err = pickContainerFromList(containers, "Select a container to forward from:")
			if err != nil {
				return nil, err
			}
		}

		alias, targetHost, terr := resolveTarget(container, aliases, containers, &jumpHost, interactive)
		if terr != nil {
			return nil, terr
		}

		portsStr, ierr := prompt.Input(fmt.Sprintf("Ports to forward from '%s' (e.g. 3000, 8080:80):", container.Name), "", func(v string) error {
			_, e := parsePortsList(v)
			return e
		})
		if ierr != nil {
			return nil, ierr
		}
		pairs, _ := parsePortsList(portsStr)
		for _, pair := range pairs {
			tunnels = append(tunnels, plannedTunnel{
				localPort:      pair.localPort,
				containerPort:  pair.containerPort,
				targetHost:     targetHost,
				alias:          alias,
				containerName:  container.Name,
				isDevcontainer: container.Managed,
			})
		}

		more, merr := prompt.Confirm("Add ports from another container?", false)
		if merr != nil {
			return nil, merr
		}
		if !more {
			break
		}
	}
	return tunnels, nil
}

func printTunnelPlan(tunnels []plannedTunnel) {
	color.Cyan("\nPort forwarding plan:")
	for _, t := range tunnels {
		tag := "              "
		if t.isDevcontainer {
			tag = color.GreenString("(devcontainer)")
		}
		fmt.Printf("  %s  %s  %s → %s:%d  %s\n",
			t.containerName, tag,
			color.YellowString("localhost:%d", t.localPort),
			t.targetHost, t.containerPort,
			color.WhiteString("(via %s)", t.alias))
	}
	fmt.Println()
}

func runTunnels(tunnels []plannedTunnel) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	cmds := make([]*exec.Cmd, 0, len(tunnels))

	killAll := func() {
		for _, c := range cmds {
			if c.Process != nil {
				_ = c.Process.Kill()
			}
		}
	}

	for _, t := range tunnels {
		c := exec.Command("ssh", "-N", "-L", fmt.Sprintf("%d:%s:%d", t.localPort, t.targetHost, t.containerPort), t.alias)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Start(); err != nil {
			mu.Lock()
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to spawn SSH process: %w", err)
			}
			mu.Unlock()
			killAll()
			continue
		}
		cmds = append(cmds, c)
		tun := t
		wg.Add(1)
		go func(cmd *exec.Cmd) {
			defer wg.Done()
			err := cmd.Wait()
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("SSH tunnel %d→%s:%d (%s) closed: %v", tun.localPort, tun.targetHost, tun.containerPort, tun.alias, err)
				}
				mu.Unlock()
				killAll()
			}
		}(c)
	}

	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	color.Green("\nAll SSH tunnels closed.\n")
	return nil
}

func runPortForward(cmd *cobra.Command, args []string) error {
	alias, _ := cmd.Flags().GetString("alias")
	service, _ := cmd.Flags().GetString("service")
	noI, _ := cmd.Flags().GetBool("no-interactive")
	nonI, _ := cmd.Flags().GetBool("non-interactive")
	interactive := !(noI || nonI)

	var portMapping string
	if len(args) == 1 {
		portMapping = args[0]
	}

	if portMapping == "" && interactive {
		tunnels, err := buildTunnelsInteractive(alias, interactive)
		if err != nil {
			return err
		}
		printTunnelPlan(tunnels)
		ok, cerr := prompt.Confirm(fmt.Sprintf("Open %d tunnel(s) now?", len(tunnels)), true)
		if cerr != nil {
			return cerr
		}
		if !ok {
			color.Yellow("Aborted. No tunnels were opened.")
			return nil
		}
		color.Green("Press Ctrl+C to terminate the port forwarding session.\n")
		return runTunnels(tunnels)
	}

	if portMapping == "" {
		return fmt.Errorf("port mapping is required in non-interactive mode")
	}

	mapping, err := parsePortMapping(portMapping, service)
	if err != nil {
		return err
	}

	containerName := alias
	if containerName == "" {
		containerName = mapping.targetHost
	}
	if alias == "" {
		container, perr := containerpicker.PickManaged("Select devcontainer to forward into:", containerpicker.PickOptions{Interactive: interactive})
		if perr != nil {
			return perr
		}
		containerName = container.Name
		candidate := containerpicker.ContainerWorkspace(container.Name)
		if candidate == "" {
			candidate = container.Name
		}
		aliases := getSSHAliases()
		switch {
		case contains(aliases, candidate):
			alias = candidate
		case len(aliases) == 0:
			if !interactive {
				return fmt.Errorf("no SSH aliases found in config. Run 'setup-ssh' first or specify --alias")
			}
			alias, err = prompt.Input(fmt.Sprintf("No SSH alias found for '%s'. Enter alias manually:", container.Name), candidate, func(v string) error {
				if strings.TrimSpace(v) == "" {
					return fmt.Errorf("SSH alias cannot be empty")
				}
				return nil
			})
			if err != nil {
				return err
			}
		case len(aliases) == 1:
			alias = aliases[0]
			color.Cyan("Using SSH alias '%s'.", alias)
		default:
			if !interactive {
				return fmt.Errorf("SSH alias '%s' not found in ~/.ssh/config. Specify --alias or run interactively", candidate)
			}
			choices := make([]prompt.Choice, len(aliases))
			for i, a := range aliases {
				choices[i] = prompt.Choice{Value: a, Label: a}
			}
			alias, err = prompt.Select(fmt.Sprintf("Select SSH alias for container '%s':", container.Name), choices, aliases[0])
			if err != nil {
				return err
			}
		}
	}

	tunnel := plannedTunnel{
		localPort:      mapping.localPort,
		containerPort:  mapping.containerPort,
		targetHost:     mapping.targetHost,
		alias:          alias,
		containerName:  containerName,
		isDevcontainer: true,
	}
	printTunnelPlan([]plannedTunnel{tunnel})
	color.Green("Press Ctrl+C to terminate the port forwarding session.\n")
	return runTunnels([]plannedTunnel{tunnel})
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/pick"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newPortForwardCommand()) }

func newPortForwardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port-forward [port_mapping]",
		Short: "Forward host ports into a running container over SSH",
		Long: `devcontainer-cli port-forward — open SSH tunnels between your machine and
a running container, in either direction. Every listener binds 127.0.0.1, so a
tunnel is never exposed on another interface. The session stays in the
foreground; press Ctrl+C to close the tunnels.

By default a tunnel goes container -> here: it listens on this machine and
reaches a port inside the container. Prefix a mapping with 'reverse:' (or pass
--reverse) to turn it around, so the container listens and reaches a service
running on this machine — a database, an API, another project's published port.
A reverse tunnel is bound by the container's SSH session, which runs as devuser,
so its container-side port must be 1024 or above.

It tunnels through a devcontainer's SSH alias (set up with 'ssh'), so that
alias must already be configured. Non-devcontainer containers are reached via a
devcontainer used as an SSH jump host.

The port fields mean the same thing in both directions — the first is always
this machine's port, the last always the container's — so only the arrow flips:
  PORT                     127.0.0.1:PORT here -> container:PORT
  LOCAL:CONTAINER          127.0.0.1:LOCAL here -> container:CONTAINER
  LOCAL:HOST:CONTAINER     127.0.0.1:LOCAL here -> HOST:CONTAINER, HOST resolved
                           inside the container network (e.g. a compose service)
  reverse:PORT             127.0.0.1:PORT in the container -> this machine:PORT
  reverse:LOCAL:CONTAINER  127.0.0.1:CONTAINER in the container ->
                           this machine:LOCAL
  reverse:LOCAL:HOST:CONTAINER
                           127.0.0.1:CONTAINER in the container -> HOST:LOCAL,
                           HOST resolved from this machine (e.g. a LAN address)

With no argument it runs an interactive picker: choose any running container,
enter one or more ports, optionally repeat for other containers, then all tunnels
are opened in parallel after you confirm.`,
		Example: `  # Interactive multi-container picker
  devcontainer-cli port-forward

  # 127.0.0.1:3000 -> container:3000
  devcontainer-cli port-forward 3000

  # 127.0.0.1:8080 -> container:80
  devcontainer-cli port-forward 8080:80

  # Reach the 'postgres' service inside the container network
  devcontainer-cli port-forward 5432:postgres:5432
  devcontainer-cli port-forward 5432 --service postgres

  # The other way round: container:5432 -> this machine's 5432
  devcontainer-cli port-forward reverse:5432
  devcontainer-cli port-forward 5432 --reverse

  # This machine's 5432, answering on container:15432
  devcontainer-cli port-forward reverse:5432:15432

  # container:5432 -> a host on your LAN, as seen from this machine
  devcontainer-cli port-forward reverse:5432:192.168.1.20:5432

  # Pin the SSH alias to tunnel through
  devcontainer-cli port-forward 3000 --alias my-custom-host`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE:         runPortForward,
	}
	addPortForwardFlags(cmd)
	return cmd
}

// addPortForwardFlags registers the flags runPortForward reads. It is shared
// with the 'agent forward' facade, which reuses runPortForward with agent-safe
// defaults.
func addPortForwardFlags(cmd *cobra.Command) {
	cmd.Flags().String("alias", "", "SSH host alias to use (bypasses auto-discovery)")
	cmd.Flags().String("service", "", "Compose service to map port to (default: localhost)")
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("reverse", false, "Reverse every mapping without an explicit prefix: the container listens and reaches a service on this machine (a single mapping can still opt in with 'reverse:')")
	cmd.Flags().Bool("ephemeral", false, "Forward directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks")
	addContainerFlag(cmd)
	cmd.Flags().String("key", "", "Private key path (default: the shared managed key under the CLI config dir)")
	cmd.Flags().String("user", sshdefaults.User, "SSH user inside the container")

	_ = cmd.RegisterFlagCompletionFunc("alias", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listSshHosts(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("service", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		cwd, err := currentDir()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return listComposeServices(defaultComposeFile(cwd)), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.MarkFlagFilename("key")
}

type parsedMapping struct {
	localPort     int
	targetHost    string
	containerPort int
	reverse       bool
}

type portPair struct {
	localPort     int
	containerPort int
	reverse       bool
}

// reversePrefixes mark a single spec as a host→container tunnel, whatever
// --reverse says. Having the direction inside the spec is what lets one
// comma-separated list ('3000,reverse:5432') mix both, and what lets a profile
// or a project's forward_ports declare a reverse tunnel at all — those are
// plain strings with nowhere to put a flag.
var reversePrefixes = []string{"reverse:", "r:"}

// splitDirection strips an optional direction prefix, returning the remaining
// mapping and whether the tunnel is reverse. Without a prefix the caller's
// default (the --reverse flag) decides.
func splitDirection(spec string, defaultReverse bool) (string, bool) {
	trimmed := strings.TrimSpace(spec)
	for _, prefix := range reversePrefixes {
		if len(trimmed) >= len(prefix) && strings.EqualFold(trimmed[:len(prefix)], prefix) {
			return strings.TrimSpace(trimmed[len(prefix):]), true
		}
	}
	return trimmed, defaultReverse
}

func parsePort(value, label string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 || n > 65535 {
		return 0, fmt.Errorf("invalid %s: %s", label, value)
	}
	return n, nil
}

// parsePortMapping resolves one tunnel spec. The port fields keep the same
// meaning in both directions — the first is always this machine's port and the
// last always the container's — so only the arrow flips: 'reverse:8080:80'
// makes the container reach this machine's 8080 on its own port 80.
func parsePortMapping(mapping, defaultService string, defaultReverse bool) (parsedMapping, error) {
	mapping, reverse := splitDirection(mapping, defaultReverse)
	if reverse && defaultService != "" {
		return parsedMapping{}, fmt.Errorf("--service names a compose service inside the container network, which a reverse tunnel never dials; use the 'reverse:LOCAL:HOST:CONTAINER' form to name a host reachable from this machine")
	}
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
		return parsedMapping{localPort: port, targetHost: host, containerPort: port, reverse: reverse}, nil
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
		return parsedMapping{localPort: local, targetHost: host, containerPort: container, reverse: reverse}, nil
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
		return parsedMapping{localPort: local, targetHost: targetHost, containerPort: container, reverse: reverse}, nil
	default:
		return parsedMapping{}, fmt.Errorf("invalid port mapping format: %s", mapping)
	}
}

func parsePortPair(item string, defaultReverse bool) (portPair, error) {
	item, reverse := splitDirection(item, defaultReverse)
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
		return portPair{localPort: p, containerPort: p, reverse: reverse}, nil
	case 2:
		local, err := parsePort(parts[0], "local port")
		if err != nil {
			return portPair{}, err
		}
		container, err := parsePort(parts[1], "container port")
		if err != nil {
			return portPair{}, err
		}
		return portPair{localPort: local, containerPort: container, reverse: reverse}, nil
	default:
		return portPair{}, fmt.Errorf("invalid port mapping: %s (expected 'port' or 'local:container')", item)
	}
}

func parsePortsList(value string, defaultReverse bool) ([]portPair, error) {
	var pairs []portPair
	for _, item := range strings.Split(value, ",") {
		if t := strings.TrimSpace(item); t != "" {
			p, err := parsePortPair(t, defaultReverse)
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
	candidate := pick.ContainerWorkspace(containerName)
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

func jumpHostAliases(containers []pick.Container, aliases []string) []string {
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

// runningContainers filters a container list down to the running ones.
func runningContainers(containers []pick.Container) []pick.Container {
	var out []pick.Container
	for _, c := range containers {
		if c.State == "running" {
			out = append(out, c)
		}
	}
	return out
}

func resolveTarget(container pick.Container, aliases []string, containers []pick.Container, jumpHost *string, interactive bool) (alias, targetHost string, err error) {
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
			console.Info("Using SSH jump host '%s'.", *jumpHost)
		case len(candidates) > 1:
			if !interactive {
				return "", "", fmt.Errorf("multiple SSH jump hosts available; specify --alias")
			}
			choices := make([]service.Option, len(candidates))
			for i, a := range candidates {
				choices[i] = service.Option{Value: a, Label: a}
			}
			sel, serr := console.Select("Select the devcontainer SSH host to tunnel through:", choices, choices[0])
			if serr != nil {
				return "", "", serr
			}
			*jumpHost = sel.Value
		default:
			if !interactive {
				return "", "", fmt.Errorf("no devcontainer SSH alias found to tunnel through '%s'. Run 'ssh --setup' first or specify --alias", container.Name)
			}
			in, ierr := console.AskDefault(fmt.Sprintf("No devcontainer SSH alias found to reach '%s'. Enter SSH alias to tunnel through:", container.Name), "", func(v string) error {
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

func buildTunnelsInteractive(flagAlias string, reverse, interactive bool) ([]service.Tunnel, error) {
	// You can only forward into a running container, so filter the unified list
	// down to running ones here.
	containers := runningContainers(pick.ListAll())
	if len(containers) == 0 {
		return nil, fmt.Errorf("no running containers found. Start one with 'devcontainer-cli up', or create a project first with 'devcontainer-cli agent create'")
	}
	aliases := getSSHAliases()
	var tunnels []service.Tunnel
	jumpHost := flagAlias

	for {
		container, err := pickContainer(containers, "Select a container to forward from:", pickContainerOptions{Interactive: interactive})
		if err != nil {
			return nil, err
		}

		alias, targetHost, terr := resolveTarget(container, aliases, containers, &jumpHost, interactive)
		if terr != nil {
			return nil, terr
		}

		portsStr, ierr := console.AskDefault(fmt.Sprintf("Ports to forward from '%s' (e.g. 3000, 8080:80, reverse:5432):", container.Name), "", func(v string) error {
			_, e := parsePortsList(v, reverse)
			return e
		})
		if ierr != nil {
			return nil, ierr
		}
		pairs, _ := parsePortsList(portsStr, reverse)
		for _, pair := range pairs {
			// A reverse tunnel is opened by the SSH server, so it can only ever
			// listen inside the container the alias connects to. Reaching
			// another container through it as a jump host works one way only.
			if pair.reverse && targetHost != "localhost" {
				return nil, fmt.Errorf("cannot open a reverse tunnel into '%s': it is reached through the jump host '%s', and a reverse tunnel listens inside the container it connects to. Select a devcontainer with its own SSH alias instead", container.Name, alias)
			}
			tunnels = append(tunnels, service.Tunnel{
				LocalPort:      pair.localPort,
				ContainerPort:  pair.containerPort,
				Reverse:        pair.reverse,
				TargetHost:     targetHost,
				Alias:          alias,
				ContainerName:  container.Name,
				IsDevcontainer: container.Managed,
			})
		}

		more, merr := console.ConfirmDefault("Add ports from another container?", false)
		if merr != nil {
			return nil, merr
		}
		if !more {
			break
		}
	}
	return tunnels, nil
}

func printTunnelPlan(tunnels []service.Tunnel) {
	console.Info("\nPort forwarding plan:")
	for _, t := range tunnels {
		tag := "              "
		if t.IsDevcontainer {
			tag = console.SuccessS("(devcontainer)")
		}
		via := console.Subtle(fmt.Sprintf("(via %s)", t.Alias))
		if t.Ephemeral {
			via = console.Subtle("(ephemeral)")
		}
		// The listening side is highlighted and always written first, so a
		// reverse tunnel reads as what it is: the container listening.
		if t.Reverse {
			console.Info("  %s  %s  %s → %s:%d  %s",
				t.ContainerName, tag,
				console.WarnS("container 127.0.0.1:%d", t.ContainerPort),
				t.TargetHost, t.LocalPort,
				via)
			continue
		}
		console.Info("  %s  %s  %s → %s:%d  %s",
			t.ContainerName, tag,
			console.WarnS("127.0.0.1:%d", t.LocalPort),
			t.TargetHost, t.ContainerPort,
			via)
	}
	warnPrivilegedReversePorts(tunnels)
	console.NewLine()
}

// privilegedPort is the first port a non-root user may bind.
const privilegedPort = 1024

// needsPrivilegedBind reports whether the tunnel asks the container's SSH
// session to bind a port only root may take.
func needsPrivilegedBind(t service.Tunnel) bool {
	return t.Reverse && t.ContainerPort < privilegedPort
}

// warnPrivilegedReversePorts flags the one failure ssh reports in terms of
// nothing the user typed ("remote port forwarding failed for listen port 80"):
// the container side of a reverse tunnel is bound by the SSH session, which
// runs as devuser and cannot take a privileged port. It stays a warning rather
// than a rejection because the session's user is configurable.
func warnPrivilegedReversePorts(tunnels []service.Tunnel) {
	for _, t := range tunnels {
		if needsPrivilegedBind(t) {
			console.Warn("Container port %d is privileged: the SSH session binds it as '%s', so this tunnel will be refused unless that user may bind ports below %d. Map it to a higher container port instead (e.g. reverse:%d:%d).",
				t.ContainerPort, sshdefaults.User, privilegedPort, t.LocalPort, t.ContainerPort+8000)
		}
	}
}

func runPortForward(cmd *cobra.Command, args []string) error {
	ephemeral, _ := cmd.Flags().GetBool("ephemeral")
	alias, _ := cmd.Flags().GetString("alias")
	serviceFlag, _ := cmd.Flags().GetString("service")
	reverse, _ := cmd.Flags().GetBool("reverse")
	interactive := interactiveFlag(cmd)

	var portMapping string
	if len(args) == 1 {
		portMapping = args[0]
	}

	if ephemeral {
		if portMapping == "" {
			return fmt.Errorf("port mapping is required in non-interactive/ephemeral mode")
		}
		mapping, err := parsePortMapping(portMapping, serviceFlag, reverse)
		if err != nil {
			return err
		}

		containerName, err := resolveContainer(cmd)
		if err != nil {
			return err
		}

		keyPath, _ := cmd.Flags().GetString("key")
		user, _ := cmd.Flags().GetString("user")

		svc := service.PortForwardService{Report: console}
		tunnel, err := svc.BuildEphemeralTunnel(service.Tunnel{
			LocalPort:     mapping.localPort,
			ContainerPort: mapping.containerPort,
			Reverse:       mapping.reverse,
			TargetHost:    mapping.targetHost,
			ContainerName: containerName,
		}, user, keyPath)
		if err != nil {
			return err
		}
		printTunnelPlan([]service.Tunnel{tunnel})
		console.Success("Press Ctrl+C to terminate the port forwarding session.\n")
		return svc.OpenTunnels([]service.Tunnel{tunnel})
	}

	// With no argument, a project that configured its tunnels (from a profile or
	// --forward-ports) gets those instead of the picker: that is what makes them
	// automatic. Passing a mapping still overrides them.
	if portMapping == "" {
		opened, err := forwardConfiguredPorts(alias, serviceFlag, reverse, interactive)
		if err != nil {
			return err
		}
		if opened {
			return nil
		}
	}

	if portMapping == "" && interactive {
		tunnels, err := buildTunnelsInteractive(alias, reverse, interactive)
		if err != nil {
			return err
		}
		printTunnelPlan(tunnels)
		ok, cerr := console.ConfirmDefault(fmt.Sprintf("Open %d tunnel(s) now?", len(tunnels)), true)
		if cerr != nil {
			return cerr
		}
		if !ok {
			console.Warn("Aborted. No tunnels were opened.")
			return nil
		}
		console.Success("Press Ctrl+C to terminate the port forwarding session.\n")
		return service.PortForwardService{Report: console}.OpenTunnels(tunnels)
	}

	if portMapping == "" {
		return fmt.Errorf("port mapping is required in non-interactive mode")
	}

	mapping, err := parsePortMapping(portMapping, serviceFlag, reverse)
	if err != nil {
		return err
	}

	alias, containerName, err := resolveTunnelTarget(alias, mapping.targetHost, interactive)
	if err != nil {
		return err
	}

	tunnel := service.Tunnel{
		LocalPort:      mapping.localPort,
		ContainerPort:  mapping.containerPort,
		Reverse:        mapping.reverse,
		TargetHost:     mapping.targetHost,
		Alias:          alias,
		ContainerName:  containerName,
		IsDevcontainer: true,
	}
	printTunnelPlan([]service.Tunnel{tunnel})
	console.Success("Press Ctrl+C to terminate the port forwarding session.\n")
	return service.PortForwardService{Report: console}.OpenTunnels([]service.Tunnel{tunnel})
}

// forwardConfiguredPorts opens the tunnels the current project declared, and
// reports whether it opened any. A directory that is not a generated project,
// or one that declared none, falls through to the usual paths.
func forwardConfiguredPorts(alias, serviceFlag string, reverse, interactive bool) (bool, error) {
	cwd, err := currentDir()
	if err != nil {
		return false, nil
	}
	config, err := domain.LoadConfig(cwd)
	if err != nil || config == nil || len(config.ForwardPorts) == 0 {
		return false, nil
	}

	mappings := make([]parsedMapping, 0, len(config.ForwardPorts))
	for _, spec := range config.ForwardPorts {
		mapping, perr := parsePortMapping(spec, serviceFlag, reverse)
		if perr != nil {
			return false, fmt.Errorf("configured forward port %q: %w", spec, perr)
		}
		mappings = append(mappings, mapping)
	}

	alias, containerName, err := resolveTunnelTarget(alias, mappings[0].targetHost, interactive)
	if err != nil {
		return false, err
	}

	tunnels := make([]service.Tunnel, 0, len(mappings))
	for _, mapping := range mappings {
		tunnels = append(tunnels, service.Tunnel{
			LocalPort:      mapping.localPort,
			ContainerPort:  mapping.containerPort,
			Reverse:        mapping.reverse,
			TargetHost:     mapping.targetHost,
			Alias:          alias,
			ContainerName:  containerName,
			IsDevcontainer: true,
		})
	}

	console.Info("Forwarding this project's configured ports: %s", strings.Join(config.ForwardPorts, ", "))
	printTunnelPlan(tunnels)
	console.Success("Press Ctrl+C to terminate the port forwarding session.\n")
	return true, service.PortForwardService{Report: console}.OpenTunnels(tunnels)
}

// resolveTunnelTarget settles which container to tunnel into and which SSH alias
// to reach it through. With an explicit --alias both are already known; without
// one it picks a managed container and then matches it to an alias, asking when
// the match is ambiguous.
func resolveTunnelTarget(alias, fallbackContainer string, interactive bool) (string, string, error) {
	if alias != "" {
		return alias, alias, nil
	}

	container, err := pickManagedContainer("Select devcontainer to forward into:", pickContainerOptions{Interactive: interactive})
	if err != nil {
		return "", "", err
	}
	containerName := container.Name
	candidate := pick.ContainerWorkspace(container.Name)
	if candidate == "" {
		candidate = container.Name
	}

	aliases := getSSHAliases()
	switch {
	case slices.Contains(aliases, candidate):
		return candidate, containerName, nil
	case len(aliases) == 0:
		if !interactive {
			return "", "", fmt.Errorf("no SSH aliases found in config. Run 'ssh --setup' first or specify --alias")
		}
		entered, aerr := console.AskDefault(fmt.Sprintf("No SSH alias found for '%s'. Enter alias manually:", container.Name), candidate, func(v string) error {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("SSH alias cannot be empty")
			}
			return nil
		})
		if aerr != nil {
			return "", "", aerr
		}
		return entered, containerName, nil
	case len(aliases) == 1:
		console.Info("Using SSH alias '%s'.", aliases[0])
		return aliases[0], containerName, nil
	default:
		if !interactive {
			return "", "", fmt.Errorf("SSH alias '%s' not found in any SSH config. Specify --alias or run interactively", candidate)
		}
		choices := make([]service.Option, len(aliases))
		for i, a := range aliases {
			choices[i] = service.Option{Value: a, Label: a}
		}
		chosen, serr := console.Select(fmt.Sprintf("Select SSH alias for container '%s':", container.Name), choices, choices[0])
		if serr != nil {
			return "", "", serr
		}
		return chosen.Value, containerName, nil
	}
}

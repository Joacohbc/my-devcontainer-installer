package commands

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service/pick"
	"github.com/spf13/cobra"
)

func newPortForwardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "forward [port-mapping]",
		Aliases: []string{"fwd"},
		Short:   "Forward host ports into a running container over SSH",
		Long: `devcontainer-cli access forward — forward TCP ports over SSH between this
machine and a running container.

Forward mappings (default) listen on this machine and reach the container
(or a sibling service on its compose network):

  devcontainer-cli access forward 3000                     # 127.0.0.1:3000 -> container:3000
  devcontainer-cli access forward 8080:80                  # 127.0.0.1:8080 -> container:80
  devcontainer-cli access forward 100.64.0.1:8080:80       # 100.64.0.1:8080 -> container:80
  devcontainer-cli access forward 5432 --service postgres  # 127.0.0.1:5432 -> postgres:5432
  devcontainer-cli access forward 5432:postgres:5432       # inline service target

Reverse mappings go the other way — the container listens on 127.0.0.1 and
reaches a port on this machine (useful for an Ollama / LM Studio server, a
local webhook receiver, or a host DB):

  devcontainer-cli access forward reverse:11434            # container:11434 -> host:11434
  devcontainer-cli access forward r:8080:3000              # container:3000 -> host:8080
  devcontainer-cli access forward --reverse 11434          # every mapping on this call is reverse

Reach a container on a different Docker host through an existing SSH connection
to it using 'access forward via <host> <container> [port-mapping]'.

Running with no arguments forwards this project's configured ports (from its
profile or --forward-ports) when any were configured, or opens an interactive
picker to combine several ports across containers.

SSH access is configured automatically if needed; press Ctrl+C to stop.`,
		Example: `  # Forward container port 3000 to 127.0.0.1:3000 (ephemeral mode, default)
  devcontainer-cli access forward 3000

  # Bind to a specific local IP (e.g. Tailscale interface)
  devcontainer-cli access forward 100.64.0.1:8080:80

  # Forward Postgres on the project network to 127.0.0.1:5432
  devcontainer-cli access forward 5432 --service postgres

  # Forward a port from a container on a remote Docker host
  devcontainer-cli access forward via me@docker-host dc-base 3000

  # Let the container reach Ollama running on this machine
  devcontainer-cli access forward reverse:11434

  # Forward using a persistent SSH alias in ~/.ssh/devcontainer-cli.config
  devcontainer-cli access forward 3000 --ephemeral=false`,
		Args: cobra.MaximumNArgs(1),
		RunE: runPortForward,
	}
	addForwardFlags(cmd)
	cmd.AddCommand(newPortForwardViaCommand())
	return cmd
}

func newPortForwardViaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "via <host> <container> [port-mapping]",
		Short: "Forward host ports into a container on a remote Docker host over SSH",
		Long: `devcontainer-cli access forward via — forward TCP ports over SSH between this
machine and a container running on a remote Docker host reached through an
existing SSH connection to <host>.`,
		Example: `  # Forward remote container port 3000 to 127.0.0.1:3000 (ephemeral, default)
  devcontainer-cli access forward via me@docker-host dc-base 3000

  # Forward a sibling service port on the remote compose network
  devcontainer-cli access forward via me@docker-host dc-base 5432 --service postgres

  # Configure/reuse persistent ProxyCommand alias and forward
  devcontainer-cli access forward via me@docker-host dc-base 3000 --ephemeral=false

  # Interactive port prompt for the remote container
  devcontainer-cli access forward via me@docker-host dc-base`,
		Args:              portForwardViaArgs,
		ValidArgsFunction: completeViaArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			portMapping := ""
			if len(args) == 3 {
				portMapping = args[2]
			}
			return runPortForwardWithTarget(cmd, args[0], args[1], portMapping)
		},
	}
	addForwardSessionFlags(cmd)
	return cmd
}

func portForwardViaArgs(_ *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("access forward via requires <host> <container> [port-mapping]")
	}
	if len(args) > 3 {
		return fmt.Errorf("accepts at most 3 arg(s), received %d", len(args))
	}
	if strings.TrimSpace(args[0]) == "" {
		return fmt.Errorf("remote host cannot be empty")
	}
	if strings.TrimSpace(args[1]) == "" {
		return fmt.Errorf("target container name cannot be empty")
	}
	return nil
}

type parsedMapping struct {
	bindAddress   string
	localPort     int
	targetHost    string
	containerPort int
	reverse       bool
}

// reversePrefixes mark a single spec as a host→container tunnel, whatever
// --reverse says. Having the direction inside the spec is what lets one
// comma-separated list ('3000,reverse:5432') mix both, and what lets a profile
// mix forward and reverse ports in one forward_ports list.
var reversePrefixes = []string{"reverse:", "rev:", "r:"}

func splitDirection(spec string, defaultReverse bool) (string, bool) {
	trimmed := strings.TrimSpace(spec)
	lower := strings.ToLower(trimmed)
	for _, p := range reversePrefixes {
		if strings.HasPrefix(lower, p) {
			return strings.TrimSpace(trimmed[len(p):]), true
		}
	}
	return trimmed, defaultReverse
}

// isBindAddress reports whether s is a valid IPv4 or IPv6 literal (with or
// without surrounding brackets for IPv6, e.g. "[::1]" or "::1").
func isBindAddress(s string) bool {
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		ip := net.ParseIP(s[1 : len(s)-1])
		return ip != nil && ip.To4() == nil
	}
	return net.ParseIP(s) != nil
}

// splitSpecTokens splits a port-mapping body on ':' while keeping bracketed
// IPv6 literals like "[::1]" intact as a single token.
func splitSpecTokens(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inBracket := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '[':
			if inBracket {
				return nil, fmt.Errorf("invalid bracket in port mapping: %s", s)
			}
			inBracket = true
			cur.WriteByte(c)
		case ']':
			if !inBracket {
				return nil, fmt.Errorf("unmatched ']' in port mapping: %s", s)
			}
			inBracket = false
			cur.WriteByte(c)
		case ':':
			if inBracket {
				cur.WriteByte(c)
			} else {
				tokens = append(tokens, strings.TrimSpace(cur.String()))
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if inBracket {
		return nil, fmt.Errorf("unclosed '[' in port mapping: %s", s)
	}
	tokens = append(tokens, strings.TrimSpace(cur.String()))
	return tokens, nil
}

func parsePortMapping(mapping, defaultService string, defaultReverse bool) (parsedMapping, error) {
	body, reverse := splitDirection(mapping, defaultReverse)
	parsed, err := parsePortFields(body, defaultService, reverse)
	if err != nil {
		return parsedMapping{}, err
	}
	if parsed.reverse && parsed.bindAddress != "" {
		return parsedMapping{}, fmt.Errorf("reverse port mapping cannot specify a local bind address (%q)", parsed.bindAddress)
	}
	if parsed.reverse && parsed.targetHost != "localhost" {
		return parsedMapping{}, fmt.Errorf("reverse port mapping cannot target service '%s': the container opens the listener itself and connects back to this machine", parsed.targetHost)
	}
	return parsed, nil
}

func parsePortFields(mapping, defaultService string, reverse bool) (parsedMapping, error) {
	target := defaultService
	if target == "" {
		target = "localhost"
	}
	parts, err := splitSpecTokens(mapping)
	if err != nil {
		return parsedMapping{}, err
	}

	var bindAddr string
	if len(parts) >= 3 && isBindAddress(parts[0]) {
		bindAddr = parts[0]
		parts = parts[1:]
	}

	switch len(parts) {
	case 1:
		port, err := parsePort(parts[0], "port")
		if err != nil {
			return parsedMapping{}, err
		}
		return parsedMapping{bindAddress: bindAddr, localPort: port, targetHost: target, containerPort: port, reverse: reverse}, nil
	case 2:
		local, err := parsePort(parts[0], "local port")
		if err != nil {
			return parsedMapping{}, err
		}
		container, err := parsePort(parts[1], "container port")
		if err != nil {
			return parsedMapping{}, err
		}
		return parsedMapping{bindAddress: bindAddr, localPort: local, targetHost: target, containerPort: container, reverse: reverse}, nil
	case 3:
		local, err := parsePort(parts[0], "local port")
		if err != nil {
			return parsedMapping{}, err
		}
		if parts[1] == "" {
			return parsedMapping{}, fmt.Errorf("target host cannot be empty in '%s'", mapping)
		}
		container, err := parsePort(parts[2], "container port")
		if err != nil {
			return parsedMapping{}, err
		}
		return parsedMapping{bindAddress: bindAddr, localPort: local, targetHost: parts[1], containerPort: container, reverse: reverse}, nil
	default:
		return parsedMapping{}, fmt.Errorf("invalid port mapping: %s (expected 'port', 'local:container', 'bind:local:container', 'local:host:container', or 'bind:local:host:container')", mapping)
	}
}

func parsePortsList(value string, defaultReverse bool) ([]parsedMapping, error) {
	var mappings []parsedMapping
	for _, item := range strings.Split(value, ",") {
		if t := strings.TrimSpace(item); t != "" {
			m, err := parsePortMapping(t, "", defaultReverse)
			if err != nil {
				return nil, err
			}
			mappings = append(mappings, m)
		}
	}
	if len(mappings) == 0 {
		return nil, fmt.Errorf("no ports specified")
	}
	return mappings, nil
}

var hostLineRe = regexp.MustCompile(`(?i)^[ \t]*host[ \t]+([^#]+)`)

func getSSHAliases() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, ".ssh", "devcontainer-cli.config"))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		m := hostLineRe.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		for _, tok := range strings.Fields(m[1]) {
			if tok != "*" && !seen[tok] {
				seen[tok] = true
				out = append(out, tok)
			}
		}
	}
	return out
}

func runningContainers(all []pick.Container) []pick.Container {
	var out []pick.Container
	for _, c := range all {
		if c.Status == "running" {
			out = append(out, c)
		}
	}
	return out
}

func resolveTarget(container pick.Container, aliases []string, containers []pick.Container, jumpHost *string, interactive bool) (alias, targetHost string, err error) {
	if container.Managed {
		for _, a := range aliases {
			if a == container.Name {
				return a, "localhost", nil
			}
		}
		return "", "", fmt.Errorf("no SSH alias configured for '%s'. Run 'devcontainer-cli access ssh --setup' in that project first", container.Name)
	}

	if *jumpHost == "" {
		// Unmanaged containers have no sshd; SSH lands in a devcontainer on the
		// same network and tunnels onward by container name.
		var devcontainers []pick.Container
		for _, c := range containers {
			if c.Managed {
				devcontainers = append(devcontainers, c)
			}
		}
		if len(devcontainers) == 0 {
			return "", "", fmt.Errorf("'%s' is not a devcontainer and no running devcontainer was found to jump through", container.Name)
		}
		jump, jerr := pickContainer(devcontainers, fmt.Sprintf("Select a devcontainer to jump through to '%s':", container.Name), pickContainerOptions{Interactive: interactive})
		if jerr != nil {
			return "", "", jerr
		}
		for _, a := range aliases {
			if a == jump.Name {
				*jumpHost = a
				break
			}
		}
		if *jumpHost == "" {
			return "", "", fmt.Errorf("no SSH alias configured for jump host '%s'. Run 'devcontainer-cli access ssh --setup' in that project first", jump.Name)
		}
	}
	return *jumpHost, container.Name, nil
}

type forwardOptions struct {
	cmd         *cobra.Command
	ephemeral   bool
	alias       string
	via         string
	container   string
	serviceFlag string
	reverse     bool
	interactive bool
	user        string
	keyPath     string
}

func resolveForwardEphemeral(cmd *cobra.Command) (ephemeral bool, alias string, err error) {
	ephemeral, _ = cmd.Flags().GetBool("ephemeral")
	if cmd.Flags().Lookup("alias") != nil {
		alias, _ = cmd.Flags().GetString("alias")
	}
	if alias != "" {
		if cmd.Flags().Changed("ephemeral") && ephemeral {
			return false, "", fmt.Errorf("--alias cannot be used with --ephemeral=true: --alias selects a configured SSH host block")
		}
		ephemeral = false
	}
	return ephemeral, alias, nil
}

func buildTunnelsInteractive(opts forwardOptions) ([]service.Tunnel, error) {
	// Remote jump target ('via <host> <container>') already identifies the
	// remote container, so prompt directly for ports without listing local
	// containers.
	if opts.via != "" {
		return buildRemoteTunnelsInteractive(opts)
	}

	// You can only forward into a running container, so filter the unified list
	// down to running ones here.
	containers := runningContainers(pick.ListAll())
	if len(containers) == 0 {
		return nil, fmt.Errorf("no running containers found. Start one with 'devcontainer-cli workspace up', or create a project first with 'devcontainer-cli agent create'")
	}
	var aliases []string
	if !opts.ephemeral {
		aliases = getSSHAliases()
	}
	var tunnels []service.Tunnel
	jumpHost := opts.alias
	pfSvc := service.PortForwardService{Report: console}

	for {
		container, err := pickContainer(containers, "Select a container to forward from:", pickContainerOptions{Interactive: opts.interactive})
		if err != nil {
			return nil, err
		}

		var alias string
		targetHost := "localhost"
		if !opts.ephemeral {
			var terr error
			alias, targetHost, terr = resolveTarget(container, aliases, containers, &jumpHost, opts.interactive)
			if terr != nil {
				return nil, terr
			}
		}

		portsStr, ierr := console.AskDefault(fmt.Sprintf("Ports to forward from '%s' (e.g. 3000, 8080:80, 100.64.0.1:8080:80, reverse:5432):", container.Name), "", func(v string) error {
			_, e := parsePortsList(v, opts.reverse)
			return e
		})
		if ierr != nil {
			return nil, ierr
		}
		mappings, _ := parsePortsList(portsStr, opts.reverse)
		for _, mapping := range mappings {
			// A reverse tunnel is opened by the SSH server, so it can only ever
			// listen inside the container the alias connects to. Reaching
			// another container through it as a jump host works one way only.
			if mapping.reverse && targetHost != "localhost" {
				return nil, fmt.Errorf("cannot open a reverse tunnel into '%s': it is reached through the jump host '%s', and a reverse tunnel listens inside the container it connects to. Select a devcontainer with its own SSH alias instead", container.Name, alias)
			}
			tunnelTargetHost := targetHost
			if targetHost == "localhost" {
				tunnelTargetHost = mapping.targetHost
			} else if mapping.targetHost != "localhost" && mapping.targetHost != targetHost {
				return nil, fmt.Errorf("conflicting target hosts: mapping specifies '%s' but selected container target is '%s'", mapping.targetHost, targetHost)
			}
			tunnel := service.Tunnel{
				BindAddress:    mapping.bindAddress,
				LocalPort:      mapping.localPort,
				ContainerPort:  mapping.containerPort,
				Reverse:        mapping.reverse,
				TargetHost:     tunnelTargetHost,
				Alias:          alias,
				ContainerName:  container.Name,
				IsDevcontainer: container.Managed,
			}
			if opts.ephemeral {
				built, berr := pfSvc.BuildEphemeralTunnel(tunnel, opts.user, opts.keyPath)
				if berr != nil {
					return nil, berr
				}
				tunnel = built
			}
			tunnels = append(tunnels, tunnel)
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

func buildRemoteTunnelsInteractive(opts forwardOptions) ([]service.Tunnel, error) {
	portsStr, ierr := console.AskDefault(fmt.Sprintf("Ports to forward from '%s' on '%s' (e.g. 3000, 8080:80, 100.64.0.1:8080:80, reverse:5432):", opts.container, opts.via), "", func(v string) error {
		_, e := parsePortsList(v, opts.reverse)
		return e
	})
	if ierr != nil {
		return nil, ierr
	}
	mappings, _ := parsePortsList(portsStr, opts.reverse)
	pfSvc := service.PortForwardService{Report: console}

	var alias string
	if !opts.ephemeral {
		var serr error
		alias, serr = resolveRemoteForwardAlias(opts)
		if serr != nil {
			return nil, serr
		}
	}

	tunnels := make([]service.Tunnel, 0, len(mappings))
	for _, mapping := range mappings {
		targetHost := mapping.targetHost
		if targetHost == "localhost" && opts.serviceFlag != "" {
			targetHost = opts.serviceFlag
		}
		if opts.ephemeral {
			built, berr := pfSvc.BuildEphemeralTunnelVia(service.Tunnel{
				BindAddress:   mapping.bindAddress,
				LocalPort:     mapping.localPort,
				ContainerPort: mapping.containerPort,
				Reverse:       mapping.reverse,
				TargetHost:    targetHost,
				ContainerName: opts.container,
			}, opts.user, opts.keyPath, opts.via)
			if berr != nil {
				return nil, berr
			}
			tunnels = append(tunnels, built)
		} else {
			tunnels = append(tunnels, service.Tunnel{
				BindAddress:    mapping.bindAddress,
				LocalPort:      mapping.localPort,
				ContainerPort:  mapping.containerPort,
				Reverse:        mapping.reverse,
				TargetHost:     targetHost,
				Alias:          alias,
				ContainerName:  opts.container,
				IsDevcontainer: true,
			})
		}
	}
	return tunnels, nil
}

func resolveRemoteForwardAlias(opts forwardOptions) (string, error) {
	for _, a := range getSSHAliases() {
		if a == opts.container {
			return a, nil
		}
	}
	return runSshSetupWithTarget(opts.cmd, opts.via, opts.container)
}

func printTunnelPlan(tunnels []service.Tunnel) {
	console.Header("Port Forwarding Plan")
	for _, t := range tunnels {
		bindAddr := t.EffectiveBindAddress()
		if t.Reverse {
			console.Info("  %s:127.0.0.1:%d -> %s:%d (reverse)", t.ContainerName, t.ContainerPort, bindAddr, t.LocalPort)
			continue
		}
		dest := fmt.Sprintf("%s:%d", t.ContainerName, t.ContainerPort)
		if t.TargetHost != "" && t.TargetHost != "localhost" {
			dest = fmt.Sprintf("%s (%s:%d)", t.ContainerName, t.TargetHost, t.ContainerPort)
		}
		console.Info("  %s:%d -> %s", bindAddr, t.LocalPort, dest)
	}
	warnPrivilegedReversePorts(tunnels)
}

// warnPrivilegedReversePorts alerts the user before ssh -N runs when a reverse
// tunnel asks sshd inside the container to bind a port below 1024: non-root
// users cannot bind those without CAP_NET_BIND_SERVICE, so ssh fails with
// "remote port forwarding failed for listen port N".
func warnPrivilegedReversePorts(tunnels []service.Tunnel) {
	for _, t := range tunnels {
		if needsPrivilegedBind(t) {
			console.Warn("Reverse port %d on '%s' is below 1024; binding it inside the container requires root or CAP_NET_BIND_SERVICE and may fail.", t.ContainerPort, t.ContainerName)
		}
	}
}

func needsPrivilegedBind(t service.Tunnel) bool {
	return t.Reverse && t.ContainerPort > 0 && t.ContainerPort < 1024
}

func runPortForward(cmd *cobra.Command, args []string) error {
	var portMapping string
	if len(args) == 1 {
		portMapping = args[0]
	}
	return runPortForwardWithTarget(cmd, "", "", portMapping)
}

func runPortForwardWithTarget(cmd *cobra.Command, via, container, portMapping string) error {
	ephemeral, alias, err := resolveForwardEphemeral(cmd)
	if err != nil {
		return err
	}
	serviceFlag, _ := cmd.Flags().GetString("service")
	reverse, _ := cmd.Flags().GetBool("reverse")
	keyPath, _ := cmd.Flags().GetString("key")
	user, _ := cmd.Flags().GetString("user")
	opts := forwardOptions{
		cmd:         cmd,
		ephemeral:   ephemeral,
		alias:       alias,
		via:         via,
		container:   container,
		serviceFlag: serviceFlag,
		reverse:     reverse,
		interactive: interactiveFlag(cmd),
		user:        user,
		keyPath:     keyPath,
	}

	// With no argument on a local workspace, a project that configured its
	// tunnels (from a profile or --forward-ports) gets those instead of the
	// picker: that is what makes them automatic. Passing a mapping still
	// overrides them.
	if portMapping == "" {
		if opts.via == "" {
			opened, err := forwardConfiguredPorts(opts)
			if err != nil {
				return err
			}
			if opened {
				return nil
			}
		}
		if !opts.interactive {
			return fmt.Errorf("port mapping is required in non-interactive mode")
		}
		return openInteractiveTunnels(opts)
	}

	return openSingleTunnel(opts, portMapping)
}

func openInteractiveTunnels(opts forwardOptions) error {
	tunnels, err := buildTunnelsInteractive(opts)
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

func openSingleTunnel(opts forwardOptions, portMapping string) error {
	mapping, err := parsePortMapping(portMapping, opts.serviceFlag, opts.reverse)
	if err != nil {
		return err
	}

	pfSvc := service.PortForwardService{Report: console}
	var tunnel service.Tunnel
	if opts.ephemeral {
		containerName := opts.container
		if containerName == "" {
			var cerr error
			containerName, cerr = resolveContainer(opts.cmd)
			if cerr != nil {
				return cerr
			}
		}
		tunnel, err = pfSvc.BuildEphemeralTunnelVia(service.Tunnel{
			BindAddress:   mapping.bindAddress,
			LocalPort:     mapping.localPort,
			ContainerPort: mapping.containerPort,
			Reverse:       mapping.reverse,
			TargetHost:    mapping.targetHost,
			ContainerName: containerName,
		}, opts.user, opts.keyPath, opts.via)
		if err != nil {
			return err
		}
	} else if opts.via != "" {
		alias, terr := resolveRemoteForwardAlias(opts)
		if terr != nil {
			return terr
		}
		tunnel = service.Tunnel{
			BindAddress:    mapping.bindAddress,
			LocalPort:      mapping.localPort,
			ContainerPort:  mapping.containerPort,
			Reverse:        mapping.reverse,
			TargetHost:     mapping.targetHost,
			Alias:          alias,
			ContainerName:  opts.container,
			IsDevcontainer: true,
		}
	} else {
		alias, containerName, terr := resolveTunnelTarget(opts.alias, mapping.targetHost, opts.interactive)
		if terr != nil {
			return terr
		}
		tunnel = service.Tunnel{
			BindAddress:    mapping.bindAddress,
			LocalPort:      mapping.localPort,
			ContainerPort:  mapping.containerPort,
			Reverse:        mapping.reverse,
			TargetHost:     mapping.targetHost,
			Alias:          alias,
			ContainerName:  containerName,
			IsDevcontainer: true,
		}
	}

	printTunnelPlan([]service.Tunnel{tunnel})
	console.Success("Press Ctrl+C to terminate the port forwarding session.\n")
	return pfSvc.OpenTunnels([]service.Tunnel{tunnel})
}

// forwardConfiguredPorts opens the tunnels the current project declared, and
// reports whether it opened any. A directory that is not a generated project,
// or one that declared none, falls through to the usual paths.
func forwardConfiguredPorts(opts forwardOptions) (bool, error) {
	cwd, err := currentDir()
	if err != nil {
		return false, nil
	}
	config, err := loadWorkspaceConfig(cwd)
	if err != nil || len(config.ForwardPorts) == 0 {
		return false, nil
	}

	mappings := make([]parsedMapping, 0, len(config.ForwardPorts))
	for _, spec := range config.ForwardPorts {
		mapping, perr := parsePortMapping(spec, opts.serviceFlag, opts.reverse)
		if perr != nil {
			return false, fmt.Errorf("configured forward port %q: %w", spec, perr)
		}
		mappings = append(mappings, mapping)
	}

	pfSvc := service.PortForwardService{Report: console}
	tunnels := make([]service.Tunnel, 0, len(mappings))
	if opts.ephemeral {
		containerName, cerr := resolveContainer(opts.cmd)
		if cerr != nil {
			return false, cerr
		}
		for _, mapping := range mappings {
			built, berr := pfSvc.BuildEphemeralTunnel(service.Tunnel{
				BindAddress:   mapping.bindAddress,
				LocalPort:     mapping.localPort,
				ContainerPort: mapping.containerPort,
				Reverse:       mapping.reverse,
				TargetHost:    mapping.targetHost,
				ContainerName: containerName,
			}, opts.user, opts.keyPath)
			if berr != nil {
				return false, berr
			}
			tunnels = append(tunnels, built)
		}
	} else {
		alias, containerName, terr := resolveTunnelTarget(opts.alias, mappings[0].targetHost, opts.interactive)
		if terr != nil {
			return false, terr
		}
		for _, mapping := range mappings {
			tunnels = append(tunnels, service.Tunnel{
				BindAddress:    mapping.bindAddress,
				LocalPort:      mapping.localPort,
				ContainerPort:  mapping.containerPort,
				Reverse:        mapping.reverse,
				TargetHost:     mapping.targetHost,
				Alias:          alias,
				ContainerName:  containerName,
				IsDevcontainer: true,
			})
		}
	}

	console.Info("Forwarding this project's configured ports: %s", strings.Join(config.ForwardPorts, ", "))
	printTunnelPlan(tunnels)
	console.Success("Press Ctrl+C to terminate the port forwarding session.\n")
	return true, pfSvc.OpenTunnels(tunnels)
}

// resolveTunnelTarget settles which container to tunnel into and which SSH alias
// reaches it, given the optional --alias and --service flags. It mirrors
// buildTunnelsInteractive so single-mapping runs and interactive runs obey the
// exact same rules.
func resolveTunnelTarget(flagAlias, targetHost string, interactive bool) (alias, containerName string, err error) {
	aliases := getSSHAliases()
	if len(aliases) == 0 {
		return "", "", fmt.Errorf("no SSH aliases configured yet. Run 'devcontainer-cli access ssh --setup' in a project directory first")
	}
	containers := runningContainers(pick.ListAll())
	if len(containers) == 0 {
		return "", "", fmt.Errorf("no running containers found. Start one with 'devcontainer-cli workspace up'")
	}

	if flagAlias != "" {
		return flagAlias, flagAlias, nil
	}

	// If --service points at a running container (e.g. --service postgres),
	// jump through a devcontainer so the tunnel lands on that container's
	// network. Otherwise target a devcontainer directly.
	if targetHost != "" && targetHost != "localhost" {
		for _, c := range containers {
			if c.Name == targetHost {
				jump := flagAlias
				a, _, rerr := resolveTarget(c, aliases, containers, &jump, interactive)
				return a, c.Name, rerr
			}
		}
	}

	var devcontainers []pick.Container
	for _, c := range containers {
		if c.Managed {
			devcontainers = append(devcontainers, c)
		}
	}
	if len(devcontainers) == 0 {
		return "", "", fmt.Errorf("no running devcontainers found. Start one with 'devcontainer-cli workspace up'")
	}
	target, perr := pickContainer(devcontainers, "Select a devcontainer to forward from:", pickContainerOptions{Interactive: interactive})
	if perr != nil {
		return "", "", perr
	}
	for _, a := range aliases {
		if a == target.Name {
			return a, target.Name, nil
		}
	}
	return "", "", fmt.Errorf("no SSH alias configured for '%s'. Run 'devcontainer-cli access ssh --setup' in that project first", target.Name)
}

func parsePort(value, label string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid %s: %s (must be 1-65535)", label, value)
	}
	return port, nil
}

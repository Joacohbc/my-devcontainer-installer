package service

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Tunnel describes a single port forward between the local machine and a
// container.
//
// In the forward direction (Reverse == false) the local machine listens on
// BindAddress:LocalPort (default 127.0.0.1:LocalPort) and forwards to
// TargetHost:ContainerPort inside the container network.
//
// In the reverse direction (Reverse == true) the container listens on
// 127.0.0.1:ContainerPort and forwards to LocalPort on this machine.
// Reverse is only valid when TargetHost is "localhost" (the container the SSH
// session connects to); ssh -R cannot bind on a container reached through a
// jump host.
type Tunnel struct {
	BindAddress    string // optional local bind IP (empty -> 127.0.0.1)
	LocalPort      int
	ContainerPort  int
	Reverse        bool
	TargetHost     string // "localhost" for the devcontainer itself, or another service's container name
	Alias          string // SSH host alias in ~/.ssh/devcontainer-cli.config
	ContainerName  string
	IsDevcontainer bool // true when ContainerName is a devcontainer-cli managed container
	Ephemeral      bool // true when bypassing ~/.ssh/config and dialing Target directly
	Target         EphemeralSshTarget
}

// EffectiveBindAddress returns the local listen IP for forward tunnels,
// defaulting to 127.0.0.1 when BindAddress is empty.
func (t Tunnel) EffectiveBindAddress() string {
	if addr := strings.TrimSpace(t.BindAddress); addr != "" {
		return addr
	}
	return "127.0.0.1"
}

// ForwardSpec returns the -L / -R argument passed to ssh(1).
//
// Forward (-L): [bind_address:]local_port:target_host:container_port.
// Reverse (-R): 127.0.0.1:container_port:127.0.0.1:local_port (explicit bind
// address on the container's loopback so sshd does not depend on
// GatewayPorts).
func (t Tunnel) ForwardSpec() string {
	target := t.TargetHost
	if target == "" {
		target = "localhost"
	}
	if t.Reverse {
		return fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", t.ContainerPort, t.LocalPort)
	}
	return fmt.Sprintf("%s:%d:%s:%d", t.EffectiveBindAddress(), t.LocalPort, target, t.ContainerPort)
}

// PortForwardService opens SSH port-forwarding tunnels (in foreground or background).
type PortForwardService struct {
	Report Reporter
}

// OpenTunnels runs a single foreground `ssh -N` session carrying all given
// tunnels. Because one ssh process owns every -L / -R flag, opening a tunnel
// to a devcontainer AND a sibling database container on its network happens in
// one connection.
func (s PortForwardService) OpenTunnels(tunnels []Tunnel) error {
	args, err := s.buildSshArgs(tunnels)
	if err != nil {
		return err
	}
	return runInteractive("ssh", args...)
}

// OpenTunnelsDetached starts an `ssh -N` session carrying all given tunnels in
// the background and returns a cleanup function that terminates it. Used by
// `access ssh --forward` so the tunnels stay up for the duration of the SSH
// session and tear down cleanly when it exits.
func (s PortForwardService) OpenTunnelsDetached(tunnels []Tunnel) (func(), error) {
	args, err := s.buildSshArgs(tunnels)
	if err != nil {
		return nil, err
	}
	return startBackground("ssh", args...)
}

// BuildEphemeralTunnel prepares a Tunnel for ephemeral forwarding: it sets
// Ephemeral = true, IsDevcontainer = true, and resolves the container's live
// target (ensuring the public key is authorized inside it). The caller fills in
// the port mapping and direction, and this fills in the connection. It goes
// through the same EnsureEphemeralAccess as an ephemeral session, so a
// container that never had `ssh --setup` run still accepts the tunnel instead
// of refusing the key.
func (s PortForwardService) BuildEphemeralTunnel(t Tunnel, user, keyPath string) (Tunnel, error) {
	return s.BuildEphemeralTunnelVia(t, user, keyPath, "")
}

// BuildEphemeralTunnelVia prepares a Tunnel for ephemeral forwarding, optionally
// jumping through a remote Docker host via SSH (`via`).
func (s PortForwardService) BuildEphemeralTunnelVia(t Tunnel, user, keyPath, via string) (Tunnel, error) {
	sshSvc := SshService{Report: s.Report}
	target, err := sshSvc.EnsureEphemeralAccess(t.ContainerName, user, keyPath, via)
	if err != nil {
		return Tunnel{}, err
	}
	t.IsDevcontainer = true
	t.Ephemeral = true
	t.Target = target
	return t, nil
}

func (s PortForwardService) buildSshArgs(tunnels []Tunnel) ([]string, error) {
	if len(tunnels) == 0 {
		return nil, fmt.Errorf("no tunnels specified")
	}

	first := tunnels[0]
	for _, t := range tunnels[1:] {
		if t.Ephemeral != first.Ephemeral {
			return nil, fmt.Errorf("cannot mix ephemeral and persistent tunnels in one session")
		}
		if first.Ephemeral {
			if t.Target.IP != first.Target.IP || t.Target.User != first.Target.User || t.Target.KeyPath != first.Target.KeyPath || t.Target.Via != first.Target.Via {
				return nil, fmt.Errorf("cannot combine tunnels targeting different containers in one ephemeral session")
			}
		} else if t.Alias != first.Alias {
			return nil, fmt.Errorf("cannot combine tunnels across different SSH aliases (%s vs %s) in one session", first.Alias, t.Alias)
		}
	}

	var args []string
	var dest string
	if first.Ephemeral {
		if first.Target.IP == "" {
			return nil, fmt.Errorf("ephemeral tunnel missing target IP")
		}
		if first.Target.User == "" {
			return nil, fmt.Errorf("ephemeral tunnel missing target user")
		}
		if first.Target.KeyPath == "" {
			return nil, fmt.Errorf("ephemeral tunnel missing target key path")
		}
		args = append(args, ephemeralSshBaseArgs(first.Target.KeyPath, first.Target.Via)...)
		dest = fmt.Sprintf("%s@%s", first.Target.User, first.Target.IP)
	} else {
		if first.Alias == "" {
			return nil, fmt.Errorf("tunnel missing SSH alias")
		}
		dest = first.Alias
	}

	args = append(args, "-N", "-o", "ExitOnForwardFailure=yes")
	for _, t := range tunnels {
		if t.Reverse {
			args = append(args, "-R", t.ForwardSpec())
		} else {
			args = append(args, "-L", t.ForwardSpec())
		}
	}
	args = append(args, dest)
	return args, nil
}

// CheckPortAvailable reports whether the given local TCP port can be bound on
// 127.0.0.1. Used before opening a tunnel so a collision produces a clear
// message instead of a silent ssh exit.
func CheckPortAvailable(port int) error {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("port %d is already in use on 127.0.0.1", port)
	}
	_ = ln.Close()
	return nil
}

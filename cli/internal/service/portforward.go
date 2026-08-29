package service

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

// PortForwardService opens SSH tunnels into containers. The cli resolves the
// containers, aliases and ports interactively; this service owns spawning and
// supervising the ssh processes.
type PortForwardService struct {
	Report Reporter
}

// tunnelBindAddress keeps a forwarded port on the loopback: a tunnel is for this
// machine, not for everything that can reach it.
const tunnelBindAddress = "127.0.0.1"

// Tunnel is one resolved port forward over an SSH alias or direct ephemeral
// connection. LocalPort is always the port on this machine and ContainerPort
// the one inside the container; Reverse is what decides which of the two
// listens and which is dialed.
type Tunnel struct {
	LocalPort     int
	ContainerPort int
	// Reverse turns the tunnel around: instead of listening on LocalPort here
	// and dialing TargetHost:ContainerPort from inside the container (ssh -L),
	// the container listens on ContainerPort and the connection is dialed to
	// TargetHost:LocalPort from this machine (ssh -R). That is how a service
	// running on the host — a database, an API, another devcontainer's
	// published port — becomes reachable inside the container.
	Reverse        bool
	TargetHost     string
	Alias          string
	ContainerName  string
	IsDevcontainer bool
	Ephemeral      bool
	// Target is only meaningful when Ephemeral: an aliased tunnel gets its
	// address, user and key from the managed Host block instead.
	Target EphemeralTarget
}

// BuildEphemeralTunnel completes t into an ephemeral direct SSH tunnel into
// t.ContainerName: the caller decides the ports, the target host and the
// direction, and this fills in the connection. It goes through the same
// EnsureEphemeralAccess as an ephemeral session, so a container that never had
// `ssh --setup` run still accepts the tunnel instead of refusing the key.
func (s PortForwardService) BuildEphemeralTunnel(t Tunnel, user, keyPath string) (Tunnel, error) {
	sshSvc := SshService{Report: s.Report}
	target, err := sshSvc.EnsureEphemeralAccess(t.ContainerName, user, keyPath)
	if err != nil {
		return Tunnel{}, err
	}

	t.IsDevcontainer = true
	t.Ephemeral = true
	t.Target = target
	return t, nil
}

// tunnelCommand builds the `ssh -N -L` invocation for one tunnel.
func tunnelCommand(t Tunnel) *exec.Cmd {
	if t.Ephemeral {
		args := []string{"-N", t.forwardFlag(), t.forwardSpec()}
		args = append(args, sshdefaults.EphemeralDialArgs(t.Target.KeyPath)...)
		args = append(args, t.Target.Destination())
		return exec.Command("ssh", args...)
	}
	return exec.Command("ssh", "-N", t.forwardFlag(), t.forwardSpec(), t.Alias)
}

// forwardFlag picks the ssh forwarding mode: -L listens here, -R listens inside
// the container.
func (t Tunnel) forwardFlag() string {
	if t.Reverse {
		return "-R"
	}
	return "-L"
}

// forwardSpec renders ssh's -L/-R argument. The bind address is explicit: an
// omitted one makes ssh listen on both 127.0.0.1 and ::1, which we don't want.
// A reverse tunnel binds inside the container, where sshd's default
// GatewayPorts=no already confines the listener to loopback — naming it keeps
// the two specs symmetrical and the intent readable.
func (t Tunnel) forwardSpec() string {
	if t.Reverse {
		return fmt.Sprintf("%s:%d:%s:%d", tunnelBindAddress, t.ContainerPort, t.TargetHost, t.LocalPort)
	}
	return fmt.Sprintf("%s:%d:%s:%d", tunnelBindAddress, t.LocalPort, t.TargetHost, t.ContainerPort)
}

// Description renders the tunnel as an arrow between the listening side and the
// dialed one, for logs and error messages.
func (t Tunnel) Description() string {
	if t.Reverse {
		return fmt.Sprintf("container:%d→%s:%d", t.ContainerPort, t.TargetHost, t.LocalPort)
	}
	return fmt.Sprintf("%d→%s:%d", t.LocalPort, t.TargetHost, t.ContainerPort)
}

// OpenTunnels spawns one `ssh -N -L` process per tunnel in parallel and blocks
// until they all close. The first failure tears down the rest and is returned.
func (s PortForwardService) OpenTunnels(tunnels []Tunnel) error {
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
		c := tunnelCommand(t)
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
			if err := cmd.Wait(); err != nil {
				mu.Lock()
				if firstErr == nil {
					target := tun.Alias
					if tun.Ephemeral {
						target = "ephemeral"
					}
					firstErr = fmt.Errorf("SSH tunnel %s (%s) closed: %v", tun.Description(), target, err)
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
	s.Report.Success("\nAll SSH tunnels closed.\n")
	return nil
}

// OpenTunnelsDetached starts one `ssh -N -L` process per tunnel without
// waiting for them, returning a stop function that kills them all. Use this
// instead of OpenTunnels when the tunnels should run alongside another
// foreground process (e.g. an interactive 'ssh' session) rather than owning
// the terminal themselves; a failed start tears down the tunnels already
// spawned before returning the error.
func (s PortForwardService) OpenTunnelsDetached(tunnels []Tunnel) (stop func(), err error) {
	cmds := make([]*exec.Cmd, 0, len(tunnels))
	stopAll := func() {
		for _, c := range cmds {
			if c.Process != nil {
				_ = c.Process.Kill()
			}
		}
	}

	for _, t := range tunnels {
		c := tunnelCommand(t)
		if err := c.Start(); err != nil {
			stopAll()
			return func() {}, fmt.Errorf("failed to spawn SSH tunnel: %w", err)
		}
		cmds = append(cmds, c)
	}
	return stopAll, nil
}

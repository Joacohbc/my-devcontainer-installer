package service

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// PortForwardService opens SSH tunnels into containers. The cli resolves the
// containers, aliases and ports interactively; this service owns spawning and
// supervising the ssh processes.
type PortForwardService struct {
	Report Reporter
}

// Tunnel is one resolved local→container port forward over an SSH alias.
type Tunnel struct {
	LocalPort      int
	ContainerPort  int
	TargetHost     string
	Alias          string
	ContainerName  string
	IsDevcontainer bool
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
		// Bind address is explicit: an omitted bind address makes ssh listen on
		// both the IPv4 and IPv6 loopback (127.0.0.1 and ::1), which we don't want.
		c := exec.Command("ssh", "-N", "-L", fmt.Sprintf("127.0.0.1:%d:%s:%d", t.LocalPort, t.TargetHost, t.ContainerPort), t.Alias)
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
					firstErr = fmt.Errorf("SSH tunnel %d→%s:%d (%s) closed: %v", tun.LocalPort, tun.TargetHost, tun.ContainerPort, tun.Alias, err)
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

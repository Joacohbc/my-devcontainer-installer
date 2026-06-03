package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
)

// Process operations are package vars so tests can swap them via SetForwardOps
// (mirrors docker.SetRunner) and avoid spawning or signalling real processes.
var (
	spawnForward = spawnForwardImpl
	processAlive = processAliveImpl
	killProcess  = killProcessImpl
)

// SetForwardOps overrides the process spawn/liveness/kill operations. A nil
// argument leaves that operation untouched. Pair with ResetForwardOps in tests.
func SetForwardOps(spawn func(args []string, logPath string) (int, error), alive func(int) bool, kill func(int) error) {
	if spawn != nil {
		spawnForward = spawn
	}
	if alive != nil {
		processAlive = alive
	}
	if kill != nil {
		killProcess = kill
	}
}

// ResetForwardOps restores the real process operations.
func ResetForwardOps() {
	spawnForward = spawnForwardImpl
	processAlive = processAliveImpl
	killProcess = killProcessImpl
}

// spawnForwardImpl starts a detached `ssh <args>` process writing its output to
// logPath and returns its pid. The process keeps running after the CLI exits.
func spawnForwardImpl(args []string, logPath string) (int, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = detachAttr()
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to spawn SSH process: %w", err)
	}
	pid := cmd.Process.Pid
	// Release the child so it is not reaped/tied to this process.
	_ = cmd.Process.Release()
	return pid, nil
}

func sshForwardArgs(localPort int, targetHost string, containerPort int, alias string) []string {
	return []string{"-N", "-L", fmt.Sprintf("%d:%s:%d", localPort, targetHost, containerPort), alias}
}

// resolveForward fills in the defaults for a configured forward: alias defaults
// to the workspace SSH host, target host to localhost.
func resolveForward(f types.PortForward, workspace string) (alias, targetHost string) {
	alias = f.Alias
	if alias == "" {
		alias = workspace
	}
	targetHost = f.TargetHost
	if targetHost == "" {
		targetHost = "localhost"
	}
	return alias, targetHost
}

// StartConfigured spawns a detached SSH tunnel for each configured forward that
// is not already running, persists the registry under .dc_<workspace>/, and
// returns the forwards it newly started.
func (s PortForwardService) StartConfigured(cwd, workspace string, forwards []types.PortForward) ([]domain.RunningForward, error) {
	if len(forwards) == 0 {
		return nil, nil
	}
	paths := project.ProjectPaths(cwd, workspace)
	existing, err := domain.LoadForwardState(paths.ForwardStateFile)
	if err != nil {
		return nil, err
	}

	alive := map[string]bool{}
	registry := make([]domain.RunningForward, 0, len(existing)+len(forwards))
	for _, rf := range existing {
		if processAlive(rf.PID) {
			alive[rf.ID] = true
			registry = append(registry, rf)
		}
	}

	if err := os.MkdirAll(paths.ForwardLogDir, 0755); err != nil {
		return nil, err
	}

	var started []domain.RunningForward
	for _, f := range forwards {
		id := domain.ForwardID(workspace, f.LocalPort)
		if alive[id] {
			s.Report.Info("Tunnel %s already running; skipping.", id)
			continue
		}
		aliasName, targetHost := resolveForward(f, workspace)
		logPath := filepath.Join(paths.ForwardLogDir, id+".log")
		pid, serr := spawnForward(sshForwardArgs(f.LocalPort, targetHost, f.ContainerPort, aliasName), logPath)
		if serr != nil {
			s.Report.Warn("Failed to start tunnel %s: %v", id, serr)
			continue
		}
		rf := domain.RunningForward{
			ID:            id,
			PID:           pid,
			LocalPort:     f.LocalPort,
			ContainerPort: f.ContainerPort,
			TargetHost:    targetHost,
			Alias:         aliasName,
			StartedAt:     time.Now().Format(time.RFC3339),
		}
		registry = append(registry, rf)
		started = append(started, rf)
		s.Report.Success("Tunnel %s up (pid %d): localhost:%d → %s:%d via %s", id, pid, f.LocalPort, targetHost, f.ContainerPort, aliasName)
	}

	if err := domain.SaveForwardState(paths.ForwardStateFile, registry); err != nil {
		return started, err
	}
	return started, nil
}

// ListForwards returns the project's running forwards, pruning dead entries from
// the registry on disk.
func (s PortForwardService) ListForwards(cwd, workspace string) ([]domain.RunningForward, error) {
	paths := project.ProjectPaths(cwd, workspace)
	existing, err := domain.LoadForwardState(paths.ForwardStateFile)
	if err != nil {
		return nil, err
	}
	alive := make([]domain.RunningForward, 0, len(existing))
	for _, rf := range existing {
		if processAlive(rf.PID) {
			alive = append(alive, rf)
		}
	}
	if len(alive) != len(existing) {
		if err := domain.SaveForwardState(paths.ForwardStateFile, alive); err != nil {
			return alive, err
		}
	}
	return alive, nil
}

// StopForward kills the forward with the given id and drops it from the
// registry. It errors if no forward with that id is registered.
func (s PortForwardService) StopForward(cwd, workspace, id string) error {
	paths := project.ProjectPaths(cwd, workspace)
	existing, err := domain.LoadForwardState(paths.ForwardStateFile)
	if err != nil {
		return err
	}
	remaining := make([]domain.RunningForward, 0, len(existing))
	found := false
	for _, rf := range existing {
		if rf.ID == id {
			found = true
			if processAlive(rf.PID) {
				if kerr := killProcess(rf.PID); kerr != nil {
					s.Report.Warn("Failed to kill pid %d for %s: %v", rf.PID, id, kerr)
				}
			}
			continue
		}
		remaining = append(remaining, rf)
	}
	if !found {
		return fmt.Errorf("no running forward with id %q", id)
	}
	if err := domain.SaveForwardState(paths.ForwardStateFile, remaining); err != nil {
		return err
	}
	s.Report.Success("Stopped tunnel %s.", id)
	return nil
}

// StopAllForwards kills every registered forward for the project and clears the
// registry. It returns how many live processes it killed. A project with no
// registry is a no-op (used by down/destroy cleanup).
func (s PortForwardService) StopAllForwards(cwd, workspace string) (int, error) {
	paths := project.ProjectPaths(cwd, workspace)
	existing, err := domain.LoadForwardState(paths.ForwardStateFile)
	if err != nil {
		return 0, err
	}
	killed := 0
	for _, rf := range existing {
		if !processAlive(rf.PID) {
			continue
		}
		if kerr := killProcess(rf.PID); kerr != nil {
			s.Report.Warn("Failed to kill pid %d for %s: %v", rf.PID, rf.ID, kerr)
			continue
		}
		killed++
	}
	if err := domain.SaveForwardState(paths.ForwardStateFile, nil); err != nil {
		return killed, err
	}
	return killed, nil
}

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
		c := exec.Command("ssh", "-N", "-L", fmt.Sprintf("%d:%s:%d", t.LocalPort, t.TargetHost, t.ContainerPort), t.Alias)
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

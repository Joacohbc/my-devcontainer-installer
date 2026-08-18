package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/osutil"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

// SshService owns the external-command and docker orchestration behind the
// setup-ssh flow (key generation, container/network inspection, key install,
// connectivity test). The cli drives the interactive prompts and output.
type SshService struct {
	Report Reporter
}

// CommandExists reports whether bin is on PATH.
func (s SshService) CommandExists(bin string) bool {
	return osutil.CommandExists(bin)
}

// ContainerRunning reports whether a container with the exact name is running.
func (s SshService) ContainerRunning(name string) bool {
	inspectSvc := InspectService{Report: s.Report}
	containers, err := inspectSvc.ListContainers(false)
	if err != nil {
		return false
	}
	for _, c := range containers {
		if c.Name == name && c.State == string(StateRunning) {
			return true
		}
	}
	return false
}

// TargetState is a managed SSH target's liveness, resolved deterministically
// from the owning docker daemon by container name — no SSH round-trip.
type TargetState int

const (
	// TargetRunning: the container exists and is running.
	TargetRunning TargetState = iota
	// TargetStopped: the container exists but is not running.
	TargetStopped
	// TargetAbsent: no container of that name exists on the daemon.
	TargetAbsent
)

// ContainerLiveness reports a container's state by name, read from whatever
// docker daemon is currently in scope — the local one, or a --via daemon when
// called under WithHostOverride. When running it also returns the container's
// first network address, so a caller can refresh a block that dials by IP. A
// non-nil error means the daemon itself could not be queried (an unreachable
// --via host), which is distinct from a reachable daemon that has no such
// container (TargetAbsent, nil): the first must not be treated as stale.
func (s SshService) ContainerLiveness(name string) (state TargetState, ip string, err error) {
	inspectSvc := InspectService{Report: s.Report}
	containers, err := inspectSvc.ListContainers(false)
	if err != nil {
		return TargetAbsent, "", err
	}
	for _, c := range containers {
		if c.Name != name {
			continue
		}
		if c.State != string(StateRunning) {
			return TargetStopped, "", nil
		}
		ips, ipErr := inspectSvc.ContainerNetworkIPs(name)
		if ipErr != nil || len(ips) == 0 {
			return TargetRunning, "", nil
		}
		return TargetRunning, ips[0].IP, nil
	}
	return TargetAbsent, "", nil
}

// ComposeUp brings the stack up detached.
func (s SshService) ComposeUp(composeFile string) error {
	status, err := docker.DockerCompose(composeFile, []string{"up", "-d"}, nil)
	if err != nil || status != 0 {
		return fmt.Errorf("docker compose up failed")
	}
	return nil
}

// GenerateKey creates an ed25519 keypair at keyPath.
func (s SshService) GenerateKey(keyPath string) error {
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return err
	}
	c := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-q")
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("ssh-keygen failed")
	}
	return nil
}

// EnsureKey makes the managed key exist at keyPath, generating it once. It
// returns created=false (and no error) when both keyPath and keyPath+".pub"
// already exist; otherwise it generates the pair and returns created=true.
func (s SshService) EnsureKey(keyPath string) (created bool, err error) {
	if fileExists(keyPath) && fileExists(keyPath+".pub") {
		return false, nil
	}
	if err := s.GenerateKey(keyPath); err != nil {
		return false, err
	}
	return true, nil
}

// PublicKey reads the public half of the key pair at keyPath (keyPath+".pub").
func (s SshService) PublicKey(keyPath string) ([]byte, error) {
	return os.ReadFile(keyPath + ".pub")
}

// PrivateKey reads the private key at keyPath.
func (s SshService) PrivateKey(keyPath string) ([]byte, error) {
	return os.ReadFile(keyPath)
}

// ContainerIPs inspects a container and returns its (network, ip) pairs, one per
// attached network with a non-empty address.
func (s SshService) ContainerIPs(container string) ([]NetworkIP, error) {
	inspectSvc := InspectService{Report: s.Report}
	ips, err := inspectSvc.ContainerNetworkIPs(container)
	if err != nil {
		return nil, fmt.Errorf("could not resolve container IP")
	}
	return ips, nil
}

// InstallKeySpec holds the parameters for installing a public key inside a container.
type InstallKeySpec struct {
	PublicKey []byte
	User      string
	Container string
	Script    string
}

// InstallKeyLocal pipes the public key into the container and runs the install
// script via `docker exec`.
func (s SshService) InstallKeyLocal(spec InstallKeySpec) error {
	args := []string{"exec", "-i", "-u", spec.User, spec.Container, "sh", "-c", spec.Script}
	status, err := docker.DockerExecStdin(spec.PublicKey, args)
	if err != nil || status != 0 {
		return fmt.Errorf("docker exec key install failed")
	}
	return nil
}

// LiveTargetPredicates builds the existence checks clean-ssh needs: whether a
// managed SSH block's workspace or container target still exists. It requires Docker
// (the container check queries the daemon). A workspace counts as alive when a
// managed container reports it OR the image registry still records it, so a merely
// stopped ("down") stack whose project is still on disk is not treated as gone; a
// container counts as alive when a container of that name exists in any state.
//
// The third predicate, existsRemoteContainer, is what a --via block's container
// (marker.Host != "") must be checked with instead — it lives on a different
// Docker daemon, invisible to the local existsContainer above. It dials each
// distinct host at most once per call (cached) and returns (alive, verified):
// when host can't be reached, alive is true (fail open) but verified is
// false, so PruneManagedBlocks can report it separately rather than either
// deleting or silently hiding it.
func (s SshService) LiveTargetPredicates() (existsWorkspace, existsContainer func(string) bool, existsRemoteContainer func(host, name string) (alive, verified bool), err error) {
	if err := docker.EnsureDocker(); err != nil {
		return nil, nil, nil, err
	}
	inspect := InspectService{Report: s.Report}
	all, err := inspect.ListContainers(false)
	if err != nil {
		return nil, nil, nil, err
	}
	containerNames := make(map[string]bool, len(all))
	workspaces := make(map[string]bool)
	for _, c := range all {
		containerNames[c.Name] = true
		if c.Managed && c.Workspace != "" {
			workspaces[c.Workspace] = true
		}
	}
	for _, e := range domain.LoadRegistry() {
		if e.Workspace != "" {
			workspaces[e.Workspace] = true
		}
	}

	remoteCache := make(map[string]map[string]bool)
	existsRemoteContainer = func(host, name string) (alive, verified bool) {
		names, cached := remoteCache[host]
		if !cached {
			names = map[string]bool{}
			reachable := true
			werr := WithHostOverride(host, func() error {
				remote, lerr := (InspectService{Report: s.Report}).ListContainers(false)
				if lerr != nil {
					reachable = false
					return nil
				}
				for _, c := range remote {
					names[c.Name] = true
				}
				return nil
			})
			if werr != nil || !reachable {
				s.Report.Warn("Could not reach '%s' to check its containers.", host)
				remoteCache[host] = nil
				return true, false
			}
			remoteCache[host] = names
		}
		if names == nil {
			return true, false
		}
		return names[name], true
	}

	return func(ws string) bool { return workspaces[ws] },
		func(name string) bool { return containerNames[name] },
		existsRemoteContainer,
		nil
}

// SSHTestResult is the outcome of a connectivity probe.
type SSHTestResult int

const (
	SSHTestOK SSHTestResult = iota
	SSHTestTimeout
	SSHTestInconclusive
)

// TestConnection runs a non-interactive `ssh <alias> echo OK`, reporting whether
// it succeeded, timed out, or was inconclusive.
func (s SshService) TestConnection(alias string) SSHTestResult {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	probeArgs := append(sshdefaults.OptionFlags(sshdefaults.ProbeOptions()), alias, "echo OK")
	c := exec.CommandContext(ctx, "ssh", probeArgs...)
	out, err := c.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return SSHTestTimeout
	}
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == "OK" {
				return SSHTestOK
			}
		}
	}
	return SSHTestInconclusive
}

// Connect opens a real `ssh <alias>` session (or runs the given command over
// SSH when args is non-empty), inheriting the terminal. Mirrors
// InspectService.Shell's exit-code contract: a non-zero remote exit becomes an
// error carrying that code, so callers don't need to special-case it.
func (s SshService) Connect(alias string, args []string) error {
	c := exec.Command("ssh", append([]string{alias}, args...)...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := c.Run()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("ssh session exited with code %d", exitErr.ExitCode())
	}
	return fmt.Errorf("failed to run ssh: %w", err)
}

// EphemeralTarget is a container resolved for a direct SSH dial: where to
// connect, as whom, and with which key — already trusted there.
type EphemeralTarget struct {
	IP      string
	User    string
	KeyPath string
}

// Destination renders the target as ssh's user@host argument.
func (t EphemeralTarget) Destination() string {
	return fmt.Sprintf("%s@%s", t.User, t.IP)
}

// noNetworkIPFallback is what an ephemeral dial falls back to when the daemon
// reports no address for the container: a container published on the host is
// still reachable on the loopback.
const noNetworkIPFallback = "127.0.0.1"

// EnsureEphemeralAccess resolves an ephemeral target: it checks the container is
// running, finds the address to dial, and makes the managed key usable against it.
//
// Installing the public key is part of resolving the target, not an extra step.
// Nothing in the image ships an authorized_keys — only the setup flow writes one
// — so an ephemeral connection into a container that never had `ssh --setup` run
// would always fail with "Permission denied (publickey)". "Ephemeral" means the
// local ~/.ssh/config is left alone; it cannot mean the target has no way to
// authenticate us. The install script is idempotent (it appends then sort -u's),
// so repeating it on a container that already trusts the key is a no-op.
func (s SshService) EnsureEphemeralAccess(containerName, user, keyPath string) (EphemeralTarget, error) {
	state, ip, err := s.ContainerLiveness(containerName)
	if err != nil {
		return EphemeralTarget{}, err
	}
	if state == TargetAbsent {
		return EphemeralTarget{}, fmt.Errorf("container '%s' not found", containerName)
	}
	if state == TargetStopped {
		return EphemeralTarget{}, fmt.Errorf("container '%s' is not running; start it with 'devcontainer-cli start'", containerName)
	}
	if ip == "" {
		ip = noNetworkIPFallback
	}
	if user == "" {
		user = sshdefaults.User
	}
	target := EphemeralTarget{IP: ip, User: user, KeyPath: domain.ResolveSSHKeyPath(keyPath)}

	if err := s.trustManagedKey(containerName, target); err != nil {
		return EphemeralTarget{}, err
	}
	return target, nil
}

// trustManagedKey generates the managed key if it is missing and installs its
// public half in the container's authorized_keys.
func (s SshService) trustManagedKey(containerName string, target EphemeralTarget) error {
	if _, err := s.EnsureKey(target.KeyPath); err != nil {
		return err
	}
	publicKey, err := s.PublicKey(target.KeyPath)
	if err != nil {
		return err
	}
	return s.InstallKeyLocal(InstallKeySpec{
		PublicKey: publicKey,
		User:      target.User,
		Container: containerName,
		Script:    sshdefaults.AuthorizedKeysInstallScript(),
	})
}

// ConnectEphemeral opens an SSH session without writing to ~/.ssh/config or
// resolving a managed alias, dialing the container's own address directly.
func (s SshService) ConnectEphemeral(containerName, user, keyPath string, args []string) error {
	target, err := s.EnsureEphemeralAccess(containerName, user, keyPath)
	if err != nil {
		return err
	}

	sshArgs := append(sshdefaults.EphemeralDialArgs(target.KeyPath), target.Destination())
	sshArgs = append(sshArgs, args...)

	c := exec.Command("ssh", sshArgs...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	runErr := c.Run()
	if runErr == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return fmt.Errorf("ssh session exited with code %d", exitErr.ExitCode())
	}
	return fmt.Errorf("failed to run ssh: %w", runErr)
}

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
func (s SshService) LiveTargetPredicates() (existsWorkspace, existsContainer func(string) bool, err error) {
	if err := docker.EnsureDocker(); err != nil {
		return nil, nil, err
	}
	inspect := InspectService{Report: s.Report}
	all, err := inspect.ListContainers(false)
	if err != nil {
		return nil, nil, err
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
	return func(ws string) bool { return workspaces[ws] },
		func(name string) bool { return containerNames[name] },
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
	c := exec.CommandContext(ctx, "ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=5",
		"-o", "ServerAliveInterval=2",
		"-o", "ServerAliveCountMax=2",
		alias, "echo OK",
	)
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

// sshConnFailureCode is the exit status ssh itself returns when the connection
// fails (host unreachable, auth rejected, connection dropped) — as opposed to a
// remote command's own exit status, which ssh passes through verbatim.
const sshConnFailureCode = 255

// Connect opens a real `ssh <alias>` session (or runs the given command over SSH
// when args is non-empty), inheriting the terminal.
//
// Exit handling differs by mode, because an interactive shell's exit status is
// not the CLI's business:
//   - Interactive (args empty): closing the session is a normal outcome even
//     when the remote login shell exits non-zero (e.g. the last command you ran
//     returned 1, or you Ctrl+C'd it → 130). Such an exit returns nil. Only
//     ssh's own connection failure (code 255) is surfaced as an error.
//   - Command mode (args non-empty): the remote exit status is meaningful for
//     scripting, so any non-zero code (including 255) is returned as an error.
func (s SshService) Connect(alias string, args []string) error {
	c := exec.Command("ssh", append([]string{alias}, args...)...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := c.Run()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return fmt.Errorf("failed to run ssh: %w", err)
	}
	code := exitErr.ExitCode()
	if len(args) == 0 {
		// Interactive session: only a genuine ssh-level failure is an error;
		// the user closing a shell whose last command failed is not.
		if code == sshConnFailureCode {
			return fmt.Errorf("ssh connection failed (exit %d)", code)
		}
		return nil
	}
	return fmt.Errorf("ssh command exited with code %d", code)
}

package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
	containers, err := inspectSvc.ListContainers(false, false)
	if err != nil {
		return false
	}
	for _, c := range containers {
		if c.Name == name {
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

// ServiceLogs returns the combined stdout+stderr logs for a compose service and
// whether the read succeeded.
func (s SshService) ServiceLogs(composeFile, service string) (string, bool) {
	status, stdout, stderr, err := docker.DockerCapture([]string{"compose", "-f", composeFile, "logs", service})
	if err != nil || status != 0 {
		return "", false
	}
	return stdout + "\n" + stderr, true
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

// PasswordFromLogs scans the compose service logs for the last line announcing
// the temporary password for user, returning the line and whether one was found.
func (s SshService) PasswordFromLogs(composeFile, service, user string) (string, bool) {
	combined, ok := s.ServiceLogs(composeFile, service)
	if !ok {
		return "", false
	}
	var last string
	for _, line := range strings.Split(combined, "\n") {
		if strings.Contains(line, user+" password") {
			last = line
		}
	}
	return last, last != ""
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

package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// SshService owns the external-command and docker orchestration behind the
// setup-ssh flow (key generation, container/network inspection, key install,
// connectivity test). The cli drives the interactive prompts and output.
type SshService struct {
	Report Reporter
}

// CommandExists reports whether bin is on PATH.
func (s SshService) CommandExists(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// ContainerRunning reports whether a container with the exact name is running.
func (s SshService) ContainerRunning(name string) bool {
	status, stdout, _, err := docker.DockerCapture([]string{"ps", "--format", "{{.Names}}"})
	if err != nil || status != 0 {
		return false
	}
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) == name {
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

// ContainerNetworks returns lines of "<network> <ip>" for a container.
func (s SshService) ContainerNetworks(container string) (string, error) {
	format := `{{range $name, $net := .NetworkSettings.Networks}}{{$name}} {{$net.IPAddress}}{{"\n"}}{{end}}`
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", format, container})
	if err != nil || status != 0 {
		return "", fmt.Errorf("could not resolve container IP")
	}
	return stdout, nil
}

// InstallKeyLocal pipes the public key into the container and runs the install
// script via `docker exec`.
func (s SshService) InstallKeyLocal(pub []byte, user, container, script string) error {
	args := []string{"exec", "-i", "-u", user, container, "sh", "-c", script}
	status, err := docker.DockerExecStdin(pub, args)
	if err != nil || status != 0 {
		return fmt.Errorf("docker exec key install failed")
	}
	return nil
}

// InstallKeyRemote installs the public key over SSH on a remote docker host.
func (s SshService) InstallKeyRemote(remote, user, container, script string, pub []byte) error {
	remoteCmd := fmt.Sprintf("docker exec -i -u %s %s sh -c '%s'", user, container, script)
	c := exec.Command("ssh", remote, remoteCmd)
	c.Stdin = bytes.NewReader(pub)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("remote key install failed")
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

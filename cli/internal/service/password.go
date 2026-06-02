package service

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// PasswordService manages devuser password operations inside a running container.
type PasswordService struct {
	Report Reporter
}

// ShowPassword returns the /etc/shadow entry for devuser, which encodes the
// password hash, last-change date, and aging fields.
func (s PasswordService) ShowPassword(container string) (string, error) {
	if err := s.ensureRunning(container); err != nil {
		return "", err
	}
	status, stdout, _, err := docker.DockerCapture([]string{
		"exec", container, "getent", "shadow", "devuser",
	})
	if err != nil {
		return "", fmt.Errorf("docker exec failed: %w", err)
	}
	if status != 0 {
		return "", fmt.Errorf("getent shadow failed (exit %d) — is devuser defined in /etc/shadow?", status)
	}
	return strings.TrimSpace(stdout), nil
}

// ChangePassword changes the devuser password interactively (stdin/stdout
// inherited from the terminal). Requires the container to be running.
func (s PasswordService) ChangePassword(container string) error {
	if err := s.ensureRunning(container); err != nil {
		return err
	}
	exitCode, err := docker.DockerInherit([]string{"exec", "-it", container, "passwd", "devuser"})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("passwd exited with code %d", exitCode)
	}
	return nil
}

// ChangePasswordNonInteractive sets the devuser password without a prompt by
// piping "devuser:<password>" through chpasswd.
func (s PasswordService) ChangePasswordNonInteractive(container, password string) error {
	if password == "" {
		return fmt.Errorf("password must not be empty")
	}
	if err := s.ensureRunning(container); err != nil {
		return err
	}
	input := []byte("devuser:" + password + "\n")
	exitCode, err := docker.DockerExecStdin(input, []string{"exec", "-i", container, "chpasswd"})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("chpasswd exited with code %d", exitCode)
	}
	s.Report.Success("Password changed successfully.")
	return nil
}

func (s PasswordService) ensureRunning(container string) error {
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", "{{.State.Status}}", container})
	if err != nil || status != 0 {
		return fmt.Errorf("container '%s' not found", container)
	}
	if strings.TrimSpace(stdout) != "running" {
		return fmt.Errorf("container '%s' is not running. Run 'devcontainer-cli' or 'devcontainer-cli start' first", container)
	}
	return nil
}

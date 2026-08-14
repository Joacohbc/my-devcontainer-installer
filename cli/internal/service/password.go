package service

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

const initialPasswordFile = types.DevUserHome + "/initial_password.txt"

// PasswordService manages devuser password operations inside a running container.
type PasswordService struct {
	Report Reporter
}

// InitialPassword returns the auto-generated devuser password captured at first
// boot in initialPasswordFile.
//
// This is the only password the CLI can reveal: Linux stores credentials as a
// one-way hash, so a password later set via ChangePassword cannot be recovered.
func (s PasswordService) InitialPassword(container string) (string, error) {
	if err := s.ensureRunning(container); err != nil {
		return "", err
	}
	status, stdout, _, err := docker.DockerCapture([]string{
		"exec", container, "cat", initialPasswordFile,
	})
	if err != nil {
		return "", fmt.Errorf("docker exec failed: %w", err)
	}
	if status != 0 {
		return "", fmt.Errorf("%s not found — the initial password is unavailable (it may have been changed since the container was created)", initialPasswordFile)
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
	inspectSvc := InspectService{Report: s.Report}
	state, err := inspectSvc.ContainerState(container)
	if err != nil {
		return fmt.Errorf("container '%s' not found. Ensure it is running", container)
	}
	if state != StateRunning {
		return fmt.Errorf("container '%s' is not running. Start it with 'devcontainer-cli up', or create the project first with 'devcontainer-cli agent create'", container)
	}
	return nil
}

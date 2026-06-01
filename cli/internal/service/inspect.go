package service

import (
	"fmt"
	"os"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// InspectService runs container-targeted operations: shell/exec, logs, copy,
// directory listing and state queries. The cli resolves which container and
// renders tables; this service owns every docker call.
type InspectService struct {
	Report Reporter
}

// EnsureDocker reports an error if the docker daemon is unavailable.
func (s InspectService) EnsureDocker() error { return docker.EnsureDocker() }

// ContainerState returns the container's State.Status (e.g. "running"), or an
// error if it cannot be inspected.
func (s InspectService) ContainerState(name string) (string, error) {
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", "{{.State.Status}}", name})
	if err != nil || status != 0 {
		return "", fmt.Errorf("container '%s' not found", name)
	}
	return strings.TrimSpace(stdout), nil
}

// ContainerImage returns the container's configured image, or "" if unknown.
func (s InspectService) ContainerImage(name string) string {
	_, stdout, _, _ := docker.DockerCapture([]string{"inspect", "-f", "{{.Config.Image}}", name})
	return strings.TrimSpace(stdout)
}

func (s InspectService) ensureRunning(name string) error {
	state, err := s.ContainerState(name)
	if err != nil || state != "running" {
		return fmt.Errorf("container '%s' is not running. Run 'devcontainer-cli' or 'devcontainer-cli start' first", name)
	}
	return nil
}

// Shell opens an interactive shell (or runs command) in the running container.
func (s InspectService) Shell(name, user string, command []string) error {
	if err := s.ensureRunning(name); err != nil {
		return err
	}
	execArgs := []string{"exec", "-it"}
	if user != "" {
		execArgs = append(execArgs, "-u", user)
	}
	execArgs = append(execArgs, name)
	if len(command) > 0 {
		execArgs = append(execArgs, command...)
	} else {
		execArgs = append(execArgs, "sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi")
	}
	exitCode, err := docker.DockerInherit(execArgs)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("shell session exited with code %d", exitCode)
	}
	return nil
}

// Ls runs `ls` inside the running container at path.
func (s InspectService) Ls(name, path string, all, long bool) error {
	if err := s.ensureRunning(name); err != nil {
		return err
	}
	execArgs := []string{"exec", "-it", name, "ls"}
	if long {
		execArgs = append(execArgs, "-l")
	}
	if all {
		execArgs = append(execArgs, "-a")
	}
	execArgs = append(execArgs, "--color=auto", path)
	exitCode, err := docker.DockerInherit(execArgs)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("ls failed with exit code %d", exitCode)
	}
	return nil
}

// Copy copies a local file/dir into the running container.
func (s InspectService) Copy(name, localPath, containerPath string) error {
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return fmt.Errorf("local path '%s' does not exist", localPath)
	}
	if err := s.ensureRunning(name); err != nil {
		return err
	}
	s.Report.Warn("\nCopying '%s' to '%s' in container '%s'...", localPath, containerPath, name)
	exitCode, err := docker.DockerInherit([]string{"cp", localPath, name + ":" + containerPath})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("copy failed with exit code %d", exitCode)
	}
	s.Report.Success("\nSuccessfully copied.\n")
	return nil
}

// ContainerLogs streams `docker logs` for a single container.
func (s InspectService) ContainerLogs(name string, follow bool, tail string) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if tail != "" {
		args = append(args, "--tail", tail)
	}
	args = append(args, name)
	status, err := docker.DockerInherit(args)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("docker logs failed with exit code %d", status)
	}
	return nil
}

// ComposeLogs streams `docker compose logs` for the project, optionally limited
// to the given services.
func (s InspectService) ComposeLogs(composeFile string, follow bool, tail string, services []string) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "--follow")
	}
	if tail != "" {
		args = append(args, "--tail", tail)
	}
	args = append(args, services...)
	status, err := docker.DockerCompose(composeFile, args, nil)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("docker compose logs failed with exit code %d", status)
	}
	return nil
}

// ContainerNetworkIPs returns lines of "network ip" for a running container,
// or an error if the container cannot be inspected.
func (s InspectService) ContainerNetworkIPs(name string) ([]string, error) {
	format := `{{range $n, $net := .NetworkSettings.Networks}}{{$n}} {{$net.IPAddress}}{{"\n"}}{{end}}`
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", format, name})
	if err != nil || status != 0 {
		return nil, fmt.Errorf("container '%s' not found", name)
	}
	var result []string
	for _, line := range strings.Split(stdout, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

// ContainerNames returns every container name known to the daemon (running or
// stopped), or nil if docker is unavailable. Used for shell completion.
func (s InspectService) ContainerNames() []string {
	if !docker.IsDockerAvailable() {
		return nil
	}
	status, stdout, _, err := docker.DockerCapture([]string{"ps", "-a", "--format", "{{.Names}}"})
	if err != nil || status != 0 {
		return nil
	}
	var names []string
	for _, line := range strings.Split(stdout, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names
}

// ListUsers returns usernames from /etc/passwd inside the container.
// Used for --user flag completion in shell and exec commands.
func (s InspectService) ListUsers(name string) []string {
	status, stdout, _, err := docker.DockerCapture([]string{"exec", name, "cut", "-d:", "-f1", "/etc/passwd"})
	if err != nil || status != 0 {
		return nil
	}
	var users []string
	for _, line := range strings.Split(stdout, "\n") {
		if u := strings.TrimSpace(line); u != "" {
			users = append(users, u)
		}
	}
	return users
}

// ListDir returns the entries under dir inside the container (directories carry
// a trailing slash), used to drive shell completion.
func (s InspectService) ListDir(name, dir string) ([]string, error) {
	status, stdout, _, err := docker.DockerCapture([]string{"exec", name, "ls", "-1", "-p", dir})
	if err != nil || status != 0 {
		return nil, fmt.Errorf("could not list %q in container %q", dir, name)
	}
	var entries []string
	for _, e := range strings.Split(stdout, "\n") {
		if e = strings.TrimSpace(e); e != "" {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

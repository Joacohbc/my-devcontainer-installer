package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// ContainerState represents the execution state of a container.
type ContainerState string

const (
	StateRunning ContainerState = "running"
	StateExited  ContainerState = "exited"
	StateCreated ContainerState = "created"
	StateDead    ContainerState = "dead"
	StatePaused  ContainerState = "paused"
)

// PortBinding represents a port mapped from the host to the container.
type PortBinding struct {
	HostPort      string
	ContainerPort string
	Protocol      string
}

// VolumeMount represents a directory or volume mounted into the container.
type VolumeMount struct {
	Source      string
	Destination string
	Type        string
}

// ContainerInfo holds the detailed fields shown by the info command.
type ContainerInfo struct {
	Name      string
	Image     string
	Status    ContainerState
	Created   time.Time
	StartedAt time.Time
	IPs       []NetworkIP
	Ports     []PortBinding
	Volumes   []VolumeMount
}

// dockerInspectRaw is the minimal subset of `docker inspect` JSON we consume.
type dockerInspectRaw struct {
	Name  string `json:"Name"`
	State struct {
		Status    string `json:"Status"`
		StartedAt string `json:"StartedAt"`
	} `json:"State"`
	Created string `json:"Created"`
	Mounts  []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
		Networks map[string]struct {
			IPAddress string   `json:"IPAddress"`
			Aliases   []string `json:"Aliases"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
	Config struct {
		Image string `json:"Image"`
	} `json:"Config"`
}

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
func (s InspectService) ContainerState(name string) (ContainerState, error) {
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", "{{.State.Status}}", name})
	if err != nil || status != 0 {
		return "", fmt.Errorf("container '%s' not found", name)
	}
	return ContainerState(strings.TrimSpace(stdout)), nil
}

// ContainerImage returns the container's configured image, or "" if unknown.
func (s InspectService) ContainerImage(name string) string {
	_, stdout, _, _ := docker.DockerCapture([]string{"inspect", "-f", "{{.Config.Image}}", name})
	return strings.TrimSpace(stdout)
}

// ContainerDetails returns full container metadata from a single docker inspect call.
func (s InspectService) ContainerDetails(name string) (*ContainerInfo, error) {
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "--format", "{{json .}}", name})
	if err != nil || status != 0 {
		return nil, fmt.Errorf("container '%s' not found", name)
	}
	var raw dockerInspectRaw
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}
	info := &ContainerInfo{
		Name:   strings.TrimPrefix(raw.Name, "/"),
		Image:  raw.Config.Image,
		Status: ContainerState(raw.State.Status),
	}
	if t, terr := time.Parse(time.RFC3339Nano, raw.Created); terr == nil {
		info.Created = t
	}
	if t, terr := time.Parse(time.RFC3339Nano, raw.State.StartedAt); terr == nil && t.Year() > 1 {
		info.StartedAt = t
	}
	for _, m := range raw.Mounts {
		src := m.Source
		if m.Type == "volume" && m.Name != "" {
			src = m.Name
		}
		info.Volumes = append(info.Volumes, VolumeMount{
			Source:      src,
			Destination: m.Destination,
			Type:        m.Type,
		})
	}
	for portProto, bindings := range raw.NetworkSettings.Ports {
		parts := strings.SplitN(portProto, "/", 2)
		containerPort := parts[0]
		protocol := "tcp"
		if len(parts) > 1 {
			protocol = parts[1]
		}
		for _, b := range bindings {
			if b.HostPort != "" {
				info.Ports = append(info.Ports, PortBinding{
					HostPort:      b.HostPort,
					ContainerPort: containerPort,
					Protocol:      protocol,
				})
			}
		}
	}
	sort.Slice(info.Ports, func(i, j int) bool {
		if info.Ports[i].HostPort == info.Ports[j].HostPort {
			return info.Ports[i].ContainerPort < info.Ports[j].ContainerPort
		}
		return info.Ports[i].HostPort < info.Ports[j].HostPort
	})
	for netName, net := range raw.NetworkSettings.Networks {
		if net.IPAddress != "" {
			info.IPs = append(info.IPs, NetworkIP{
				Network: netName,
				IP:      net.IPAddress,
				Aliases: net.Aliases,
			})
		}
	}
	sort.Slice(info.IPs, func(i, j int) bool {
		if info.IPs[i].Network == info.IPs[j].Network {
			return info.IPs[i].IP < info.IPs[j].IP
		}
		return info.IPs[i].Network < info.IPs[j].Network
	})
	sort.Slice(info.Volumes, func(i, j int) bool {
		return info.Volumes[i].Source < info.Volumes[j].Source
	})
	return info, nil
}

func (s InspectService) ensureRunning(name string) error {
	state, err := s.ContainerState(name)
	if err != nil || state != StateRunning {
		return fmt.Errorf("container '%s' is not running. Run 'devcontainer-cli' or 'devcontainer-cli start' first", name)
	}
	return nil
}

// loginShellLauncher resolves the current user's configured login shell (from
// /etc/passwd, falling back to $SHELL, then bash, then sh) and execs it as a
// login shell (-l) so that /etc/profile and the user's profile/rc files are
// loaded — a fully interactive login session, not a bare shell.
const loginShellLauncher = `SH="$(getent passwd "$(id -u)" 2>/dev/null | cut -d: -f7)"; ` +
	`[ -x "$SH" ] || SH="$SHELL"; ` +
	`[ -x "$SH" ] || SH="$(command -v bash)"; ` +
	`[ -x "$SH" ] || SH="$(command -v sh)"; ` +
	`exec "$SH" -l`

// Shell opens an interactive login shell (or runs command) in the running
// container. With no command it launches the user's configured login shell so
// profiles and rc files are sourced.
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
		execArgs = append(execArgs, "sh", "-c", loginShellLauncher)
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

// Copy copies a local file/dir from the host into the running container.
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

// CopyFromContainer copies a file/dir from the running container out to the host.
func (s InspectService) CopyFromContainer(name, containerPath, localPath string) error {
	if err := s.ensureRunning(name); err != nil {
		return err
	}
	s.Report.Warn("\nCopying '%s' from container '%s' to '%s'...", containerPath, name, localPath)
	exitCode, err := docker.DockerInherit([]string{"cp", name + ":" + containerPath, localPath})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("copy failed with exit code %d", exitCode)
	}
	s.Report.Success("\nSuccessfully copied.\n")
	return nil
}

// CopyAsset materializes a copyable embedded script and copies it into the
// devuser home of the running container, leaving it owned by devuser and
// executable. When dest is empty it defaults to DevUserHome/<filename>.
func (s InspectService) CopyAsset(name, assetName, dest string) error {
	asset, ok := assets.LookupCopyable(assetName)
	if !ok {
		return fmt.Errorf("unknown copyable asset '%s' (run with shell completion to list options)", assetName)
	}
	if err := s.ensureRunning(name); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "dc-asset-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	res := assets.Preflight([]string{asset.File}, tmpDir)
	if len(res.Missing) > 0 {
		return fmt.Errorf("asset '%s' could not be materialized", asset.File)
	}

	if dest == "" {
		dest = types.DevUserHome + "/" + asset.File
	}

	if err := s.Copy(name, filepath.Join(tmpDir, asset.File), dest); err != nil {
		return err
	}

	// docker cp lands the file as root; make it a devuser-owned executable script.
	fixArgs := []string{"exec", name, "sh", "-c",
		fmt.Sprintf("chown devuser:devuser %q && chmod +x %q", dest, dest)}
	if exitCode, err := docker.DockerInherit(fixArgs); err != nil {
		return err
	} else if exitCode != 0 {
		s.Report.Warn("could not set ownership/permissions on '%s' (exit %d)", dest, exitCode)
	}
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

// NetworkIP pairs a docker network name with the container's IP on it and the
// network-scoped DNS aliases (extra names) it answers to on that network.
type NetworkIP struct {
	Network string
	IP      string
	Aliases []string
}

// ContainerNetworkIPs returns structured NetworkIP pairs for a running container,
// or an error if the container cannot be inspected.
func (s InspectService) ContainerNetworkIPs(name string) ([]NetworkIP, error) {
	format := `{{range $n, $net := .NetworkSettings.Networks}}{{$n}} {{$net.IPAddress}}{{"\n"}}{{end}}`
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", format, name})
	if err != nil || status != 0 {
		return nil, fmt.Errorf("container '%s' not found", name)
	}
	var result []NetworkIP
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sep := strings.Index(line, " ")
		if sep < 0 {
			continue
		}
		ip := strings.TrimSpace(line[sep+1:])
		if ip != "" {
			result = append(result, NetworkIP{Network: line[:sep], IP: ip})
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
	containers, err := s.ListContainers(false)
	if err != nil {
		return nil
	}
	var names []string
	for _, c := range containers {
		names = append(names, c.Name)
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

// Container represents a high-level Docker container listed by InspectService.
type Container struct {
	Name      string
	Image     string
	Status    string
	State     string
	Managed   bool
	Ports     string
	Workspace string // workspace name, "standalone" for quick-run, "" if unmanaged
}

type dockerPSLine struct {
	Names  string `json:"Names"`
	Image  string `json:"Image"`
	Status string `json:"Status"`
	State  string `json:"State"`
	Labels string `json:"Labels"`
	Ports  string `json:"Ports"`
}

func parsePSLines(stdout string) []dockerPSLine {
	var out []dockerPSLine
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var obj dockerPSLine
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if obj.Names == "" {
			continue
		}
		out = append(out, obj)
	}
	return out
}

// ListManaged returns every container carrying the CLI managed label (running
// and stopped). Docker errors yield an empty list.
func (s InspectService) ListManaged() []Container {
	containers, _ := s.ListContainers(true)
	return containers
}

// ListAll returns every container regardless of label (running and stopped),
// flagging which are CLI-managed devcontainers.
func (s InspectService) ListAll() []Container {
	containers, _ := s.ListContainers(false)
	return containers
}

// composeProjectLabel is the label docker compose stamps on every container with
// the project name. The generator sets COMPOSE_PROJECT_NAME to the workspace, so
// for managed devcontainers this label carries the workspace name.
const composeProjectLabel = "com.docker.compose.project"

// parseLabels turns docker's comma-separated "k=v,k=v" label string into a map.
func parseLabels(s string) map[string]string {
	out := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

// deriveWorkspace returns the workspace name for a managed devcontainer (from the
// compose project label), "standalone" for a quick-run container, or "" when the
// container is not CLI-managed.
func deriveWorkspace(labels map[string]string) string {
	if proj := labels[composeProjectLabel]; proj != "" {
		return proj
	}
	if _, ok := labels[types.LabelQuickRun]; ok {
		return "standalone"
	}
	return ""
}

// ListContainers queries the Docker daemon for every container, running and
// stopped (always passes -a). When managedOnly is true only containers carrying
// the CLI managed label are returned. The result is uniform across callers;
// each command filters by State for its own needs (e.g. start wants stopped
// containers, stop wants running ones).
func (s InspectService) ListContainers(managedOnly bool) ([]Container, error) {
	args := []string{"ps", "-a", "--format", "{{json .}}"}
	if managedOnly {
		args = append(args, "--filter", "label="+types.LabelManaged+"=true")
	}

	status, stdout, _, err := docker.DockerCapture(args)
	if err != nil {
		return nil, err
	}
	if status != 0 {
		return nil, fmt.Errorf("docker ps exited with status %d", status)
	}
	if strings.TrimSpace(stdout) == "" {
		return nil, nil
	}

	var out []Container
	for _, l := range parsePSLines(stdout) {
		labels := parseLabels(l.Labels)
		out = append(out, Container{
			Name:      l.Names,
			Image:     l.Image,
			Status:    l.Status,
			State:     l.State,
			Managed:   managedOnly || labels[types.LabelManaged] == "true",
			Ports:     l.Ports,
			Workspace: deriveWorkspace(labels),
		})
	}
	return out, nil
}

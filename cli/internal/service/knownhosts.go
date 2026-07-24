package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// containerHostKeyCat reads every sshd host public key out of a container. The
// keys are the ones `sshd` presents on the wire, so pinning them is what makes
// `ssh` skip both the "authenticity of host … can't be established" prompt and
// the "REMOTE HOST IDENTIFICATION HAS CHANGED" failure.
const containerHostKeyCat = "cat /etc/ssh/ssh_host_*_key.pub 2>/dev/null"

// hostKeyTypePrefixes are the leading tokens of a known_hosts/`*.pub` key line.
// Anything else on stdout (a shell warning, an empty glob) is not a key.
var hostKeyTypePrefixes = []string{"ssh-", "ecdsa-", "sk-ssh-", "sk-ecdsa-"}

// ContainerHostKeys returns the container's sshd host public keys as
// `<type> <base64>` pairs, read through the docker daemon rather than over the
// network. That out-of-band channel is what makes re-pinning safe: the key is
// learned from the daemon that owns the container, not from whoever answers on
// the IP.
func (s SshService) ContainerHostKeys(container string) ([]string, error) {
	status, stdout, _, err := docker.DockerCapture([]string{"exec", container, "sh", "-c", containerHostKeyCat})
	if err != nil || status != 0 {
		return nil, fmt.Errorf("could not read ssh host keys from container '%s'", container)
	}
	keys := parseHostKeys(stdout)
	if len(keys) == 0 {
		return nil, fmt.Errorf("container '%s' has no ssh host keys", container)
	}
	return keys, nil
}

// parseHostKeys keeps the key lines of a concatenated `*.pub` dump, normalized to
// `<type> <base64>` (the trailing comment is dropped: it is not part of the
// identity and differs between the container's copy and a known_hosts entry).
func parseHostKeys(stdout string) []string {
	var keys []string
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !hasHostKeyType(fields[0]) {
			continue
		}
		keys = append(keys, fields[0]+" "+fields[1])
	}
	return keys
}

func hasHostKeyType(field string) bool {
	for _, prefix := range hostKeyTypePrefixes {
		if strings.HasPrefix(field, prefix) {
			return true
		}
	}
	return false
}

// PinContainerHostKeys records container's current host keys for host (the
// address the generated Host block dials) in the CLI-managed known_hosts,
// replacing whatever was pinned for that address before.
//
// This is the fix for the recurring "REMOTE HOST IDENTIFICATION HAS CHANGED"
// failure: a devcontainer regenerates its host keys whenever the image is
// rebuilt, but keeps the same static IP, so the previously recorded key no
// longer matches. Because the CLI can ask docker which keys the container
// actually has, it re-pins them instead of leaving the user to delete the
// offending line by hand.
func (s SshService) PinContainerHostKeys(container, host string) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("no host to pin keys for")
	}
	keys, err := s.ContainerHostKeys(container)
	if err != nil {
		return err
	}
	return writeKnownHosts(domain.ManagedKnownHostsPath(), host, keys)
}

// writeKnownHosts rewrites path with host's entries replaced by keys, creating
// the file (0600) and its directory (0700) when missing.
func writeKnownHosts(path, host string, keys []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(ReplaceKnownHostsEntries(string(data), host, keys)), 0o600)
}

// ReplaceKnownHostsEntries returns content with every entry for host removed and
// the given keys appended as `<host> <type> <base64>` lines. Entries for other
// hosts, comments and blank lines are preserved.
func ReplaceKnownHostsEntries(content, host string, keys []string) string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		if knownHostsLineMatches(line, host) {
			continue
		}
		out = append(out, line)
	}
	body := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if body != "" {
		body += "\n"
	}
	for _, key := range keys {
		body += host + " " + key + "\n"
	}
	return body
}

// knownHostsLineMatches reports whether a known_hosts line records host. It
// understands the `@marker` prefix and the comma-separated pattern list, and
// matches the bare `host` and bracketed `[host]:port` spellings. Hashed entries
// (|1|…) never match — the CLI writes its file with HashKnownHosts no precisely
// so its own entries stay findable.
func knownHostsLineMatches(line, host string) bool {
	fields := strings.Fields(line)
	if len(fields) > 0 && strings.HasPrefix(fields[0], "@") {
		fields = fields[1:]
	}
	if len(fields) < 2 {
		return false
	}
	for _, pattern := range strings.Split(fields[0], ",") {
		if pattern == host || strings.HasPrefix(pattern, "["+host+"]:") {
			return true
		}
	}
	return false
}

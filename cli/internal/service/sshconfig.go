package service

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var hostAliasRe = regexp.MustCompile(`^\s*Host\s+(.+)$`)

// managedMarkerRe matches the comment that sshdefaults.ManagedComment emits above
// a CLI-generated Host block, capturing the owning workspace.
var managedMarkerRe = regexp.MustCompile(`^\s*#\s*devcontainer-cli:managed\s+workspace=(\S+)\s*$`)

// managedMarkerWorkspace returns the workspace tagged on a managed marker line, or
// ("", false) when line is not such a marker.
func managedMarkerWorkspace(line string) (string, bool) {
	m := managedMarkerRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// isManagedMarkerLine reports whether line is any CLI managed-block marker.
func isManagedMarkerLine(line string) bool {
	_, ok := managedMarkerWorkspace(line)
	return ok
}

// HostAliasesInLine returns the aliases declared on a single `Host a b c` line,
// or nil when line is not a Host declaration.
func HostAliasesInLine(line string) []string {
	m := hostAliasRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	return strings.Fields(m[1])
}

// HasHostAlias reports whether content declares a Host block for alias.
func HasHostAlias(content, alias string) bool {
	for _, line := range strings.Split(content, "\n") {
		if slices.Contains(HostAliasesInLine(line), alias) {
			return true
		}
	}
	return false
}

// StripHostBlock returns content with the Host block for alias removed.
func StripHostBlock(content, alias string) string {
	var out []string
	skip := false
	for _, line := range strings.Split(content, "\n") {
		hosts := HostAliasesInLine(line)
		isHostLine := len(hosts) > 0
		if skip {
			if isHostLine {
				if slices.Contains(hosts, alias) {
					continue
				}
				skip = false
				out = append(out, line)
			}
			continue
		}
		if isHostLine && slices.Contains(hosts, alias) {
			// Drop a managed marker comment sitting directly above this Host line so
			// removing the block does not leave an orphaned marker behind.
			if n := len(out); n > 0 && isManagedMarkerLine(out[n-1]) {
				out = out[:n-1]
			}
			skip = true
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// HasManagedBlock reports whether content contains a CLI managed Host block tagged
// with workspace.
func HasManagedBlock(content, workspace string) bool {
	for _, line := range strings.Split(content, "\n") {
		if ws, ok := managedMarkerWorkspace(line); ok && ws == workspace {
			return true
		}
	}
	return false
}

// StripManagedBlock returns content with the CLI managed block tagged workspace
// removed: the marker comment line plus the Host stanza beneath it, up to the next
// Host line, the next marker, or EOF.
func StripManagedBlock(content, workspace string) string {
	var out []string
	skip := false
	sawHost := false
	for _, line := range strings.Split(content, "\n") {
		if ws, ok := managedMarkerWorkspace(line); ok && ws == workspace {
			// Start dropping: the marker line, then the Host stanza that follows it.
			skip = true
			sawHost = false
			continue
		}
		if skip {
			isHostLine := len(HostAliasesInLine(line)) > 0
			// The block's own Host line is the first Host line after the marker; only
			// a later Host/Match line or another marker ends the stanza.
			if (isHostLine && sawHost) || isManagedMarkerLine(line) {
				skip = false
				out = append(out, line)
				continue
			}
			if isHostLine {
				sawHost = true
			}
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// ExtractHostBlock returns just the Host block for alias from content, or "" if
// absent.
func ExtractHostBlock(content, alias string) string {
	var out []string
	printing := false
	for _, line := range strings.Split(content, "\n") {
		hosts := HostAliasesInLine(line)
		isHostLine := len(hosts) > 0
		if printing {
			if isHostLine {
				if !slices.Contains(hosts, alias) {
					break
				}
				out = append(out, line)
				continue
			}
			out = append(out, line)
			continue
		}
		if isHostLine && slices.Contains(hosts, alias) {
			printing = true
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// ConfigHostAliases returns every non-wildcard Host alias declared in content,
// de-duplicated and in first-seen order.
func ConfigHostAliases(content string) []string {
	var hosts []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(content, "\n") {
		for _, h := range HostAliasesInLine(line) {
			h = strings.TrimSpace(h)
			if h != "" && !strings.Contains(h, "*") && !strings.Contains(h, "?") && !seen[h] {
				seen[h] = true
				hosts = append(hosts, h)
			}
		}
	}
	return hosts
}

// sshConfigPath returns the path to ~/.ssh/config.
func sshConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

// ReadSSHConfig resolves ~/.ssh/config, creating the .ssh dir (0700) and an
// empty config (0600) if missing, and returns the path and current contents.
func (s SshService) ReadSSHConfig() (path string, content string, err error) {
	path, err = sshConfigPath()
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", "", err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
			return "", "", err
		}
	}
	_ = os.Chmod(path, 0o600)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	return path, string(data), nil
}

// ReplaceHostBlock backs up the config (to path+".bak"), removes the existing
// Host block for alias, appends newBlock, and writes the result. It returns the
// backup path.
func (s SshService) ReplaceHostBlock(path, content, alias, newBlock string) (string, error) {
	backupPath := path + ".bak"
	if err := os.WriteFile(backupPath, []byte(content), 0o600); err != nil {
		return "", err
	}
	stripped := strings.TrimRight(StripHostBlock(content, alias), "\n")
	if stripped != "" {
		stripped += "\n\n"
	}
	if err := os.WriteFile(path, []byte(stripped+newBlock+"\n"), 0o600); err != nil {
		return "", err
	}
	return backupPath, nil
}

// AppendHostBlock appends newBlock to the config, separated from existing
// content by a blank line.
func (s SshService) AppendHostBlock(path, content, newBlock string) error {
	body := strings.TrimRight(content, "\n")
	if body != "" {
		body += "\n\n"
	}
	return os.WriteFile(path, []byte(body+newBlock+"\n"), 0o600)
}

// RemoveManagedBlock removes the CLI managed Host block tagged with workspace from
// ~/.ssh/config. Unlike ReadSSHConfig it does not create the file: a missing config
// (or no matching block) is a no-op returning removed=false. When a block is
// removed it backs the original up to path+".bak" first and returns that path.
func (s SshService) RemoveManagedBlock(workspace string) (removed bool, backupPath string, err error) {
	path, err := sshConfigPath()
	if err != nil {
		return false, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", err
	}
	content := string(data)
	if !HasManagedBlock(content, workspace) {
		return false, "", nil
	}
	backupPath = path + ".bak"
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return false, "", err
	}
	stripped := strings.TrimRight(StripManagedBlock(content, workspace), "\n")
	if stripped != "" {
		stripped += "\n"
	}
	if err := os.WriteFile(path, []byte(stripped), 0o600); err != nil {
		return false, "", err
	}
	return true, backupPath, nil
}

// ConfigHostAliasesFromDisk reads ~/.ssh/config and returns its non-wildcard
// Host aliases, or nil if the file cannot be read.
func (s SshService) ConfigHostAliasesFromDisk() []string {
	path, err := sshConfigPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return ConfigHostAliases(string(data))
}

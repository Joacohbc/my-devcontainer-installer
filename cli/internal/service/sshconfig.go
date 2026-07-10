package service

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

var hostAliasRe = regexp.MustCompile(`^\s*Host\s+(.+)$`)

// ManagedMarker is the parsed form of the comment sshdefaults.ManagedComment emits
// above a CLI-generated Host block: what the block targets (Kind), its stable
// identity (Ref), and the Host alias it configures.
type ManagedMarker struct {
	Kind  string // "workspace" or "container"
	Ref   string // unique workspace name, or container name
	Alias string // the Host alias configured by the block (may be empty for legacy markers)
}

// parseManagedMarker parses a managed-block marker comment. It reads the modern
// "v=1 kind=... ref=... alias=..." key/value form and also maps the legacy
// "workspace=<ws>" form to {Kind: workspace, Ref: ws}. It returns ok=false when
// line is not a marker or lacks a resolvable kind+ref.
func parseManagedMarker(line string) (ManagedMarker, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return ManagedMarker{}, false
	}
	body := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
	if !strings.HasPrefix(body, sshdefaults.ManagedMarker) {
		return ManagedMarker{}, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(body, sshdefaults.ManagedMarker))

	var m ManagedMarker
	for _, field := range strings.Fields(rest) {
		k, v, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch k {
		case "kind":
			m.Kind = v
		case "ref":
			m.Ref = v
		case "alias":
			m.Alias = v
		case "workspace": // legacy form implies a workspace-kind block keyed by <ws>
			if m.Kind == "" {
				m.Kind = string(sshdefaults.KindWorkspace)
			}
			if m.Ref == "" {
				m.Ref = v
			}
		}
	}
	if m.Kind == "" || m.Ref == "" {
		return ManagedMarker{}, false
	}
	return m, true
}

// isManagedMarkerLine reports whether line is any CLI managed-block marker.
func isManagedMarkerLine(line string) bool {
	_, ok := parseManagedMarker(line)
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

// HasManagedBlockByRef reports whether content contains a CLI managed Host block
// whose marker targets the given kind + ref.
func HasManagedBlockByRef(content, kind, ref string) bool {
	for _, line := range strings.Split(content, "\n") {
		if m, ok := parseManagedMarker(line); ok && m.Kind == kind && m.Ref == ref {
			return true
		}
	}
	return false
}

// HasManagedBlock reports whether content contains a CLI managed workspace block
// tagged with workspace.
func HasManagedBlock(content, workspace string) bool {
	return HasManagedBlockByRef(content, string(sshdefaults.KindWorkspace), workspace)
}

// StripManagedBlockByRef returns content with the CLI managed block matching kind +
// ref removed: the marker comment line plus the Host stanza beneath it, up to the
// next Host line, the next marker, or EOF.
func StripManagedBlockByRef(content, kind, ref string) string {
	return stripManagedBlockWhere(content, func(m ManagedMarker) bool {
		return m.Kind == kind && m.Ref == ref
	})
}

// StripManagedBlock returns content with the CLI managed workspace block tagged
// workspace removed.
func StripManagedBlock(content, workspace string) string {
	return StripManagedBlockByRef(content, string(sshdefaults.KindWorkspace), workspace)
}

// stripManagedBlockWhere drops every managed block whose parsed marker satisfies
// match: the marker line plus the Host stanza beneath it, up to the next Host line,
// the next marker, or EOF.
func stripManagedBlockWhere(content string, match func(ManagedMarker) bool) string {
	var out []string
	skip := false
	sawHost := false
	for _, line := range strings.Split(content, "\n") {
		if m, ok := parseManagedMarker(line); ok && match(m) {
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

// ListManagedBlocks enumerates every CLI managed block in content. When a marker
// omits its alias (legacy form) the alias is recovered from the Host line beneath
// it so callers can report the block precisely.
func ListManagedBlocks(content string) []ManagedMarker {
	lines := strings.Split(content, "\n")
	var out []ManagedMarker
	for i, line := range lines {
		m, ok := parseManagedMarker(line)
		if !ok {
			continue
		}
		if m.Alias == "" {
			for j := i + 1; j < len(lines); j++ {
				if aliases := HostAliasesInLine(lines[j]); len(aliases) > 0 {
					m.Alias = aliases[0]
					break
				}
				if isManagedMarkerLine(lines[j]) {
					break
				}
			}
		}
		out = append(out, m)
	}
	return out
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

// targetAlive reports whether a managed block's target still exists, using the
// caller-supplied existence predicates. An unrecognized kind is treated as alive so
// clean-ssh never removes a block it does not understand.
func targetAlive(b ManagedMarker, existsWorkspace, existsContainer func(string) bool) bool {
	switch b.Kind {
	case string(sshdefaults.KindContainer):
		return existsContainer(b.Ref)
	case string(sshdefaults.KindWorkspace):
		return existsWorkspace(b.Ref)
	default:
		return true
	}
}

// PruneManagedBlocks removes CLI managed Host blocks from ~/.ssh/config whose target
// no longer exists, as judged by the two existence predicates (keyed by workspace
// name and container name respectively). It returns the blocks it removed (or, in
// dryRun mode, would remove). A missing config, or nothing stale, is a no-op
// returning an empty slice. When it rewrites the file it first backs the original up
// to path+".bak" and returns that path. The predicates are injected so this stays
// free of infra/registry dependencies and is table-testable.
func (s SshService) PruneManagedBlocks(existsWorkspace, existsContainer func(string) bool, dryRun bool) (removed []ManagedMarker, backupPath string, err error) {
	path, err := sshConfigPath()
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", err
	}
	content := string(data)

	var stale []ManagedMarker
	for _, b := range ListManagedBlocks(content) {
		if !targetAlive(b, existsWorkspace, existsContainer) {
			stale = append(stale, b)
		}
	}
	if len(stale) == 0 {
		return nil, "", nil
	}
	if dryRun {
		return stale, "", nil
	}

	backupPath = path + ".bak"
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return nil, "", err
	}
	stripped := content
	for _, b := range stale {
		stripped = StripManagedBlockByRef(stripped, b.Kind, b.Ref)
	}
	stripped = strings.TrimRight(stripped, "\n")
	if stripped != "" {
		stripped += "\n"
	}
	if err := os.WriteFile(path, []byte(stripped), 0o600); err != nil {
		return nil, "", err
	}
	return stale, backupPath, nil
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

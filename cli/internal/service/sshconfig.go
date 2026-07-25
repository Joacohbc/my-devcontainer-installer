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

// AliasForManagedRef returns the Host alias of the CLI managed block in content
// matching kind + ref (e.g. kind="workspace", ref=<workspace name>), if any.
func AliasForManagedRef(content, kind, ref string) (alias string, ok bool) {
	for _, m := range ListManagedBlocks(content) {
		if m.Kind == kind && m.Ref == ref && m.Alias != "" {
			return m.Alias, true
		}
	}
	return "", false
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

// optionKeyword returns the ssh config keyword a stanza line sets, or "" when the
// line declares nothing (blank, comment, Host line). Both the `Keyword value` and
// the `Keyword=value` spellings are recognized.
func optionKeyword(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}
	keyword := trimmed
	if i := strings.IndexAny(trimmed, " \t="); i >= 0 {
		keyword = trimmed[:i]
	}
	if strings.EqualFold(keyword, "Host") || strings.EqualFold(keyword, "Match") {
		return ""
	}
	return keyword
}

// HostNameForAlias returns the HostName the Host block for alias dials, or "" when
// there is no such block or it sets no HostName (a ProxyCommand block, where ssh
// falls back to the alias itself).
func HostNameForAlias(content, alias string) string {
	for _, line := range strings.Split(ExtractHostBlock(content, alias), "\n") {
		if !strings.EqualFold(optionKeyword(line), "HostName") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			return fields[1]
		}
	}
	return ""
}

// ConfigHostTargets returns every address content can dial, de-duplicated and in
// first-seen order: each block's Host aliases plus its HostName. Wildcard
// patterns are skipped — they name no single host. It is what clean ssh checks
// pinned host keys against, so an entry is only ever considered orphaned when
// no block (managed or hand-written) still reaches it.
func ConfigHostTargets(content string) []string {
	var targets []string
	seen := make(map[string]bool)
	add := func(value string) {
		if value == "" || strings.ContainsAny(value, "*?") || seen[value] {
			return
		}
		seen[value] = true
		targets = append(targets, value)
	}
	for _, line := range strings.Split(content, "\n") {
		for _, alias := range HostAliasesInLine(line) {
			add(alias)
		}
		if strings.EqualFold(optionKeyword(line), "HostName") {
			if fields := strings.Fields(line); len(fields) >= 2 {
				add(fields[1])
			}
		}
	}
	return targets
}

// EnsureHostKeyOptions returns content with the Host block for alias carrying the
// host-key options for knownHostsFile — replacing stale values and appending the
// missing ones in place, so a block written by an older CLI version is upgraded
// without being moved or rewritten wholesale. It reports whether anything changed.
func EnsureHostKeyOptions(content, alias, knownHostsFile string) (string, bool) {
	lines := strings.Split(content, "\n")
	start, end := hostBlockRange(lines, alias)
	if start < 0 {
		return content, false
	}
	rewrittenBlock := append([]string{lines[start]}, rewriteHostKeyOptions(lines[start+1:end], knownHostsFile)...)
	updated := strings.Join(slices.Concat(lines[:start], rewrittenBlock, lines[end:]), "\n")
	return updated, updated != content
}

// hostBlockRange locates the Host block declaring alias: the index of its Host
// line and of the first line past the block (the next Host declaration, or the
// end of the file). start is -1 when no block declares alias.
func hostBlockRange(lines []string, alias string) (start, end int) {
	start = -1
	for index, line := range lines {
		aliases := HostAliasesInLine(line)
		if len(aliases) == 0 {
			continue
		}
		if start >= 0 {
			return start, index
		}
		if slices.Contains(aliases, alias) {
			start = index
		}
	}
	if start < 0 {
		return -1, 0
	}
	return start, len(lines)
}

// rewriteHostKeyOptions returns a block body carrying exactly the host-key
// options for knownHostsFile: any previous spelling of them is dropped and the
// current ones are appended below the block's other settings.
func rewriteHostKeyOptions(body []string, knownHostsFile string) []string {
	options := sshdefaults.HostKeyOptions(knownHostsFile)
	settings, trailingBlanks := splitTrailingBlanks(dropOptions(body, options))
	rewritten := slices.Clone(settings)
	for _, option := range options {
		rewritten = append(rewritten, option.Line())
	}
	return append(rewritten, trailingBlanks...)
}

// dropOptions returns body without the lines setting any of options' keywords.
func dropOptions(body []string, options []sshdefaults.ConfigOption) []string {
	var kept []string
	for _, line := range body {
		if setsAnyOption(line, options) {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

// setsAnyOption reports whether line sets one of options' keywords, compared
// case-insensitively as ssh reads them.
func setsAnyOption(line string, options []sshdefaults.ConfigOption) bool {
	keyword := optionKeyword(line)
	if keyword == "" {
		return false
	}
	for _, option := range options {
		if strings.EqualFold(option.Keyword, keyword) {
			return true
		}
	}
	return false
}

// splitTrailingBlanks separates the blank lines closing a block from the settings
// above them, so appended options land inside the block instead of after the gap
// that follows it.
func splitTrailingBlanks(lines []string) (settings, blanks []string) {
	lastSetting := len(lines)
	for lastSetting > 0 && strings.TrimSpace(lines[lastSetting-1]) == "" {
		lastSetting--
	}
	return lines[:lastSetting], lines[lastSetting:]
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

// ManagedAlias resolves the Host alias already configured for a CLI managed
// block matching kind + ref (e.g. a workspace's own devcontainer), if
// ~/.ssh/config has one. Unlike ReadSSHConfig it does not create the file: a
// missing config is treated as "not found" (ok=false), not an error. Used by
// 'ssh' to skip the setup-ssh bootstrap when access is already configured.
func (s SshService) ManagedAlias(kind sshdefaults.Kind, ref string) (alias string, ok bool, err error) {
	path, err := sshConfigPath()
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	alias, ok = AliasForManagedRef(string(data), string(kind), ref)
	return alias, ok, nil
}

// AliasHostName returns the HostName the Host block for alias dials in
// ~/.ssh/config, or "" when the config, the block, or its HostName is absent.
// It is what the CLI pins host keys against: the exact address ssh will use.
func (s SshService) AliasHostName(alias string) (string, error) {
	path, err := sshConfigPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return HostNameForAlias(string(data), alias), nil
}

// ManagedHostName returns the address the managed block for kind + ref dials:
// its alias' HostName. It reads the config once, and yields "" when the config,
// the block, or its HostName is absent (a ProxyCommand block sets none). Callers
// use it to learn what a block had pinned *before* removing it.
func (s SshService) ManagedHostName(kind sshdefaults.Kind, ref string) (string, error) {
	path, err := sshConfigPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	content := string(data)
	alias, ok := AliasForManagedRef(content, string(kind), ref)
	if !ok {
		return "", nil
	}
	return HostNameForAlias(content, alias), nil
}

// HostIsReferenced reports whether any Host block in ~/.ssh/config still dials
// host. It guards the removal of a pinned key: two aliases may point at the same
// container, so a key is only dropped once nothing reaches it any more.
func (s SshService) HostIsReferenced(host string) (bool, error) {
	if host == "" {
		return false, nil
	}
	path, err := sshConfigPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return slices.Contains(ConfigHostTargets(string(data)), host), nil
}

// EnsureHostKeyPinning upgrades an already written Host block in place so it
// verifies host keys against the CLI-managed known_hosts. Blocks generated
// before host-key pinning existed record container keys in the user's global
// ~/.ssh/known_hosts, where a rebuilt container's new key reads as a changed
// host key; this repairs them without the user re-running setup-ssh. It backs
// the config up to path+".bak" before rewriting and reports whether the block
// needed changing.
func (s SshService) EnsureHostKeyPinning(alias, knownHostsFile string) (updated bool, err error) {
	path, err := sshConfigPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	content, changed := EnsureHostKeyOptions(string(data), alias, knownHostsFile)
	if !changed {
		return false, nil
	}
	if err := os.WriteFile(path+".bak", data, 0o600); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveManagedBlockByRef removes the CLI managed Host block matching kind + ref
// (a workspace name, or a specific container name in loose --container mode) from
// ~/.ssh/config. Unlike ReadSSHConfig it does not create the file: a missing
// config (or no matching block) is a no-op returning removed=false. When a block
// is removed it backs the original up to path+".bak" first and returns that path.
func (s SshService) RemoveManagedBlockByRef(kind sshdefaults.Kind, ref string) (removed bool, backupPath string, err error) {
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
	if !HasManagedBlockByRef(content, string(kind), ref) {
		return false, "", nil
	}
	backupPath = path + ".bak"
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return false, "", err
	}
	stripped := strings.TrimRight(StripManagedBlockByRef(content, string(kind), ref), "\n")
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

// RemoveManagedBlocks removes exactly the given managed blocks from
// ~/.ssh/config, without re-checking whether their target is alive — the
// caller (clean-ssh, after the user picks a subset of the stale blocks it
// found) has already decided which ones to drop. A missing config, or an
// empty blocks slice, is a no-op returning an empty result. When it rewrites
// the file it first backs the original up to path+".bak" and returns that
// path.
func (s SshService) RemoveManagedBlocks(blocks []ManagedMarker) (removed []ManagedMarker, backupPath string, err error) {
	if len(blocks) == 0 {
		return nil, "", nil
	}
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

	backupPath = path + ".bak"
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return nil, "", err
	}
	stripped := content
	for _, b := range blocks {
		stripped = StripManagedBlockByRef(stripped, b.Kind, b.Ref)
	}
	stripped = strings.TrimRight(stripped, "\n")
	if stripped != "" {
		stripped += "\n"
	}
	if err := os.WriteFile(path, []byte(stripped), 0o600); err != nil {
		return nil, "", err
	}
	return blocks, backupPath, nil
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

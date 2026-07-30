package service

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
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
	Host  string // --via jump target the block's docker calls must route through (empty for ordinary local/legacy blocks)
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
		case "host":
			m.Host = v
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

// The CLI owns its own SSH config file and never writes Host blocks into the
// user's ~/.ssh/config — that file only ever gains a single Include directive
// pointing at the managed one. Two path helpers keep the split explicit:
//
//   - managedConfigPath: the CLI's file. Every managed Host block is read from
//     and written to it. Overridable with `config ssh-config-file`.
//   - domain.UserSSHConfigPath: the user's file. Read-only except for the
//     Include line written by EnsureInclude.
//
// Read/write of managed blocks goes through the managed file alone. Queries
// that must not miss hand-written config the user maintains themselves — alias
// collision checks, "is this host still referenced", completion — read BOTH
// (see managedAndUserConfigs).

// managedConfigPath returns the path of the CLI-owned SSH config file.
func managedConfigPath() (string, error) {
	path := domain.ResolveSSHConfigPath("")
	if path == "" {
		return "", fmt.Errorf("could not resolve the managed SSH config path")
	}
	return path, nil
}

// readFileIfExists reads a file, treating "does not exist" as empty content rather
// than an error. Every managed-block query is a no-op on a missing config.
func readFileIfExists(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// managedAndUserConfigs returns the managed config and the user's ~/.ssh/config
// concatenated. Use it for read-only queries that must see the user's own
// hand-written blocks too: an alias collision check that only looked at the
// managed file would happily shadow a Host the user already defined, and a
// host-reference check that missed the user's file would unpin a key their own
// block still dials.
func (s SshService) managedAndUserConfigs() (string, error) {
	managedPath, err := managedConfigPath()
	if err != nil {
		return "", err
	}
	managed, err := readFileIfExists(managedPath)
	if err != nil {
		return "", err
	}
	user, err := readFileIfExists(domain.UserSSHConfigPath())
	if err != nil {
		return "", err
	}
	return managed + "\n" + user, nil
}

// ReadManagedConfig resolves the CLI-owned SSH config file, creating its parent
// dir (0700) and an empty file (0600) if missing, and returns the path and
// current contents.
func (s SshService) ReadManagedConfig() (path string, content string, err error) {
	path, err = managedConfigPath()
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

// isStanzaStart reports whether line opens a Host or Match stanza. Note this is
// exactly what optionKeyword deliberately does NOT report (it answers "is this a
// setting inside a stanza"), so the two are complements, not duplicates.
func isStanzaStart(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}
	keyword := trimmed
	if i := strings.IndexAny(trimmed, " \t="); i >= 0 {
		keyword = trimmed[:i]
	}
	return strings.EqualFold(keyword, "Host") || strings.EqualFold(keyword, "Match")
}

// includeMarker tags the Include line the CLI writes into the user's config so
// it is recognizable (and removable) as CLI-authored.
const includeMarker = "# " + sshdefaults.ManagedMarker + " v=" + sshdefaults.MarkerVersion + " kind=include"

// includeDirective renders the `Include` line for the managed config. A managed
// file that sits inside ~/.ssh is referenced by its bare filename, which is how
// OpenSSH resolves relative includes; anything else gets an absolute path.
func includeDirective(managedPath string) string {
	sshDir := filepath.Dir(domain.UserSSHConfigPath())
	if filepath.Dir(managedPath) == sshDir {
		return "Include " + filepath.Base(managedPath)
	}
	return "Include " + managedPath
}

// hasInclude reports whether content already pulls in managedPath, accepting
// either the relative or the absolute spelling regardless of which one the CLI
// would write today (the user may have moved the file, or written it by hand).
func hasInclude(content, managedPath string) bool {
	rel := filepath.Base(managedPath)
	for _, line := range strings.Split(content, "\n") {
		if !strings.EqualFold(optionKeyword(line), "Include") {
			continue
		}
		for _, field := range strings.Fields(strings.TrimSpace(line))[1:] {
			if field == managedPath || field == rel || field == "~/.ssh/"+rel {
				return true
			}
		}
	}
	return false
}

// EnsureInclude makes sure the user's ~/.ssh/config pulls in the CLI-managed
// config file, and reports whether it had to add the directive.
//
// The Include is inserted at the TOP, before any Host/Match block: OpenSSH keeps
// the FIRST value obtained for each keyword, so an Include appended at the
// bottom would be shadowed by anything above it — and an Include sitting inside
// a Host stanza would silently become part of that stanza. This is the only
// write the CLI ever performs on the user's own config; the original is backed
// up to path+".bak" first.
func (s SshService) EnsureInclude() (added bool, err error) {
	managedPath, err := managedConfigPath()
	if err != nil {
		return false, err
	}
	userPath := domain.UserSSHConfigPath()
	if err := os.MkdirAll(filepath.Dir(userPath), 0o700); err != nil {
		return false, err
	}
	content, err := readFileIfExists(userPath)
	if err != nil {
		return false, err
	}
	if hasInclude(content, managedPath) {
		return false, nil
	}

	header := includeMarker + "\n" + includeDirective(managedPath) + "\n"
	if strings.TrimSpace(content) == "" {
		return true, os.WriteFile(userPath, []byte(header), 0o600)
	}
	if err := os.WriteFile(userPath+".bak", []byte(content), 0o600); err != nil {
		return false, err
	}
	updated := insertBeforeFirstStanza(content, header)
	if err := os.WriteFile(userPath, []byte(updated), 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// insertBeforeFirstStanza splices block into content just above the first
// Host/Match line, or at the end when there is none, and returns the result.
func insertBeforeFirstStanza(content, block string) string {
	lines := strings.Split(content, "\n")
	insertAt := len(lines)
	for i, line := range lines {
		if isStanzaStart(line) {
			insertAt = i
			break
		}
	}
	head := strings.Join(lines[:insertAt], "\n")
	tail := strings.Join(lines[insertAt:], "\n")
	if head != "" && !strings.HasSuffix(head, "\n") {
		head += "\n"
	}
	return head + block + "\n" + tail
}

// MigrateManagedBlocks moves CLI-managed Host blocks that older versions wrote
// straight into ~/.ssh/config over to the managed config file, and returns the
// markers it moved. Nothing without a managed marker is ever touched, so blocks
// the user wrote by hand stay exactly where they are. It is a no-op (empty
// result, no writes) once there is nothing left to move, so callers can run it
// unconditionally.
func (s SshService) MigrateManagedBlocks() (moved []ManagedMarker, err error) {
	userPath := domain.UserSSHConfigPath()
	userContent, err := readFileIfExists(userPath)
	if err != nil {
		return nil, err
	}
	stale := ListManagedBlocks(userContent)
	if len(stale) == 0 {
		return nil, nil
	}

	managedPath, managedContent, err := s.ReadManagedConfig()
	if err != nil {
		return nil, err
	}

	plan := planBlockMigration(userContent, managedContent, stale)

	// Write the destination first: a failure there must not leave the blocks
	// deleted from the user's config with nowhere to go.
	if err := os.WriteFile(managedPath, []byte(plan.managedContent), 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(userPath+".bak", []byte(userContent), 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(userPath, []byte(plan.userContent), 0o600); err != nil {
		return nil, err
	}
	return plan.moved, nil
}

// blockMigration is the outcome of moving managed blocks between the two
// configs: the new content of each file, and the markers that actually moved.
type blockMigration struct {
	managedContent string
	userContent    string
	moved          []ManagedMarker
}

// planBlockMigration computes both files' new content without touching the
// filesystem, so the ordering guarantees MigrateManagedBlocks needs (destination
// written before the source is truncated) stay visible in one place and the
// rewriting rules can be tested on strings alone.
func planBlockMigration(userContent, managedContent string, stale []ManagedMarker) blockMigration {
	remaining := userContent
	body := strings.TrimRight(managedContent, "\n")
	var moved []ManagedMarker
	for _, m := range stale {
		stanza := strings.TrimRight(ExtractHostBlock(remaining, m.Alias), "\n")
		remaining = StripManagedBlockByRef(remaining, m.Kind, m.Ref)
		if stanza == "" {
			// The marker was there but the stanza was not; drop the orphaned
			// marker and move on rather than writing an empty block.
			continue
		}
		// ExtractHostBlock returns the stanza only, so the marker is re-rendered
		// rather than carried over. That also normalizes the legacy
		// "workspace=<ws>" form to the current v=1 spelling on the way across.
		marker := sshdefaults.ManagedComment(sshdefaults.Kind(m.Kind), m.Ref, m.Alias, m.Host)
		if body != "" {
			body += "\n\n"
		}
		body += marker + "\n" + stanza
		moved = append(moved, m)
	}

	remaining = strings.TrimRight(remaining, "\n")
	if remaining != "" {
		remaining += "\n"
	}
	return blockMigration{managedContent: body + "\n", userContent: remaining, moved: moved}
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
// the managed config has one. Unlike ReadManagedConfig it does not create the file: a
// missing config is treated as "not found" (ok=false), not an error. Used by
// 'ssh' to skip the setup-ssh bootstrap when access is already configured.
func (s SshService) ManagedAlias(kind sshdefaults.Kind, ref string) (alias string, ok bool, err error) {
	path, err := managedConfigPath()
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

// HostAliasConflict describes an existing Host block that already claims an
// alias the CLI is about to write.
type HostAliasConflict struct {
	Path    string // the file that declares it
	Block   string // the existing stanza, for display
	Managed bool   // true when it lives in the CLI-owned config
}

// FindHostAliasConflict reports whether alias is already declared, checking the
// managed config first and then the user's own ~/.ssh/config. Looking at both is
// what stops the CLI from silently shadowing a Host the user wrote by hand.
// Returns nil when the alias is free.
func (s SshService) FindHostAliasConflict(alias string) (*HostAliasConflict, error) {
	managedPath, err := managedConfigPath()
	if err != nil {
		return nil, err
	}
	managed, err := readFileIfExists(managedPath)
	if err != nil {
		return nil, err
	}
	if HasHostAlias(managed, alias) {
		return &HostAliasConflict{Path: managedPath, Block: ExtractHostBlock(managed, alias), Managed: true}, nil
	}
	userPath := domain.UserSSHConfigPath()
	user, err := readFileIfExists(userPath)
	if err != nil {
		return nil, err
	}
	if HasHostAlias(user, alias) {
		return &HostAliasConflict{Path: userPath, Block: ExtractHostBlock(user, alias)}, nil
	}
	return nil, nil
}

// AliasHostName returns the HostName the Host block for alias dials, or "" when
// the block or its HostName is absent. It is what the CLI pins host keys
// against: the exact address ssh will use. It reads BOTH configs, because 'ssh'
// can be pointed at an alias the user wrote by hand.
func (s SshService) AliasHostName(alias string) (string, error) {
	content, err := s.managedAndUserConfigs()
	if err != nil {
		return "", err
	}
	return HostNameForAlias(content, alias), nil
}

// ManagedHostName returns the address the managed block for kind + ref dials:
// its alias' HostName. It reads the config once, and yields "" when the config,
// the block, or its HostName is absent (a ProxyCommand block sets none). Callers
// use it to learn what a block had pinned *before* removing it.
func (s SshService) ManagedHostName(kind sshdefaults.Kind, ref string) (string, error) {
	path, err := managedConfigPath()
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

// HostIsReferenced reports whether any Host block still dials host. It guards
// the removal of a pinned key: two aliases may point at the same container, so a
// key is only dropped once nothing reaches it any more. It reads BOTH configs —
// a block the user wrote by hand keeps the key alive just as much as a managed
// one does.
func (s SshService) HostIsReferenced(host string) (bool, error) {
	if host == "" {
		return false, nil
	}
	content, err := s.managedAndUserConfigs()
	if err != nil {
		return false, err
	}
	return slices.Contains(ConfigHostTargets(content), host), nil
}

// EnsureHostKeyPinning upgrades an already written Host block in place so it
// verifies host keys against the CLI-managed known_hosts. Blocks generated
// before host-key pinning existed record container keys in the user's global
// ~/.ssh/known_hosts, where a rebuilt container's new key reads as a changed
// host key; this repairs them without the user re-running setup-ssh. It backs
// the config up to path+".bak" before rewriting and reports whether the block
// needed changing.
func (s SshService) EnsureHostKeyPinning(alias, knownHostsFile string) (updated bool, err error) {
	path, err := managedConfigPath()
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
// the managed SSH config. Unlike ReadManagedConfig it does not create the file: a missing
// config (or no matching block) is a no-op returning removed=false. When a block
// is removed it backs the original up to path+".bak" first and returns that path.
func (s SshService) RemoveManagedBlockByRef(kind sshdefaults.Kind, ref string) (removed bool, backupPath string, err error) {
	path, err := managedConfigPath()
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

// targetAlive reports whether a managed block's target still exists and
// whether that verdict could actually be verified. An unrecognized kind is
// treated as alive+verified so clean-ssh never removes a block it does not
// understand.
func targetAlive(b ManagedMarker, existsWorkspace, existsContainer func(string) bool, existsRemoteContainer func(host, name string) (alive, verified bool)) (alive, verified bool) {
	switch b.Kind {
	case string(sshdefaults.KindContainer):
		if b.Host != "" {
			return existsRemoteContainer(b.Host, b.Ref)
		}
		return existsContainer(b.Ref), true
	case string(sshdefaults.KindWorkspace):
		return existsWorkspace(b.Ref), true
	default:
		return true, true
	}
}

// FindOrphanedMarkers returns every managed marker in content that has no
// "Host <alias>" stanza directly beneath it — the shape BuildConfigBlock
// always produces. Leftover garbage from an incomplete append/replace, it is
// reported regardless of whether its workspace/container is otherwise alive:
// no command can resolve or connect through it either way. Legacy markers
// with no explicit alias= are skipped here; ListManagedBlocks recovers those
// separately.
func FindOrphanedMarkers(content string) []ManagedMarker {
	lines := strings.Split(content, "\n")
	var out []ManagedMarker
	for i, line := range lines {
		m, ok := parseManagedMarker(line)
		if !ok || m.Alias == "" {
			continue
		}
		if i+1 < len(lines) && slices.Contains(HostAliasesInLine(lines[i+1]), m.Alias) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// PruneManagedBlocks removes CLI managed Host blocks whose target no longer
// exists, plus any orphaned/duplicate marker from FindOrphanedMarkers
// regardless of target liveness. Results are deduplicated by kind+ref:
// StripManagedBlockByRef already drops every occurrence for a ref in one
// pass, so a live target's orphaned+valid copies collapse into a single
// removal (the alias resets via the normal setup-ssh bootstrap on next
// connect).
//
// A --via block whose remote host could not be reached is never removed —
// deleting it could throw away still-valid access — but it is not silently
// dropped either: it comes back in unverified so the caller (clean-ssh) can
// show it separately and let the user remove it explicitly.
//
// A missing config, or nothing stale/unverified, is a no-op returning empty
// slices. Rewrites back the original up to path+".bak" first. The predicates
// are injected so this stays free of infra/registry dependencies and is
// table-testable.
func (s SshService) PruneManagedBlocks(existsWorkspace, existsContainer func(string) bool, existsRemoteContainer func(host, name string) (alive, verified bool), dryRun bool) (removed, unverified []ManagedMarker, backupPath string, err error) {
	path, err := managedConfigPath()
	if err != nil {
		return nil, nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, "", nil
		}
		return nil, nil, "", err
	}
	content := string(data)

	orphanRefs := make(map[string]bool)
	for _, m := range FindOrphanedMarkers(content) {
		orphanRefs[m.Kind+"|"+m.Ref] = true
	}

	var stale, unverifiedBlocks []ManagedMarker
	seen := make(map[string]bool)
	seenUnverified := make(map[string]bool)
	for _, b := range ListManagedBlocks(content) {
		key := b.Kind + "|" + b.Ref
		if orphanRefs[key] {
			if !seen[key] {
				seen[key] = true
				stale = append(stale, b)
			}
			continue
		}
		if seen[key] || seenUnverified[key] {
			continue
		}
		// verified must be checked before alive: on failure alive is just its
		// fail-open default (true), not a real finding.
		alive, verified := targetAlive(b, existsWorkspace, existsContainer, existsRemoteContainer)
		if !verified {
			seenUnverified[key] = true
			unverifiedBlocks = append(unverifiedBlocks, b)
			continue
		}
		if alive {
			continue
		}
		seen[key] = true
		stale = append(stale, b)
	}
	if len(stale) == 0 {
		return nil, unverifiedBlocks, "", nil
	}
	if dryRun {
		return stale, unverifiedBlocks, "", nil
	}

	backupPath = path + ".bak"
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return nil, nil, "", err
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
		return nil, nil, "", err
	}
	return stale, unverifiedBlocks, backupPath, nil
}

// RemoveManagedBlocks removes exactly the given managed blocks from the
// managed SSH config, without re-checking whether their target is alive — the
// caller (clean-ssh, after the user picks a subset of the stale blocks it
// found) has already decided which ones to drop. A missing config, or an
// empty blocks slice, is a no-op returning an empty result. When it rewrites
// the file it first backs the original up to path+".bak" and returns that
// path.
func (s SshService) RemoveManagedBlocks(blocks []ManagedMarker) (removed []ManagedMarker, backupPath string, err error) {
	if len(blocks) == 0 {
		return nil, "", nil
	}
	path, err := managedConfigPath()
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

// ConfigHostAliasesFromDisk returns the non-wildcard Host aliases declared
// across BOTH the managed config and the user's ~/.ssh/config, or nil if
// neither can be read. It backs shell completion, where offering only the
// CLI's own aliases would be a regression.
func (s SshService) ConfigHostAliasesFromDisk() []string {
	content, err := s.managedAndUserConfigs()
	if err != nil {
		return nil
	}
	return ConfigHostAliases(content)
}

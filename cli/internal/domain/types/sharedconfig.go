package types

// Shared persistent tool-config support. A single daemon-level Docker volume
// (devcontainer-shared-config) is mounted into every managed container at
// SharedConfigMountPath; the container entrypoint materializes one entry per
// SharedConfigEntries row inside the volume and symlinks it into devuser's home.
// This lets AI/dev tools (Claude Code, Codex, gh, …) keep their config, sessions
// and logins across every container the CLI creates — login once, reuse forever.
//
// The volume is intentionally NOT workspace-prefixed (unlike PersistVolumeSpecs):
// it is shared by all workspaces. It is declared `external: true` in the compose
// document so `docker compose down -v` / destroy never delete it.

// SharedConfigKind distinguishes a directory entry from a single-file entry: a
// dir entry is mkdir'd in the volume, a file entry is touch'd, before symlinking.
type SharedConfigKind string

const (
	SharedConfigDir  SharedConfigKind = "dir"
	SharedConfigFile SharedConfigKind = "file"
)

// SharedConfigEntry maps one tool's config inside the shared volume to its
// location in devuser's home directory.
type SharedConfigEntry struct {
	ID     string           // subpath inside the volume, e.g. "claude"
	Target string           // path relative to DevUserHome, e.g. ".claude"
	Kind   SharedConfigKind // dir → mkdir+symlink; file → touch+symlink
}

// SharedConfigEntries is the catalog of tool configs persisted across containers.
// Adding a future tool is one new row here PLUS one matching row in the shell
// table inside internal/infra/assets/entrypoint.sh (a test keeps them in sync).
//
// Note: the "gemini" entry (~/.gemini) is also the documented config home of
// the Antigravity CLI (agy) — settings (antigravity-cli/settings.json), plugins
// (antigravity-cli/plugins/), skills and MCP config all live under it (see
// antigravity.google/docs/cli-settings and /docs/cli-plugins). Do not add a
// nested entry for ~/.gemini/antigravity-cli: symlinking a subpath of an
// already-symlinked dir would break. The agy binary itself installs to
// ~/.local/bin (docs/cli-install) and is intentionally NOT shared.
//
// The "agents" entry (~/.agents) is the cross-tool home for reusable agent
// rules/workflows/skills: Graphify, Caveman, Antigravity and others drop their
// reusable assets there, so persisting it keeps those definitions across every
// container the CLI creates. It also doubles as the canonical store for skills
// and agents installed globally with the skills.sh CLI (`npx skills add -g`),
// which places them under ~/.agents/skills (and symlinks them into each
// agent's own config dir, e.g. ~/.claude/skills) — see fixSymlinksFunc in
// service/sharedconfig.go for how those cross-entry links are kept alive in
// the volume.
//
// The volume is FLAT (one dir per entry id) while the home it is symlinked
// into is not, so a relative cross-entry link written against the home layout
// (~/.claude/skills/x -> ../../.agents/skills/x) has no name to land on once
// ~/.claude is itself a symlink into that flat root. Both the entrypoint and
// the sync helper therefore also mirror the home layout at the volume root
// (<volume>/.claude -> claude, <volume>/.config/gh -> ../gh, …). Adding an
// entry needs nothing extra for this: the alias is derived from Target.
var SharedConfigEntries = []SharedConfigEntry{
	{ID: "claude", Target: ".claude", Kind: SharedConfigDir},
	{ID: "claude.json", Target: ".claude.json", Kind: SharedConfigFile},
	{ID: "antigravity", Target: ".antigravity", Kind: SharedConfigDir},
	{ID: "antigravity-config", Target: ".config/antigravity", Kind: SharedConfigDir},
	{ID: "gemini", Target: ".gemini", Kind: SharedConfigDir}, // Gemini CLI creds + Antigravity CLI settings/plugins/skills/MCP
	{ID: "agents", Target: ".agents", Kind: SharedConfigDir}, // reusable agent rules/workflows/skills (Graphify, Caveman, Antigravity, …)
	{ID: "codex", Target: ".codex", Kind: SharedConfigDir},
	{ID: "gh", Target: ".config/gh", Kind: SharedConfigDir},
	// The user's own shell aliases. Persisting them here is what makes
	// `config alias` edits apply to every container without an image rebuild:
	// the baked defaults (~/.devcontainer_aliases.sh, from the aliases module)
	// are sourced first, then this file, so user definitions win.
	{ID: SharedConfigAliasID, Target: SharedConfigAliasTarget, Kind: SharedConfigFile},
}

// SharedConfigAliasID is the entry id of the user's own shell alias file. It is
// referenced by name from domain.UserAliasFilePath() and the `config alias`
// commands, so keep it a named constant rather than a bare string.
const SharedConfigAliasID = "alias.sh"

// SharedConfigAliasTarget is where that entry is symlinked inside the container,
// relative to DevUserHome. The aliases module sources it from every rc file, so
// the two must agree.
const SharedConfigAliasTarget = ".alias.sh"

// SharedConfigEntryByID returns the entry for an id and whether it exists.
func SharedConfigEntryByID(id string) (SharedConfigEntry, bool) {
	for _, e := range SharedConfigEntries {
		if e.ID == id {
			return e, true
		}
	}
	return SharedConfigEntry{}, false
}

// SharedConfigIDs returns every entry id in display order.
func SharedConfigIDs() []string {
	ids := make([]string, len(SharedConfigEntries))
	for i, e := range SharedConfigEntries {
		ids[i] = e.ID
	}
	return ids
}

// SharedConfigVolumeName is the ONE daemon-level volume shared by every
// workspace/container. It must never be passed through prefixVolume().
const SharedConfigVolumeName = "devcontainer-shared-config"

// SharedConfigMountPath is where the shared volume is mounted inside containers.
const SharedConfigMountPath = "/mnt/shared-config"

// SharedConfigMount returns the docker volume spec ("name:path") for the mount.
func SharedConfigMount() string {
	return SharedConfigVolumeName + ":" + SharedConfigMountPath
}

// SharedConfigEnabled reports whether the shared-config volume is enabled for a
// config. A nil Compose.SharedConfig means "unset" and is treated as enabled
// (the default, including legacy configs); set it to false to opt out.
func SharedConfigEnabled(c *DevcontainerConfig) bool {
	return c.Compose.SharedConfig == nil || *c.Compose.SharedConfig
}

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
var SharedConfigEntries = []SharedConfigEntry{
	{ID: "claude", Target: ".claude", Kind: SharedConfigDir},
	{ID: "claude.json", Target: ".claude.json", Kind: SharedConfigFile},
	{ID: "antigravity", Target: ".antigravity", Kind: SharedConfigDir},
	{ID: "antigravity-config", Target: ".config/antigravity", Kind: SharedConfigDir},
	{ID: "gemini", Target: ".gemini", Kind: SharedConfigDir},
	{ID: "codex", Target: ".codex", Kind: SharedConfigDir},
	{ID: "gh", Target: ".config/gh", Kind: SharedConfigDir},
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

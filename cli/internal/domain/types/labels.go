package types

import (
	"fmt"
	"strings"
)

const LabelNamespace = "dev.devcontainer-installer"
const LabelManaged = LabelNamespace + ".managed"
const LabelProject = LabelNamespace + ".project"
const LabelVersion = LabelNamespace + ".version"
const LabelQuickRun = LabelNamespace + ".quick-run"

// Image identity labels. Every image the CLI builds carries them so any image
// can be classified from the daemon alone, without images.json:
//   - kind: ImageKindProject (built for projects, identified by content and
//     garbage-collected when no registered project or container uses it) or
//     ImageKindNamed (a reusable `image` definition, never collected).
//   - origin: what it was built from (profile ids, "custom", or the image name).
//   - modules: the selected Dockerfile module ids, comma-separated.
//   - fingerprint: the content hash the tag's short suffix comes from.
//   - cli-version: the CLI release whose templates rendered the Dockerfile.
//   - owner: the configuration registry that built it (domain.RegistryOwnerID).
//     Automatic collection only ever removes images its own registry built,
//     because images.json describes one user's projects while the daemon may
//     be shared with other users, sudo runs or other config directories.
const (
	LabelOwner = LabelNamespace + ".owner"
	// LabelModuleOptions holds the non-default module options as JSON
	// ({"<id>": {...}}), so an image derived from this one inherits them.
	LabelModuleOptions = LabelNamespace + ".module-options"
	LabelKind          = LabelNamespace + ".kind"
	LabelOrigin        = LabelNamespace + ".origin"
	LabelModules       = LabelNamespace + ".modules"
	LabelFingerprint   = LabelNamespace + ".fingerprint"
	LabelCLIVersion    = LabelNamespace + ".cli-version"
)

const (
	ImageKindProject = "project"
	ImageKindNamed   = "named"
)

// CLIVersion is the running CLI's release, stamped into image labels. The root
// command sets it at start-up; it stays "dev" in tests and local builds.
var CLIVersion = "dev"

// LabelSharedConfig marks the single shared tool-config volume so prune can
// identify (and protect) it. The volume name is the primary filter; this label
// is a belt-and-braces identifier applied at `docker volume create`.
const LabelSharedConfig = LabelNamespace + ".shared-config"
const SchemaVersion = "1"

// ImageNamespace is the repository prefix for project images, tagged by
// origin and content (devcontainer-cli/<origin>-<short-fingerprint>:latest).
// Prune detects images by the managed label instead, so both local and remote
// images are covered.
const ImageNamespace = "devcontainer-cli"

// ImageNamespaceImg is the repository prefix for reusable named images built
// by the image module (devcontainer-cli-img/<name>:<tag>). This isolates them
// from project lifecycle cleanup (clean/destroy/prune).
const ImageNamespaceImg = "devcontainer-cli-img"

func ProjectID(config *DevcontainerConfig) string {
	replacer := strings.NewReplacer(":", "_", "/", "_", "@", "_")
	return replacer.Replace(config.Image)
}

// DockerfileLabelBlock renders the image's LABEL instruction. Labels are
// stripped before the fingerprint is computed, so stamping the fingerprint and
// CLI version here never changes the image's identity.
func DockerfileLabelBlock(config *DevcontainerConfig) string {
	kind := config.ImageKind
	if kind == "" {
		kind = ImageKindProject
	}
	modules := config.Dockerfile.AllModuleIDs()
	pairs := [][2]string{
		{LabelManaged, "true"},
		{LabelProject, ProjectID(config)},
		{LabelVersion, SchemaVersion},
		{LabelKind, kind},
		{LabelOrigin, config.ResolvedOrigin()},
		{LabelModules, strings.Join(modules, ",")},
		{LabelFingerprint, config.Fingerprint},
		{LabelCLIVersion, CLIVersion},
	}
	if config.Owner != "" {
		pairs = append(pairs, [2]string{LabelOwner, config.Owner})
	}
	if opts := config.Dockerfile.ModuleOptionsJSON(); opts != "" {
		pairs = append(pairs, [2]string{LabelModuleOptions, opts})
	}
	lines := make([]string, 0, len(pairs))
	for _, p := range pairs {
		lines = append(lines, fmt.Sprintf("%s=%q", p[0], p[1]))
	}
	return "LABEL " + strings.Join(lines, " \\\n      ")
}

func ComposeLabels(config *DevcontainerConfig) map[string]string {
	return map[string]string{
		LabelManaged: "true",
		LabelProject: ProjectID(config),
		LabelVersion: SchemaVersion,
	}
}

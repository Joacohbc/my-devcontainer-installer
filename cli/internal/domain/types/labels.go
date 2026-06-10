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

// LabelSharedConfig marks the single shared tool-config volume so prune can
// identify (and protect) it. The volume name is the primary filter; this label
// is a belt-and-braces identifier applied at `docker volume create`.
const LabelSharedConfig = LabelNamespace + ".shared-config"
const SchemaVersion = "1"

// ImageNamespace is the repository prefix for locally built images
// (devcontainer-cli/<fingerprint>:latest). Prune detects images by the
// managed label instead, so both local and remote images are covered.
const ImageNamespace = "devcontainer-cli"

func ProjectID(config *DevcontainerConfig) string {
	replacer := strings.NewReplacer(":", "_", "/", "_", "@", "_")
	return replacer.Replace(config.Image)
}

func DockerfileLabelBlock(config *DevcontainerConfig) string {
	project := ProjectID(config)
	return fmt.Sprintf(
		"LABEL %s=\"true\" \\\n      %s=\"%s\" \\\n      %s=\"%s\"",
		LabelManaged,
		LabelProject, project,
		LabelVersion, SchemaVersion,
	)
}

func ComposeLabels(config *DevcontainerConfig) map[string]string {
	return map[string]string{
		LabelManaged: "true",
		LabelProject: ProjectID(config),
		LabelVersion: SchemaVersion,
	}
}

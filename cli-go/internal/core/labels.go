package core

import (
	"fmt"
	"strings"
)

const LabelNamespace = "dev.devcontainer-installer"
const LabelManaged = LabelNamespace + ".managed"
const LabelProject = LabelNamespace + ".project"
const LabelVersion = LabelNamespace + ".version"
const LabelQuickRun = LabelNamespace + ".quick-run"
const SchemaVersion = "1"

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

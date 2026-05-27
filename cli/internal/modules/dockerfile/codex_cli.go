package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/core"
)

var CodexCliModule = &DockerfileModuleSpec{
	ID:       "codex-cli",
	Label:    "Codex CLI (npx, no global install)",
	Category: core.CategoryInfra,
	Requires: []string{"nodejs"},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-codex-cli.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Codex CLI — install script shipped under %s\n##\n", core.PostScriptDir)
	},
}

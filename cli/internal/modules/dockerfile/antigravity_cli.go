package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/core"
)

var AntigravityCliModule = &DockerfileModuleSpec{
	ID:       "antigravity-cli",
	Label:    "Antigravity CLI (native standalone installer)",
	Category: core.CategoryInfra,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-antigravity.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Antigravity CLI — install script shipped under %s\n##\n", core.PostScriptDir)
	},
}

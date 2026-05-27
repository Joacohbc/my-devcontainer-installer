package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/core"
)

var OpencodeModule = &DockerfileModuleSpec{
	ID:       "opencode",
	Label:    "OpenCode (opencode.ai installer)",
	Category: core.CategoryInfra,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-opencode.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## OpenCode — install script shipped under %s\n##\n", core.PostScriptDir)
	},
}

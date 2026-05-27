package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var CopilotCliModule = &ModuleSpec{
	ID:       "copilot-cli",
	Label:    "GitHub Copilot CLI (standalone binary)",
	Category: types.CategoryInfra,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-copilot.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## GitHub Copilot CLI — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}

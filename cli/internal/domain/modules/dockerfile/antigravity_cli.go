package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var AntigravityCliModule = &ModuleSpec{
	ID:         "antigravity-cli",
	Label:      "Antigravity CLI (native standalone installer)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-antigravity.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Antigravity CLI — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}

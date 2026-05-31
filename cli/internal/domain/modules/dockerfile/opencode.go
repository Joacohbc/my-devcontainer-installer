package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var OpencodeModule = &ModuleSpec{
	ID:         types.ModuleOpencode,
	Label:      "OpenCode (opencode.ai installer)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-opencode.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## OpenCode — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}

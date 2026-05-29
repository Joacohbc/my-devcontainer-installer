package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var CodexCliModule = &ModuleSpec{
	ID:         "codex-cli",
	Label:      "Codex CLI (npx, no global install)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	Requires:   []string{"nodejs"},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-codex-cli.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Codex CLI — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
